package gateway

import (
	"crypto/rand"
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
)

// PasswordStore manages the admin password (bcrypt-style hashing).
// The hashed password is stored in data/auth.json.
type PasswordStore struct {
	mu       sync.RWMutex
	hash     string // SHA256(salt+password)
	salt     string
	path     string
	isSetup  bool // true if a password has been configured
}

type authData struct {
	Hash string `json:"hash"`
	Salt string `json:"salt"`
}

// NewPasswordStore creates a password store that persists to the given path.
func NewPasswordStore(path string) *PasswordStore {
	ps := &PasswordStore{path: path}
	ps.load()
	return ps
}

// IsSetup returns true if a password has been configured.
func (ps *PasswordStore) IsSetup() bool {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.isSetup
}

// SetPassword sets a new admin password.
func (ps *PasswordStore) SetPassword(password string) error {
	if len(password) < 4 {
		return fmt.Errorf("password must be at least 4 characters")
	}
	salt := generateSalt()
	hash := hashPassword(password, salt)
	ps.mu.Lock()
	ps.hash = hash
	ps.salt = salt
	ps.isSetup = true
	ps.mu.Unlock()
	return ps.save()
}

// Verify checks if the given password matches.
func (ps *PasswordStore) Verify(password string) bool {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	if !ps.isSetup {
		return false
	}
	return hashPassword(password, ps.salt) == ps.hash
}

func (ps *PasswordStore) load() {
	data, err := os.ReadFile(ps.path)
	if err != nil {
		return
	}
	var auth authData
	if json.Unmarshal(data, &auth) == nil && auth.Hash != "" {
		ps.hash = auth.Hash
		ps.salt = auth.Salt
		ps.isSetup = true
	}
}

func (ps *PasswordStore) save() error {
	data, _ := json.MarshalIndent(authData{Hash: ps.hash, Salt: ps.salt}, "", "  ")
	return os.WriteFile(ps.path, data, 0600)
}

func generateSalt() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func hashPassword(password, salt string) string {
	h := sha256.Sum256([]byte(salt + password))
	return hex.EncodeToString(h[:])
}

// ── HTTP Handlers ──

// handleAuthLogin handles POST /v1/auth/login.
// Request: {"password": "xxx", "remember": true}
// Response: {"token": "jwt...", "expires_in": 604800}
func (g *Gateway) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Password string `json:"password"`
		Remember bool   `json:"remember"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	if g.passwordStore == nil || !g.passwordStore.IsSetup() {
		http.Error(w, `{"error":"password not configured, run setup first"}`, http.StatusServiceUnavailable)
		return
	}

	if !g.passwordStore.Verify(req.Password) {
		slog.Warn("auth: failed login attempt", "remote", r.RemoteAddr)
		time.Sleep(500 * time.Millisecond) // brute-force protection
		http.Error(w, `{"error":"wrong password"}`, http.StatusUnauthorized)
		return
	}

	// Generate JWT
	if g.jwtCfg == nil {
		http.Error(w, `{"error":"JWT not configured"}`, http.StatusInternalServerError)
		return
	}

	expiry := 24 * time.Hour
	if req.Remember {
		expiry = 7 * 24 * time.Hour
	}

	// Temporarily set the JWT expiry for this token
	cfgCopy := *g.jwtCfg
	cfgCopy.Expiration = expiry

	tenants := g.tenants.List()
	tenantID := "default"
	if len(tenants) > 0 {
		tenantID = tenants[0].ID
	}

	token, err := GenerateJWT(cfgCopy, tenantID, "admin")
	if err != nil {
		http.Error(w, `{"error":"token generation failed"}`, http.StatusInternalServerError)
		return
	}

	slog.Info("auth: login success", "remote", r.RemoteAddr, "remember", req.Remember)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"token":      token,
		"expires_in": int(expiry.Seconds()),
	})
}

// handleAuthStatus returns whether a password is set and if the user is authenticated.
func (g *Gateway) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	isSetup := g.passwordStore != nil && g.passwordStore.IsSetup()

	// Check if the request has valid auth
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
		"password_set":    isSetup,
		"authenticated":   isAuthenticated,
		"localhost":       extractHost(r) == "127.0.0.1" || extractHost(r) == "::1",
	})
}

// handleAuthSetPassword sets or changes the admin password.
// POST /v1/auth/set-password {"password": "new_password", "current": "old_password"}
func (g *Gateway) handleAuthSetPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Password string `json:"password"`
		Current  string `json:"current"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	if g.passwordStore == nil {
		http.Error(w, `{"error":"password store not initialized"}`, http.StatusInternalServerError)
		return
	}

	// If password is already set, require current password
	if g.passwordStore.IsSetup() {
		if !g.passwordStore.Verify(req.Current) {
			http.Error(w, `{"error":"current password incorrect"}`, http.StatusForbidden)
			return
		}
	}

	if err := g.passwordStore.SetPassword(req.Password); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	slog.Info("auth: password updated", "remote", r.RemoteAddr)
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
