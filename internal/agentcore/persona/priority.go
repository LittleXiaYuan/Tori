package persona

import (
	"log/slog"
	"sync"
)

// PriorityLevel defines the precedence of persona overrides (lower = higher priority).
type PriorityLevel int

const (
	PrioritySession      PriorityLevel = 0 // Highest — per-session forced persona
	PriorityConversation PriorityLevel = 1 // Per-conversation persona override
	PriorityPreset       PriorityLevel = 2 // Active preset from PresetManager
	PriorityDefault      PriorityLevel = 3 // Lowest — base Persona (IDENTITY.md + SOUL.md)
)

// Override represents a persona override at a specific level.
type Override struct {
	Level      PriorityLevel `json:"level"`
	SystemNote string        `json:"system_note"`
	Source     string        `json:"source"` // e.g. "preset:jarvis", "session:abc123", "api"
}

// PriorityChain resolves the effective persona from multiple override layers.
// Resolution order: session (per-session forced) > conversation (per-conversation) > preset > default.
type PriorityChain struct {
	mu            sync.RWMutex
	base          *Persona             // default (IDENTITY.md + SOUL.md + skills)
	presets       *PresetManager       // preset layer
	conversations map[string]*Override // conversationID → override
	sessions      map[string]*Override // sessionID → override
}

// NewPriorityChain creates a priority chain backed by a base persona and preset manager.
func NewPriorityChain(base *Persona, presets *PresetManager) *PriorityChain {
	return &PriorityChain{
		base:          base,
		presets:       presets,
		conversations: make(map[string]*Override),
		sessions:      make(map[string]*Override),
	}
}

// SetSessionOverride forces a persona for a specific session (highest priority).
func (pc *PriorityChain) SetSessionOverride(sessionID, systemNote, source string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.sessions[sessionID] = &Override{
		Level:      PrioritySession,
		SystemNote: systemNote,
		Source:     source,
	}
	slog.Info("persona: session override set", "session", sessionID, "source", source)
}

// ClearSessionOverride removes the per-session persona override.
func (pc *PriorityChain) ClearSessionOverride(sessionID string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	delete(pc.sessions, sessionID)
}

// SetConversationOverride sets a persona for a specific conversation.
func (pc *PriorityChain) SetConversationOverride(conversationID, systemNote, source string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.conversations[conversationID] = &Override{
		Level:      PriorityConversation,
		SystemNote: systemNote,
		Source:     source,
	}
	slog.Info("persona: conversation override set", "conversation", conversationID, "source", source)
}

// ClearConversationOverride removes the per-conversation persona override.
func (pc *PriorityChain) ClearConversationOverride(conversationID string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	delete(pc.conversations, conversationID)
}

// Resolve returns the effective system prompt for a given session and conversation.
// It walks the priority chain from highest to lowest:
//  1. Session override (if set for this sessionID)
//  2. Conversation override (if set for this conversationID)
//  3. Active preset (from PresetManager)
//  4. Base persona (IDENTITY.md + SOUL.md + skills)
func (pc *PriorityChain) Resolve(sessionID, conversationID string) string {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	// Level 0: session override
	if sessionID != "" {
		if ov, ok := pc.sessions[sessionID]; ok && ov.SystemNote != "" {
			return ov.SystemNote
		}
	}

	// Level 1: conversation override
	if conversationID != "" {
		if ov, ok := pc.conversations[conversationID]; ok && ov.SystemNote != "" {
			return ov.SystemNote
		}
	}

	// Level 2: active preset
	if pc.presets != nil {
		if note := pc.presets.SystemPrompt(); note != "" {
			return note
		}
	}

	// Level 3: base persona
	if pc.base != nil {
		return pc.base.SystemPrompt()
	}

	return ""
}

// Presets returns the underlying PresetManager (may be nil).
func (pc *PriorityChain) Presets() *PresetManager {
	return pc.presets
}

// FeatureEnabled checks if a feature is enabled for the active preset.
// Returns true when no PresetManager is configured (fail-open).
func (pc *PriorityChain) FeatureEnabled(feature string) bool {
	if pc.presets == nil {
		return true
	}
	return pc.presets.ActiveHasFeature(feature)
}

// FloatFeature returns a numeric feature value from the active preset.
// Returns def when no PresetManager is configured.
func (pc *PriorityChain) FloatFeature(name string, def float64) float64 {
	if pc.presets == nil {
		return def
	}
	return pc.presets.ActiveFloatFeature(name, def)
}

// ActivePreset returns the current active preset (may be nil if no PresetManager).
func (pc *PriorityChain) ActivePreset() *Preset {
	if pc.presets == nil {
		return nil
	}
	return pc.presets.Active()
}

// ActiveEmotionStyle returns the EmotionStyle of the active preset.
func (pc *PriorityChain) ActiveEmotionStyle() EmotionStyle {
	if pc.presets == nil {
		return EmotionStyleFormal
	}
	return pc.presets.ActiveEmotionStyle()
}

// ResolveWithLevel returns both the effective prompt and which priority level it came from.
func (pc *PriorityChain) ResolveWithLevel(sessionID, conversationID string) (string, PriorityLevel) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	if sessionID != "" {
		if ov, ok := pc.sessions[sessionID]; ok && ov.SystemNote != "" {
			return ov.SystemNote, PrioritySession
		}
	}

	if conversationID != "" {
		if ov, ok := pc.conversations[conversationID]; ok && ov.SystemNote != "" {
			return ov.SystemNote, PriorityConversation
		}
	}

	if pc.presets != nil {
		if note := pc.presets.SystemPrompt(); note != "" {
			return note, PriorityPreset
		}
	}

	if pc.base != nil {
		if prompt := pc.base.SystemPrompt(); prompt != "" {
			return prompt, PriorityDefault
		}
	}

	return "", PriorityDefault
}

// SystemPromptFunc returns a closure suitable for planner.SetPersonaPrompt().
// It resolves using the default (no session/conversation context) — for backward compatibility.
func (pc *PriorityChain) SystemPromptFunc() func() string {
	return func() string {
		return pc.Resolve("", "")
	}
}

// ActiveOverrides returns all currently active overrides for debugging.
func (pc *PriorityChain) ActiveOverrides() map[string]*Override {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	result := make(map[string]*Override)
	for k, v := range pc.sessions {
		cp := *v
		result["session:"+k] = &cp
	}
	for k, v := range pc.conversations {
		cp := *v
		result["conversation:"+k] = &cp
	}
	return result
}

// ClearAll removes all session and conversation overrides.
func (pc *PriorityChain) ClearAll() {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.sessions = make(map[string]*Override)
	pc.conversations = make(map[string]*Override)
}
