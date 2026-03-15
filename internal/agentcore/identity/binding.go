package identity

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

const (
	bindCodeLength = 6
	bindCodeTTL    = 10 * time.Minute
)

// BindCode is a one-time code for cross-platform identity binding.
type BindCode struct {
	Code      string    `json:"code"`
	UserID    string    `json:"user_id"`
	Platform  string    `json:"platform"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Binding represents a verified cross-platform identity link.
type Binding struct {
	UserID     string    `json:"user_id"`
	Platform   string    `json:"platform"`
	ExternalID string    `json:"external_id"`
	DisplayName string   `json:"display_name,omitempty"`
	BoundAt    time.Time `json:"bound_at"`
}

// BindingStore manages cross-platform identity bindings and bind codes.
type BindingStore struct {
	mu       sync.RWMutex
	bindings map[string][]Binding // userID -> bindings
	codes    map[string]*BindCode // code -> bind code
}

// NewBindingStore creates an identity binding store.
func NewBindingStore() *BindingStore {
	s := &BindingStore{
		bindings: make(map[string][]Binding),
		codes:    make(map[string]*BindCode),
	}
	go s.gcLoop()
	return s
}

// GenerateCode creates a one-time bind code for a user on a platform.
func (s *BindingStore) GenerateCode(userID, platform string) (*BindCode, error) {
	if userID == "" || platform == "" {
		return nil, fmt.Errorf("user_id and platform required")
	}

	code, err := randomCode(bindCodeLength)
	if err != nil {
		return nil, fmt.Errorf("generate code: %w", err)
	}

	bc := &BindCode{
		Code:      code,
		UserID:    userID,
		Platform:  platform,
		ExpiresAt: time.Now().Add(bindCodeTTL),
	}

	s.mu.Lock()
	s.codes[code] = bc
	s.mu.Unlock()

	return bc, nil
}

// Bind verifies a code and creates a cross-platform binding.
func (s *BindingStore) Bind(code, externalID, displayName string) (*Binding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	bc, ok := s.codes[code]
	if !ok {
		return nil, fmt.Errorf("invalid bind code")
	}
	if time.Now().After(bc.ExpiresAt) {
		delete(s.codes, code)
		return nil, fmt.Errorf("bind code expired")
	}

	// Check if already bound
	for _, b := range s.bindings[bc.UserID] {
		if b.Platform == bc.Platform && b.ExternalID == externalID {
			delete(s.codes, code)
			return &b, nil // already bound
		}
	}

	binding := Binding{
		UserID:      bc.UserID,
		Platform:    bc.Platform,
		ExternalID:  externalID,
		DisplayName: displayName,
		BoundAt:     time.Now(),
	}
	s.bindings[bc.UserID] = append(s.bindings[bc.UserID], binding)
	delete(s.codes, code) // consume code
	return &binding, nil
}

// Resolve finds the unified user ID from an external platform identity.
func (s *BindingStore) Resolve(platform, externalID string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for userID, bindings := range s.bindings {
		for _, b := range bindings {
			if b.Platform == platform && b.ExternalID == externalID {
				return userID, true
			}
		}
	}
	return "", false
}

// GetBindings returns all bindings for a user.
func (s *BindingStore) GetBindings(userID string) []Binding {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Binding, len(s.bindings[userID]))
	copy(result, s.bindings[userID])
	return result
}

// Unbind removes a specific platform binding for a user.
func (s *BindingStore) Unbind(userID, platform, externalID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	bindings := s.bindings[userID]
	for i, b := range bindings {
		if b.Platform == platform && b.ExternalID == externalID {
			s.bindings[userID] = append(bindings[:i], bindings[i+1:]...)
			return true
		}
	}
	return false
}

func (s *BindingStore) gcLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for code, bc := range s.codes {
			if now.After(bc.ExpiresAt) {
				delete(s.codes, code)
			}
		}
		s.mu.Unlock()
	}
}

func randomCode(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b)[:length], nil
}
