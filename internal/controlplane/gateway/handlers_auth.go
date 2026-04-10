package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"yunque-agent/internal/apperror"
)

const (
	bcryptCost          = 12
	minPasswordLen      = 8
	loginLockoutMax     = 5
	loginLockoutWindow  = 15 * time.Minute
)

// authKVStore abstracts Ledger KV to avoid import cycles.
type authKVStore interface {
	Put(ctx context.Context, key string, value any) error
	Get(ctx context.Context, key string, dest any) (bool, error)
}

// PasswordStore manages the admin password with bcrypt hashing and brute-force protection.
// Persisted to Ledger KV (preferred) or data/auth.json (fallback).
type PasswordStore struct {
	mu      sync.RWMutex
	bcrypt  string // bcrypt hash (preferred)
	legacy  string // SHA256(salt+password) — only used for migration
	salt    string // only present for legacy hashes
	path    string
	isSetup bool
	kvs     authKVStore

	failMu   sync.Mutex
	failures map[string]*loginFailure // IP → failure tracking
}

type loginFailure struct {
	Count    int
	LastFail time.Time
}

type authData struct {
	Bcrypt string `json:"bcrypt,omitempty"`
	Hash   string `json:"hash,omitempty"` // legacy SHA256
	Salt   string `json:"salt,omitempty"` // legacy
}

func NewPasswordStore(path string) *PasswordStore {
	ps := &PasswordStore{path: path, failures: make(map[string]*loginFailure)}
	ps.load()
	return ps
}

// SetKVStore enables Ledger KV-backed persistence, replacing file I/O.
// Migrates existing file data to KV on first call.
func (ps *PasswordStore) SetKVStore(kvs authKVStore) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.kvs = kvs

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if ps.isSetup {
		ad := authData{Bcrypt: ps.bcrypt, Hash: ps.legacy, Salt: ps.salt}
		if err := kvs.Put(ctx, "auth", ad); err != nil {
			slog.Warn("auth: KV migration failed", "err", err)
		} else {
			slog.Info("auth: migrated to Ledger KV")
		}
	} else {
		var ad authData
		if found, err := kvs.Get(ctx, "auth", &ad); err == nil && found {
			if ad.Bcrypt != "" {
				ps.bcrypt = ad.Bcrypt
				ps.isSetup = true
			} else if ad.Hash != "" {
				ps.legacy = ad.Hash
				ps.salt = ad.Salt
				ps.isSetup = true
			}
			slog.Info("auth: restored from Ledger KV")
		}
	}
}

func (ps *PasswordStore) IsSetup() bool {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.isSetup
}

func (ps *PasswordStore) SetPassword(password string) error {
	if len(password) < minPasswordLen {
		return fmt.Errorf("password must be at least %d characters", minPasswordLen)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return fmt.Errorf("bcrypt hash: %w", err)
	}
	ps.mu.Lock()
	ps.bcrypt = string(hash)
	ps.legacy = ""
	ps.salt = ""
	ps.isSetup = true
	ps.mu.Unlock()
	return ps.save()
}

// Verify checks the password. If a legacy SHA256 hash matches, it auto-migrates to bcrypt.
func (ps *PasswordStore) Verify(password string) bool {
	ps.mu.RLock()
	bcryptHash := ps.bcrypt
	legacyHash := ps.legacy
	salt := ps.salt
	setup := ps.isSetup
	ps.mu.RUnlock()

	if !setup {
		return false
	}

	if bcryptHash != "" {
		return bcrypt.CompareHashAndPassword([]byte(bcryptHash), []byte(password)) == nil
	}

	// Legacy SHA256 path — verify then migrate
	if legacyHash != "" && legacySHA256(password, salt) == legacyHash {
		go ps.migrateToBycrpt(password)
		return true
	}
	return false
}

func (ps *PasswordStore) migrateToBycrpt(password string) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		slog.Warn("auth: bcrypt migration failed", "err", err)
		return
	}
	ps.mu.Lock()
	ps.bcrypt = string(hash)
	ps.legacy = ""
	ps.salt = ""
	ps.mu.Unlock()
	if err := ps.save(); err != nil {
		slog.Warn("auth: bcrypt migration save failed", "err", err)
	} else {
		slog.Info("auth: migrated password hash from SHA256 to bcrypt")
	}
}

// CheckLockout returns true if the IP is locked out due to too many failures.
func (ps *PasswordStore) CheckLockout(ip string) bool {
	ps.failMu.Lock()
	defer ps.failMu.Unlock()
	f, ok := ps.failures[ip]
	if !ok {
		return false
	}
	if time.Since(f.LastFail) > loginLockoutWindow {
		delete(ps.failures, ip)
		return false
	}
	return f.Count >= loginLockoutMax
}

func (ps *PasswordStore) RecordFailure(ip string) {
	ps.failMu.Lock()
	defer ps.failMu.Unlock()
	f, ok := ps.failures[ip]
	if !ok || time.Since(f.LastFail) > loginLockoutWindow {
		ps.failures[ip] = &loginFailure{Count: 1, LastFail: time.Now()}
		return
	}
	f.Count++
	f.LastFail = time.Now()
}

func (ps *PasswordStore) ClearFailures(ip string) {
	ps.failMu.Lock()
	defer ps.failMu.Unlock()
	delete(ps.failures, ip)
}

func (ps *PasswordStore) load() {
	data, err := os.ReadFile(ps.path)
	if err != nil {
		return
	}
	var auth authData
	if json.Unmarshal(data, &auth) != nil {
		return
	}
	if auth.Bcrypt != "" {
		ps.bcrypt = auth.Bcrypt
		ps.isSetup = true
	} else if auth.Hash != "" {
		ps.legacy = auth.Hash
		ps.salt = auth.Salt
		ps.isSetup = true
	}
}

func (ps *PasswordStore) save() error {
	ps.mu.RLock()
	ad := authData{Bcrypt: ps.bcrypt, Hash: ps.legacy, Salt: ps.salt}
	kvs := ps.kvs
	ps.mu.RUnlock()

	if kvs != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := kvs.Put(ctx, "auth", ad); err != nil {
			slog.Warn("auth: KV save failed, falling back to file", "err", err)
		} else {
			return nil
		}
	}

	data, _ := json.MarshalIndent(ad, "", "  ")
	return os.WriteFile(ps.path, data, 0600)
}

func legacySHA256(password, salt string) string {
	h := sha256.Sum256([]byte(salt + password))
	return hex.EncodeToString(h[:])
}

// ── HTTP Handlers ──

// handleAuthLogin handles POST /v1/auth/login with IP-based lockout.
func (g *Gateway) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST required")
		return
	}

	var req struct {
		Password string `json:"password"`
		Remember bool   `json:"remember"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid request body")
		return
	}

	if g.passwordStore == nil || !g.passwordStore.IsSetup() {
		apperror.Write(w, apperror.New(apperror.CodeInternal, "password not configured, run setup first"))
		return
	}

	ip := extractHost(r)
	if g.passwordStore.CheckLockout(ip) {
		slog.Warn("auth: IP locked out", "ip", ip)
		apperror.WriteCode(w, apperror.CodeTooManyReqs, fmt.Sprintf("too many failed attempts, try again in %d minutes", int(loginLockoutWindow.Minutes())))
		return
	}

	if !g.passwordStore.Verify(req.Password) {
		g.passwordStore.RecordFailure(ip)
		slog.Warn("auth: failed login attempt", "ip", ip)
		time.Sleep(500 * time.Millisecond)
		apperror.WriteCode(w, apperror.CodeUnauthorized, "wrong password")
		return
	}

	g.passwordStore.ClearFailures(ip)

	if g.jwtCfg == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "JWT not configured")
		return
	}

	expiry := 24 * time.Hour
	if req.Remember {
		expiry = 7 * 24 * time.Hour
	}

	cfgCopy := *g.jwtCfg
	cfgCopy.Expiration = expiry

	tenants := g.tenants.List()
	tenantID := "default"
	if len(tenants) > 0 {
		tenantID = tenants[0].ID
	}

	token, err := GenerateJWT(cfgCopy, tenantID, "admin")
	if err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, "token generation failed")
		return
	}

	slog.Info("auth: login success", "ip", ip, "remember", req.Remember)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"token":      token,
		"expires_in": int(expiry.Seconds()),
	})
}

func (g *Gateway) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	isSetup := g.passwordStore != nil && g.passwordStore.IsSetup()

	isAuthenticated := false
	token := r.Header.Get("X-API-Key")
	if token == "" {
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			token = strings.TrimPrefix(auth, "Bearer ")
		}
	}
	if token != "" {
		if t := g.tenants.ByAPIKey(token); t != nil {
			isAuthenticated = true
		} else if g.jwtCfg != nil {
			if _, err := ValidateJWT(*g.jwtCfg, token); err == nil {
				isAuthenticated = true
			}
		}
	}

	json.NewEncoder(w).Encode(map[string]any{
		"password_set":  isSetup,
		"authenticated": isAuthenticated,
	})
}

// handleAuthSetPassword sets or changes the admin password.
func (g *Gateway) handleAuthSetPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST required")
		return
	}

	var req struct {
		Password string `json:"password"`
		Current  string `json:"current"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid request body")
		return
	}

	if g.passwordStore == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "password store not initialized")
		return
	}

	if g.passwordStore.IsSetup() {
		if !g.passwordStore.Verify(req.Current) {
			apperror.WriteCode(w, apperror.CodeForbidden, "current password incorrect")
			return
		}
	}

	if err := g.passwordStore.SetPassword(req.Password); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
		return
	}

	slog.Info("auth: password updated", "ip", extractHost(r))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func extractHost(r *http.Request) string {
	host := r.RemoteAddr
	if idx := strings.LastIndex(host, ":"); idx > 0 {
		host = host[:idx]
	}
	return strings.Trim(host, "[]")
}
