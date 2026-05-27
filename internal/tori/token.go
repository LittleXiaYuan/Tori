package tori

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"sync"
	"time"

	"yunque-agent/internal/appdir"
)

// StoredToken is the persistent representation of a Tori OAuth2 token pair.
type StoredToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	UserID       string    `json:"user_id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	APIKey       string    `json:"api_key"`
	ToriBaseURL  string    `json:"tori_base_url"`
}

// TokenStore manages Tori OAuth2 tokens with persistence and auto-refresh.
type TokenStore struct {
	mu    sync.RWMutex
	token *StoredToken
	cfg   OAuthConfig
}

// NewTokenStore creates a store, loading any previously saved token.
func NewTokenStore(cfg OAuthConfig) *TokenStore {
	ts := &TokenStore{cfg: cfg}
	ts.load()
	return ts
}

func tokenPath() string {
	return appdir.File("tori_token.json")
}

func (ts *TokenStore) load() {
	data, err := os.ReadFile(tokenPath())
	if err != nil {
		return
	}
	var t StoredToken
	if json.Unmarshal(data, &t) == nil && t.AccessToken != "" {
		ts.token = &t
	}
}

func (ts *TokenStore) save() error {
	ts.mu.RLock()
	t := ts.token
	ts.mu.RUnlock()
	if t == nil {
		os.Remove(tokenPath())
		return nil
	}
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(tokenPath(), data, 0600)
}

// Store saves a new token pair (called after successful bind or refresh).
func (ts *TokenStore) Store(token *TokenResponse, user *UserInfo, toriBaseURL string) error {
	ts.mu.Lock()
	ts.token = &StoredToken{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(token.ExpiresIn) * time.Second),
		ToriBaseURL:  toriBaseURL,
	}
	if user != nil {
		ts.token.UserID = user.UserID
		ts.token.Username = user.Username
		ts.token.Email = user.Email
		ts.token.APIKey = user.APIKey
	}
	ts.mu.Unlock()
	return ts.save()
}

// Clear removes the stored token (called on unbind).
func (ts *TokenStore) Clear() error {
	ts.mu.Lock()
	ts.token = nil
	ts.mu.Unlock()
	return ts.save()
}

// Get returns the current token (may be nil if unbound).
func (ts *TokenStore) Get() *StoredToken {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	if ts.token == nil {
		return nil
	}
	cp := *ts.token
	return &cp
}

// IsBound returns true if there is a stored Tori token.
func (ts *TokenStore) IsBound() bool {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ts.token != nil
}

// AccessToken returns the current access token, refreshing if expired.
func (ts *TokenStore) AccessToken() (string, error) {
	ts.mu.RLock()
	t := ts.token
	ts.mu.RUnlock()

	if t == nil {
		return "", nil
	}

	if time.Now().Before(t.ExpiresAt.Add(-30 * time.Second)) {
		return t.AccessToken, nil
	}

	return ts.refresh()
}

func (ts *TokenStore) refresh() (string, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if ts.token == nil {
		return "", nil
	}

	cfg := ts.cfg
	cfg.ToriBaseURL = ts.token.ToriBaseURL
	newToken, err := RefreshAccessToken(cfg, ts.token.RefreshToken)
	if err != nil {
		return "", err
	}

	ts.token.AccessToken = newToken.AccessToken
	if newToken.RefreshToken != "" {
		ts.token.RefreshToken = newToken.RefreshToken
	}
	ts.token.ExpiresAt = time.Now().Add(time.Duration(newToken.ExpiresIn) * time.Second)

	go ts.save()
	return ts.token.AccessToken, nil
}

// StartAutoRefresh runs a background loop that refreshes the token before
// expiry. Stops when ctx is cancelled or the token is cleared.
func (ts *TokenStore) StartAutoRefresh(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ts.mu.RLock()
			t := ts.token
			ts.mu.RUnlock()

			if t == nil {
				continue
			}

			if time.Until(t.ExpiresAt) < 10*time.Minute {
				if _, err := ts.refresh(); err != nil {
					slog.Warn("tori: token refresh failed", "err", err)
				} else {
					slog.Debug("tori: token refreshed")
				}
			}
		}
	}
}
