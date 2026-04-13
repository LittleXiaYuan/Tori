package modes

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	ldg "github.com/LittleXiaYuan/ledger"
)

// ModeManager is the central coordinator for the multi-mode system.
//
// It manages per-tenant mode selection, creates and caches BehaviorEngine
// instances, persists preferences to Ledger, and records switch events
// for audit and analytics.
//
// Thread-safe: all public methods are safe for concurrent use.
type ModeManager struct {
	mu sync.RWMutex

	ledger  *ldg.Ledger
	llmCall LLMCallFunc
	locale  string

	// Per-tenant current mode. Key = tenantID, value = mode.
	tenantMode map[string]PersonaMode

	// Per-session mode override. Key = sessionID, value = mode.
	// Takes precedence over tenantMode when present.
	sessionMode map[string]PersonaMode

	// Cached engines. Key = tenantID + ":" + mode.
	engines map[string]BehaviorEngine

	// Default mode for new tenants.
	defaultMode PersonaMode
}

// NewModeManager creates a mode manager.
// The default mode is Companion (warm, principled, safe default).
func NewModeManager(l *ldg.Ledger, llmCall LLMCallFunc, locale string) *ModeManager {
	if locale == "" {
		locale = "zh"
	}
	return &ModeManager{
		ledger:      l,
		llmCall:     llmCall,
		locale:      locale,
		tenantMode:  make(map[string]PersonaMode),
		sessionMode: make(map[string]PersonaMode),
		engines:     make(map[string]BehaviorEngine),
		defaultMode: ModeCompanion,
	}
}

// SetDefaultMode changes the default mode for new tenants.
func (m *ModeManager) SetDefaultMode(mode PersonaMode) {
	if !mode.Valid() {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.defaultMode = mode
}

// SetLLMCall updates the LLM function (e.g., after provider switch).
func (m *ModeManager) SetLLMCall(fn LLMCallFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.llmCall = fn
	// Invalidate cached engines so they pick up the new LLM
	m.engines = make(map[string]BehaviorEngine)
}

// ─── Engine access ──────────────────────────────────────────────────────────

// Engine returns the BehaviorEngine for a tenant+session combination.
//
// Resolution order:
//  1. Session-level override (if sessionID is non-empty and has an override)
//  2. Tenant-level mode
//  3. Loaded from Ledger (persisted preference)
//  4. Default mode (Companion)
func (m *ModeManager) Engine(ctx context.Context, tenantID, sessionID string) BehaviorEngine {
	mode := m.resolveMode(ctx, tenantID, sessionID)
	return m.getOrCreateEngine(tenantID, mode)
}

// CurrentMode returns the active mode for a tenant+session.
func (m *ModeManager) CurrentMode(ctx context.Context, tenantID, sessionID string) PersonaMode {
	return m.resolveMode(ctx, tenantID, sessionID)
}

// ─── Mode switching ─────────────────────────────────────────────────────────

// SetMode changes the mode for a tenant. If sessionID is non-empty,
// the change only applies to that session.
func (m *ModeManager) SetMode(ctx context.Context, tenantID string, mode PersonaMode, sessionID string) error {
	if !mode.Valid() {
		return fmt.Errorf("modes: invalid mode %q", mode)
	}

	m.mu.Lock()
	oldMode := m.tenantMode[tenantID]
	if oldMode == "" {
		oldMode = m.defaultMode
	}

	if sessionID != "" {
		m.sessionMode[sessionID] = mode
	} else {
		m.tenantMode[tenantID] = mode
	}
	m.mu.Unlock()

	// Persist to ledger
	m.persistMode(ctx, tenantID, mode, sessionID)

	// Record switch event
	m.recordSwitch(ctx, SwitchEvent{
		TenantID:  tenantID,
		SessionID: sessionID,
		From:      oldMode,
		To:        mode,
		Reason:    "user_request",
		At:        time.Now(),
	})

	slog.Info("modes: switched",
		"tenant", tenantID,
		"session", sessionID,
		"from", oldMode,
		"to", mode,
	)

	return nil
}

// ClearSessionOverride removes a session-level mode override.
func (m *ModeManager) ClearSessionOverride(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessionMode, sessionID)
}

// ─── Mode listing ───────────────────────────────────────────────────────────

// ModeInfo describes a mode for API responses.
type ModeInfo struct {
	Mode        PersonaMode `json:"mode"`
	Name        string      `json:"name"`
	NameEN      string      `json:"name_en"`
	Description string      `json:"description"`
	Features    []string    `json:"features"`
	Active      bool        `json:"active"`
}

// ListModes returns all available modes with their metadata.
// The active flag is set based on the tenant's current mode.
func (m *ModeManager) ListModes(ctx context.Context, tenantID, sessionID string) []ModeInfo {
	current := m.resolveMode(ctx, tenantID, sessionID)

	var modes []ModeInfo
	for _, pm := range AllModes {
		preset := ModePresets[pm]
		modes = append(modes, ModeInfo{
			Mode:        pm,
			Name:        preset.Name,
			NameEN:      preset.NameEN,
			Description: preset.Description,
			Features:    preset.Features,
			Active:      pm == current,
		})
	}
	return modes
}

// ─── Internal resolution ────────────────────────────────────────────────────

func (m *ModeManager) resolveMode(ctx context.Context, tenantID, sessionID string) PersonaMode {
	m.mu.RLock()

	// 1. Session override
	if sessionID != "" {
		if mode, ok := m.sessionMode[sessionID]; ok {
			m.mu.RUnlock()
			return mode
		}
	}

	// 2. Tenant-level
	if mode, ok := m.tenantMode[tenantID]; ok {
		m.mu.RUnlock()
		return mode
	}

	m.mu.RUnlock()

	// 3. Try loading from ledger
	if mode := m.loadPersistedMode(ctx, tenantID); mode.Valid() {
		m.mu.Lock()
		m.tenantMode[tenantID] = mode
		m.mu.Unlock()
		return mode
	}

	// 4. Default
	m.mu.RLock()
	def := m.defaultMode
	m.mu.RUnlock()
	return def
}

func (m *ModeManager) getOrCreateEngine(tenantID string, mode PersonaMode) BehaviorEngine {
	cacheKey := tenantID + ":" + string(mode)

	m.mu.RLock()
	if engine, ok := m.engines[cacheKey]; ok {
		m.mu.RUnlock()
		return engine
	}
	m.mu.RUnlock()

	// Create new engine
	engine := m.createEngine(tenantID, mode)

	m.mu.Lock()
	m.engines[cacheKey] = engine
	m.mu.Unlock()

	return engine
}

func (m *ModeManager) createEngine(tenantID string, mode PersonaMode) BehaviorEngine {
	m.mu.RLock()
	llmCall := m.llmCall
	locale := m.locale
	l := m.ledger
	m.mu.RUnlock()

	switch mode {
	case ModeSpirit:
		return NewSpiritMode(llmCall, l, tenantID, locale)
	case ModeCompanion:
		return NewCompanionMode(llmCall, l, tenantID, locale)
	case ModeScholar:
		return NewScholarMode()
	default:
		return NewCompanionMode(llmCall, l, tenantID, locale)
	}
}

// ─── Ledger persistence ─────────────────────────────────────────────────────

const modePreferenceKey = "persona_mode"

func (m *ModeManager) persistMode(ctx context.Context, tenantID string, mode PersonaMode, sessionID string) {
	if m.ledger == nil {
		return
	}

	key := modePreferenceKey
	if sessionID != "" {
		key = fmt.Sprintf("session:%s:mode", sessionID)
	}

	if err := m.ledger.Memory.PutPreference(ctx, tenantID, key, string(mode)); err != nil {
		slog.Warn("modes: failed to persist mode", "tenant", tenantID, "err", err)
	}
}

func (m *ModeManager) loadPersistedMode(ctx context.Context, tenantID string) PersonaMode {
	if m.ledger == nil {
		return ""
	}

	results, err := m.ledger.Memory.Search(ctx, ldg.MemoryQuery{
		TenantID: tenantID,
		Query:    modePreferenceKey,
		Kinds:    []ldg.MemoryKind{ldg.MemoryPreference},
		Limit:    1,
	})
	if err != nil || len(results) == 0 {
		return ""
	}

	mode := PersonaMode(results[0].Content)
	if mode.Valid() {
		return mode
	}
	return ""
}

func (m *ModeManager) recordSwitch(ctx context.Context, evt SwitchEvent) {
	if m.ledger == nil {
		return
	}

	payload, err := json.Marshal(evt)
	if err != nil {
		return
	}

	m.ledger.Events.Append(ctx, &ldg.Event{
		TaskID:  "mode_switch:" + evt.TenantID,
		Kind:    ldg.EventKind("mode_switch"),
		Actor:   "user",
		Payload: payload,
	})
}
