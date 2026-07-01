package connectors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"yunque-agent/internal/appdir"
)

// ConnectorStatus represents the connection state.
type ConnectorStatus string

const (
	StatusDisconnected ConnectorStatus = "disconnected"
	StatusConnecting   ConnectorStatus = "connecting"
	StatusConnected    ConnectorStatus = "connected"
	StatusError        ConnectorStatus = "error"
)

// ConnectorDef defines a connector type (e.g., GitHub, Gmail).
type ConnectorDef struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Icon        string   `json:"icon"`
	Category    string   `json:"category"`
	AuthType    string   `json:"auth_type"` // "oauth2", "api_key", "token"
	Beta        bool     `json:"beta,omitempty"`
	Scopes      []string `json:"scopes,omitempty"`

	// OAuth2 settings
	AuthURL   string `json:"auth_url,omitempty"`
	TokenURL  string `json:"token_url,omitempty"`
	RevokeURL string `json:"revoke_url,omitempty"`

	// Actions that this connector provides
	Actions []ActionDef `json:"actions"`
}

// ActionDef defines an action that a connector can perform.
type ActionDef struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Parameters  []ParamDef `json:"parameters,omitempty"`
}

// ParamDef defines a parameter for an action.
type ParamDef struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required,omitempty"`
}

// ConnectorInstance represents a user-configured connector instance.
type ConnectorInstance struct {
	ConnectorID string          `json:"connector_id"`
	Status      ConnectorStatus `json:"status"`
	ConnectedAt *time.Time      `json:"connected_at,omitempty"`
	UserInfo    string          `json:"user_info,omitempty"` // display name or email
	Error       string          `json:"error,omitempty"`
	LastEvent   *ConnectorEvent `json:"last_event,omitempty"`
	Credentials *Credentials    `json:"-"` // not serialized to listing responses
}

// ConnectorEvent is a UI-safe audit crumb for the latest connector lifecycle or
// action event. It intentionally excludes credentials and request parameters.
type ConnectorEvent struct {
	Kind        string    `json:"kind"`
	ConnectorID string    `json:"connector_id"`
	ActionID    string    `json:"action_id,omitempty"`
	Status      string    `json:"status"`
	Message     string    `json:"message,omitempty"`
	At          time.Time `json:"at"`
}

// Credentials stores the authentication tokens (encrypted on disk).
type Credentials struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	Scopes       []string  `json:"scopes,omitempty"`
	APIKey       string    `json:"api_key,omitempty"`
}

// TokenExpired checks if the access token has expired.
func (c *Credentials) TokenExpired() bool {
	if c.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(c.ExpiresAt.Add(-60 * time.Second))
}

// Registry manages connector definitions and instances.
type Registry struct {
	mu        sync.RWMutex
	defs      map[string]*ConnectorDef
	instances map[string]*ConnectorInstance
	store     string // directory for persisted state
	handlers  map[string]ConnectorHandler
}

// ConnectorHandler implements connector-specific logic (API calls, token refresh, etc.).
type ConnectorHandler interface {
	// Connect initiates the connection (returns an auth URL for OAuth2, or connects directly for API key).
	Connect(ctx context.Context, creds *Credentials) (*Credentials, error)
	// Disconnect revokes the connection.
	Disconnect(ctx context.Context, creds *Credentials) error
	// Execute runs a connector action.
	Execute(ctx context.Context, creds *Credentials, actionID string, params map[string]any) (any, error)
	// Refresh refreshes the access token if expired.
	Refresh(ctx context.Context, creds *Credentials) (*Credentials, error)
	// Validate checks if the current credentials are still valid.
	Validate(ctx context.Context, creds *Credentials) (bool, string, error)
}

// NewRegistry creates a new connector registry.
func NewRegistry() *Registry {
	return &Registry{
		defs:      make(map[string]*ConnectorDef),
		instances: make(map[string]*ConnectorInstance),
		store:     appdir.Sub("connectors"),
		handlers:  make(map[string]ConnectorHandler),
	}
}

// RegisterDef adds a connector definition.
func (r *Registry) RegisterDef(def *ConnectorDef) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.defs[def.ID] = def
}

// RegisterHandler sets a handler for a connector type.
func (r *Registry) RegisterHandler(connID string, handler ConnectorHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[connID] = handler
}

// ListDefs returns all connector definitions.
func (r *Registry) ListDefs() []*ConnectorDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*ConnectorDef, 0, len(r.defs))
	for _, d := range r.defs {
		result = append(result, d)
	}
	return result
}

// GetDef returns a connector definition by ID.
func (r *Registry) GetDef(id string) *ConnectorDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.defs[id]
}

// HasHandler reports whether a connector has a registered runtime handler.
func (r *Registry) HasHandler(id string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.handlers[id]
	return ok
}

// GetInstance returns the instance for a connector, or a default disconnected one.
func (r *Registry) GetInstance(connID string) *ConnectorInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if inst, ok := r.instances[connID]; ok {
		return inst
	}
	return &ConnectorInstance{
		ConnectorID: connID,
		Status:      StatusDisconnected,
	}
}

// ListInstances returns all connector instances (including disconnected).
func (r *Registry) ListInstances() []ConnectorInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]ConnectorInstance, 0, len(r.defs))
	for id, def := range r.defs {
		if inst, ok := r.instances[id]; ok {
			result = append(result, ConnectorInstance{
				ConnectorID: inst.ConnectorID,
				Status:      inst.Status,
				ConnectedAt: inst.ConnectedAt,
				UserInfo:    inst.UserInfo,
				Error:       inst.Error,
				LastEvent:   cloneConnectorEvent(inst.LastEvent),
			})
		} else {
			result = append(result, ConnectorInstance{
				ConnectorID: def.ID,
				Status:      StatusDisconnected,
			})
		}
	}
	return result
}

// ConnectWithKey connects a connector using an API key or personal access token.
func (r *Registry) ConnectWithKey(ctx context.Context, connID, key string) error {
	r.mu.Lock()
	handler, ok := r.handlers[connID]
	r.mu.Unlock()

	if !ok {
		return fmt.Errorf("no handler for connector %s", connID)
	}

	creds := &Credentials{AccessToken: key, APIKey: key}
	newCreds, err := handler.Connect(ctx, creds)
	if err != nil {
		r.setInstance(connID, &ConnectorInstance{
			ConnectorID: connID,
			Status:      StatusError,
			Error:       err.Error(),
			LastEvent:   newConnectorEvent(connID, "", "connect", "error", err.Error()),
		})
		return err
	}

	valid, userInfo, err := handler.Validate(ctx, newCreds)
	if err != nil || !valid {
		msg := "validation failed"
		if err != nil {
			msg = err.Error()
		}
		r.setInstance(connID, &ConnectorInstance{
			ConnectorID: connID,
			Status:      StatusError,
			Error:       msg,
			LastEvent:   newConnectorEvent(connID, "", "connect", "error", msg),
		})
		return errors.New(msg)
	}

	now := time.Now()
	inst := &ConnectorInstance{
		ConnectorID: connID,
		Status:      StatusConnected,
		ConnectedAt: &now,
		UserInfo:    userInfo,
		Credentials: newCreds,
		LastEvent:   newConnectorEvent(connID, "", "connect", "ok", userInfo),
	}
	r.setInstance(connID, inst)
	r.saveCreds(connID, newCreds)
	slog.Info("connector connected", "id", connID, "user", userInfo)
	return nil
}

// ConnectOAuth2 stores OAuth2 tokens after the callback.
func (r *Registry) ConnectOAuth2(ctx context.Context, connID string, creds *Credentials) error {
	r.mu.RLock()
	handler, ok := r.handlers[connID]
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("no handler for connector %s", connID)
	}

	valid, userInfo, err := handler.Validate(ctx, creds)
	if err != nil || !valid {
		return fmt.Errorf("oauth2 validation failed: %v", err)
	}

	now := time.Now()
	inst := &ConnectorInstance{
		ConnectorID: connID,
		Status:      StatusConnected,
		ConnectedAt: &now,
		UserInfo:    userInfo,
		Credentials: creds,
		LastEvent:   newConnectorEvent(connID, "", "oauth2_connect", "ok", userInfo),
	}
	r.setInstance(connID, inst)
	r.saveCreds(connID, creds)
	slog.Info("connector oauth2 connected", "id", connID, "user", userInfo)
	return nil
}

// Disconnect disconnects a connector.
func (r *Registry) Disconnect(ctx context.Context, connID string) error {
	r.mu.RLock()
	handler := r.handlers[connID]
	inst := r.instances[connID]
	r.mu.RUnlock()

	if inst != nil && inst.Credentials != nil && handler != nil {
		_ = handler.Disconnect(ctx, inst.Credentials)
	}

	r.setInstance(connID, &ConnectorInstance{
		ConnectorID: connID,
		Status:      StatusDisconnected,
		LastEvent:   newConnectorEvent(connID, "", "disconnect", "ok", ""),
	})
	r.deleteCreds(connID)
	slog.Info("connector disconnected", "id", connID)
	return nil
}

// Execute runs an action on a connected connector.
func (r *Registry) Execute(ctx context.Context, connID, actionID string, params map[string]any) (any, error) {
	r.mu.RLock()
	handler, ok := r.handlers[connID]
	inst := r.instances[connID]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no handler for connector %s", connID)
	}
	if inst == nil || inst.Status != StatusConnected || inst.Credentials == nil {
		return nil, fmt.Errorf("connector %s is not connected", connID)
	}

	// Auto-refresh if expired
	if inst.Credentials.TokenExpired() {
		newCreds, err := handler.Refresh(ctx, inst.Credentials)
		if err != nil {
			r.setInstance(connID, &ConnectorInstance{
				ConnectorID: connID,
				Status:      StatusError,
				Error:       "token refresh failed: " + err.Error(),
				LastEvent:   newConnectorEvent(connID, actionID, "refresh", "error", err.Error()),
			})
			return nil, fmt.Errorf("token refresh failed: %w", err)
		}
		inst.Credentials = newCreds
		r.saveCreds(connID, newCreds)
	}

	result, err := handler.Execute(ctx, inst.Credentials, actionID, params)
	if err != nil {
		r.recordEvent(connID, newConnectorEvent(connID, actionID, "execute", "error", err.Error()))
		return nil, err
	}
	r.recordEvent(connID, newConnectorEvent(connID, actionID, "execute", "ok", ""))
	return result, nil
}

// LoadPersisted loads saved credentials from disk on startup.
func (r *Registry) LoadPersisted(ctx context.Context) {
	r.mu.RLock()
	defs := make([]string, 0, len(r.defs))
	for id := range r.defs {
		defs = append(defs, id)
	}
	r.mu.RUnlock()

	for _, connID := range defs {
		creds, err := r.loadCreds(connID)
		if err != nil || creds == nil {
			continue
		}

		r.mu.RLock()
		handler, ok := r.handlers[connID]
		r.mu.RUnlock()
		if !ok {
			continue
		}

		valid, userInfo, err := handler.Validate(ctx, creds)
		if err != nil || !valid {
			slog.Warn("connector persisted creds invalid", "id", connID, "err", err)
			continue
		}

		now := time.Now()
		r.setInstance(connID, &ConnectorInstance{
			ConnectorID: connID,
			Status:      StatusConnected,
			ConnectedAt: &now,
			UserInfo:    userInfo,
			Credentials: creds,
			LastEvent:   newConnectorEvent(connID, "", "restore", "ok", userInfo),
		})
		slog.Info("connector restored from disk", "id", connID, "user", userInfo)
	}
}

// ─── Internal helpers ────────────────────────────────

func (r *Registry) setInstance(connID string, inst *ConnectorInstance) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if inst.LastEvent == nil {
		if prev, ok := r.instances[connID]; ok {
			inst.LastEvent = cloneConnectorEvent(prev.LastEvent)
		}
	}
	r.instances[connID] = inst
}

func (r *Registry) recordEvent(connID string, event *ConnectorEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	inst, ok := r.instances[connID]
	if !ok || inst == nil {
		inst = &ConnectorInstance{ConnectorID: connID, Status: StatusDisconnected}
	}
	cp := *inst
	cp.LastEvent = cloneConnectorEvent(event)
	r.instances[connID] = &cp
}

func newConnectorEvent(connID, actionID, kind, status, message string) *ConnectorEvent {
	return &ConnectorEvent{
		Kind:        kind,
		ConnectorID: connID,
		ActionID:    actionID,
		Status:      status,
		Message:     truncateConnectorEventMessage(message),
		At:          time.Now(),
	}
}

func cloneConnectorEvent(event *ConnectorEvent) *ConnectorEvent {
	if event == nil {
		return nil
	}
	cp := *event
	return &cp
}

func truncateConnectorEventMessage(message string) string {
	message = strings.TrimSpace(message)
	if len([]rune(message)) <= 180 {
		return message
	}
	runes := []rune(message)
	return string(runes[:180]) + "..."
}

func (r *Registry) saveCreds(connID string, creds *Credentials) {
	path := filepath.Join(r.store, connID+".json")
	data, err := json.Marshal(creds)
	if err != nil {
		slog.Warn("connector save creds failed", "id", connID, "err", err)
		return
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		slog.Warn("connector write creds failed", "id", connID, "err", err)
	}
}

func (r *Registry) loadCreds(connID string) (*Credentials, error) {
	path := filepath.Join(r.store, connID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}
	return &creds, nil
}

func (r *Registry) deleteCreds(connID string) {
	path := filepath.Join(r.store, connID+".json")
	os.Remove(path)
}
