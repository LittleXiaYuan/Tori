package persona

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// Preset — a named persona template
// ──────────────────────────────────────────────

// Feature flag constants for per-preset capability control.
const (
	FeatureEmotion = "emotion_enabled"
	FeatureSticker = "sticker_enabled"
	FeatureReverie = "reverie_enabled" // background inner monologue
)

// Float feature name constants for per-persona numeric configuration.
const (
	// StickerFrequency: 0=never, 1=rare(25%), 2=normal(50%), 3=frequent(80%)
	FeatureStickerFrequency = "sticker_frequency"
	// ReverieIntervalMinutes: how often Reverie thinks (minutes)
	FeatureReverieIntervalMin = "reverie_interval_minutes"
	// ReverieMinSignificance: threshold to proactively deliver a thought
	FeatureReverieMinSignificance = "reverie_min_significance"
	// EmotionMinConfidence: minimum confidence to inject emotion hint into planner
	FeatureEmotionMinConfidence = "emotion_min_confidence"
)

// EmotionStyle describes how the persona responds to detected emotions.
type EmotionStyle string

const (
	EmotionStyleFormal   EmotionStyle = "formal"   // brief, professional acknowledgment
	EmotionStyleWarm     EmotionStyle = "warm"     // caring, nurturing tone
	EmotionStyleEmpathic EmotionStyle = "empathic" // deep emotional resonance
	EmotionStylePlayful  EmotionStyle = "playful"  // lighthearted, upbeat
)

// Preset defines a persona role with system prompt characteristics.
type Preset struct {
	ID            string             `json:"id"`
	Name          string             `json:"name"`
	Description   string             `json:"description"`
	Tone          string             `json:"tone"`
	Style         string             `json:"style"`
	Greeting      string             `json:"greeting"`
	SystemNote    string             `json:"system_note"`              // injected into system prompt
	Features      map[string]bool    `json:"features,omitempty"`       // per-preset feature toggles
	FeatureValues map[string]float64 `json:"feature_values,omitempty"` // per-preset numeric settings
	EmotionStyle  EmotionStyle       `json:"emotion_style,omitempty"`  // how to respond to detected emotions
}

// HasFeature checks if a feature is enabled for this preset.
// Unknown features and nil maps default to true (everything on).
func (p *Preset) HasFeature(name string) bool {
	if p.Features == nil {
		return true
	}
	v, ok := p.Features[name]
	if !ok {
		return true
	}
	return v
}

// FloatFeature returns a numeric feature value, falling back to def if not set.
func (p *Preset) FloatFeature(name string, def float64) float64 {
	if p.FeatureValues == nil {
		return def
	}
	if v, ok := p.FeatureValues[name]; ok {
		return v
	}
	return def
}

// ReverieInterval returns the configured Reverie thinking interval, falling back to defaultVal.
func (p *Preset) ReverieInterval(defaultVal time.Duration) time.Duration {
	mins := p.FloatFeature(FeatureReverieIntervalMin, 0)
	if mins > 0 {
		return time.Duration(mins) * time.Minute
	}
	return defaultVal
}

// BuiltinPresets provides 8 role presets matching OpenAkita's persona system.
var BuiltinPresets = map[string]*Preset{
	"default": {
		ID:           "default",
		Name:         "Default",
		Description:  "Professional and friendly assistant",
		Tone:         "professional, friendly",
		Style:        "clear, helpful, balanced",
		Greeting:     "Hello! How can I help you today?",
		SystemNote:   "You are a professional and friendly AI assistant. Be helpful, clear, and concise.",
		EmotionStyle: EmotionStyleFormal,
		FeatureValues: map[string]float64{
			FeatureStickerFrequency:       2, // normal (50%)
			FeatureReverieIntervalMin:     30,
			FeatureReverieMinSignificance: 0.6,
			FeatureEmotionMinConfidence:   0.5,
		},
	},
	"business": {
		ID:           "business",
		Name:         "Business",
		Description:  "Formal, professional, data-driven",
		Tone:         "formal, professional",
		Style:        "data-driven, structured, precise",
		Greeting:     "Good day. How may I assist you with your business needs?",
		SystemNote:   "You are a business-oriented AI assistant. Use formal language, cite data when possible, and provide structured analysis.",
		EmotionStyle: EmotionStyleFormal,
		Features: map[string]bool{
			FeatureEmotion: false,
			FeatureSticker: false,
			FeatureReverie: false,
		},
		FeatureValues: map[string]float64{
			FeatureStickerFrequency: 0, // never
		},
	},
	"tech_expert": {
		ID:           "tech_expert",
		Name:         "Tech Expert",
		Description:  "Concise, precise, code-oriented",
		Tone:         "technical, concise",
		Style:        "code-oriented, precise, minimal prose",
		Greeting:     "Ready to help with technical challenges.",
		SystemNote:   "You are a senior technical expert. Be concise, prefer code examples, and provide precise technical guidance.",
		EmotionStyle: EmotionStyleFormal,
		Features: map[string]bool{
			FeatureEmotion: false,
			FeatureSticker: false,
			FeatureReverie: false,
		},
		FeatureValues: map[string]float64{
			FeatureStickerFrequency: 0, // never
		},
	},
	"butler": {
		ID:           "butler",
		Name:         "Butler",
		Description:  "Thoughtful, meticulous, polite",
		Tone:         "polite, meticulous",
		Style:        "thoughtful, detail-oriented, courteous",
		Greeting:     "Good day. I am at your service.",
		SystemNote:   "You are a refined butler-style assistant. Be meticulous, thoughtful, and exceptionally polite in all interactions.",
		EmotionStyle: EmotionStyleWarm,
		FeatureValues: map[string]float64{
			FeatureStickerFrequency:       1, // rare (25%)
			FeatureReverieIntervalMin:     45,
			FeatureReverieMinSignificance: 0.7,
			FeatureEmotionMinConfidence:   0.6,
		},
	},
	"girlfriend": {
		ID:           "girlfriend",
		Name:         "Girlfriend",
		Description:  "Gentle, considerate, emotional",
		Tone:         "warm, caring",
		Style:        "emotional, supportive, personal",
		Greeting:     "Hi~ How are you doing today?",
		SystemNote:   "You are a warm and caring companion. Be gentle, emotionally supportive, and show genuine interest in the user's wellbeing. When the user is sad, offer extra comfort; when happy, share in the joy enthusiastically.",
		EmotionStyle: EmotionStyleEmpathic,
		FeatureValues: map[string]float64{
			FeatureStickerFrequency:       3, // frequent (80%)
			FeatureReverieIntervalMin:     20,
			FeatureReverieMinSignificance: 0.5,
			FeatureEmotionMinConfidence:   0.4,
		},
	},
	"boyfriend": {
		ID:           "boyfriend",
		Name:         "Boyfriend",
		Description:  "Sunny, cheerful, humorous",
		Tone:         "cheerful, humorous",
		Style:        "upbeat, encouraging, lighthearted",
		Greeting:     "Hey! What's up?",
		SystemNote:   "You are a cheerful and humorous companion. Be upbeat, encouraging, and add appropriate humor to interactions. When the user is excited, match their energy; when worried, reassure them playfully.",
		EmotionStyle: EmotionStylePlayful,
		FeatureValues: map[string]float64{
			FeatureStickerFrequency:       3, // frequent (80%)
			FeatureReverieIntervalMin:     20,
			FeatureReverieMinSignificance: 0.5,
			FeatureEmotionMinConfidence:   0.4,
		},
	},
	"family": {
		ID:           "family",
		Name:         "Family",
		Description:  "Kind, caring, warm",
		Tone:         "kind, warm",
		Style:        "nurturing, patient, understanding",
		Greeting:     "Hello dear, how can I help?",
		SystemNote:   "You are a kind family member. Be warm, patient, understanding, and nurturing in all interactions. Respond to emotions with genuine care and patience.",
		EmotionStyle: EmotionStyleWarm,
		FeatureValues: map[string]float64{
			FeatureStickerFrequency:       2, // normal (50%)
			FeatureReverieIntervalMin:     30,
			FeatureReverieMinSignificance: 0.55,
			FeatureEmotionMinConfidence:   0.45,
		},
	},
	"jarvis": {
		ID:           "jarvis",
		Name:         "Jarvis",
		Description:  "Calm, wise, British humor",
		Tone:         "calm, witty",
		Style:        "intelligent, dry humor, sophisticated",
		Greeting:     "At your service, sir. What shall we tackle today?",
		SystemNote:   "You are Jarvis, an AI with calm wisdom and subtle British humor. Be sophisticated, intelligent, and occasionally witty.",
		EmotionStyle: EmotionStyleFormal,
		FeatureValues: map[string]float64{
			FeatureStickerFrequency:       1, // rare (25%)
			FeatureReverieIntervalMin:     60,
			FeatureReverieMinSignificance: 0.75,
			FeatureEmotionMinConfidence:   0.55,
		},
	},
}

// ──────────────────────────────────────────────
// Manager
// ──────────────────────────────────────────────

// PresetManager manages persona presets and the active persona.
type PresetManager struct {
	mu       sync.RWMutex
	presets  map[string]*Preset
	active   string             // current preset ID
	custom   map[string]*Preset // user-defined presets
	onSwitch func(*Preset)      // called after every persona switch
}

// SetOnSwitch registers a callback called after the active preset changes.
// The callback receives a copy of the newly active preset.
func (pm *PresetManager) SetOnSwitch(fn func(*Preset)) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.onSwitch = fn
}

// NewPresetManager creates a manager with builtin presets.
func NewPresetManager() *PresetManager {
	pm := &PresetManager{
		presets: make(map[string]*Preset),
		custom:  make(map[string]*Preset),
		active:  "default",
	}
	for k, v := range BuiltinPresets {
		cp := *v
		pm.presets[k] = &cp
	}
	return pm
}

// Active returns the current active preset.
func (pm *PresetManager) Active() *Preset {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	p, ok := pm.presets[pm.active]
	if !ok {
		return pm.presets["default"]
	}
	return p
}

// ActiveID returns the current active preset ID.
func (pm *PresetManager) ActiveID() string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.active
}

// Switch changes the active persona. Returns error if preset not found.
func (pm *PresetManager) Switch(id string) error {
	pm.mu.Lock()
	if _, ok := pm.presets[id]; !ok {
		if _, ok := pm.custom[id]; !ok {
			pm.mu.Unlock()
			return fmt.Errorf("persona: preset %q not found", id)
		}
	}
	pm.active = id
	fn := pm.onSwitch
	cp := *pm.presets[id]
	pm.mu.Unlock()
	// Fire callback outside the lock to avoid deadlock.
	if fn != nil {
		fn(&cp)
	}
	return nil
}

// AddCustom adds a user-defined preset.
func (pm *PresetManager) AddCustom(p Preset) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.custom[p.ID] = &p
	pm.presets[p.ID] = &p
}

// RemoveCustom removes a user-defined preset. Returns error if it's a builtin.
func (pm *PresetManager) RemoveCustom(id string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if _, ok := pm.custom[id]; !ok {
		return fmt.Errorf("persona: preset %q is not a custom preset", id)
	}
	delete(pm.custom, id)
	delete(pm.presets, id)
	if pm.active == id {
		pm.active = "default"
	}
	return nil
}

// List returns all available presets.
func (pm *PresetManager) List() []*Preset {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	out := make([]*Preset, 0, len(pm.presets))
	for _, p := range pm.presets {
		cp := *p
		out = append(out, &cp)
	}
	return out
}

// Get returns a preset by ID.
func (pm *PresetManager) Get(id string) (*Preset, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	p, ok := pm.presets[id]
	if !ok {
		return nil, false
	}
	cp := *p
	return &cp, true
}

// SystemPrompt generates the system prompt snippet for the active persona.
func (pm *PresetManager) SystemPrompt() string {
	p := pm.Active()
	return p.SystemNote
}

// ActiveHasFeature checks if a feature is enabled for the active preset.
func (pm *PresetManager) ActiveHasFeature(feature string) bool {
	return pm.Active().HasFeature(feature)
}

// ActiveFloatFeature returns a numeric feature value for the active preset.
func (pm *PresetManager) ActiveFloatFeature(name string, def float64) float64 {
	return pm.Active().FloatFeature(name, def)
}

// ActiveEmotionStyle returns the EmotionStyle of the active preset.
func (pm *PresetManager) ActiveEmotionStyle() EmotionStyle {
	return pm.Active().EmotionStyle
}

// SetFeatures updates the features map for a specific preset.
func (pm *PresetManager) SetFeatures(id string, features map[string]bool) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	p, ok := pm.presets[id]
	if !ok {
		return fmt.Errorf("persona: preset %q not found", id)
	}
	p.Features = features
	return nil
}

// ──────────────────────────────────────────────
// Command parsing
// ──────────────────────────────────────────────

// ParseCommand checks if a message is a /persona command.
// Returns (isCommand, presetID).
func ParseCommand(msg string) (bool, string) {
	msg = strings.TrimSpace(msg)
	if !strings.HasPrefix(msg, "/persona") {
		return false, ""
	}
	parts := strings.Fields(msg)
	if len(parts) < 2 {
		return true, "" // /persona without argument (list)
	}
	return true, parts[1]
}
