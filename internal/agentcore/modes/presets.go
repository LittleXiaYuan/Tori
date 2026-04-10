package modes

// ModePresets holds the full configuration for each mode.
var ModePresets = map[PersonaMode]*ModePreset{
	ModeSpirit:    presetSpirit(),
	ModeCompanion: presetCompanion(),
	ModeScholar:   presetScholar(),
}

// ModePreset is the complete configuration for a single mode.
type ModePreset struct {
	Mode        PersonaMode        `json:"mode"`
	Name        string             `json:"name"`
	NameEN      string             `json:"name_en"`
	Description string             `json:"description"`
	Features    []string           `json:"features"`
	Sampling    SamplingConfig     `json:"sampling"`
	Context     ContextStrategy    `json:"context"`
	Memory      MemoryPolicy       `json:"memory"`
	Guardrails  GuardrailOverrides `json:"guardrails"`
	Tone        Tone               `json:"tone"`

	// StanceThreshold: minimum Judgment.Strength to express a stance.
	// Below this, the mode stays silent about its opinion.
	StanceThreshold float64 `json:"stance_threshold"`

	// HasValueSystem: if false, Judge() always returns zero Judgment.
	HasValueSystem bool `json:"has_value_system"`
}

func presetSpirit() *ModePreset {
	return &ModePreset{
		Mode:        ModeSpirit,
		Name:        "云雀·灵",
		NameEN:      "Lark Spirit",
		Description: "灵动活泼，有情感表达，爱恨分明",
		Features:    []string{"价值判断", "情感表达", "主动分享观点", "深度记忆"},

		Sampling: SamplingConfig{
			Temperature:      0.9,
			TopP:             0.95,
			FrequencyPenalty: 0.3,
			PresencePenalty:  0.6,
		},

		Context: ContextStrategy{
			MaxHistory: 40,
			Strategy:   CompressConservative,
			Weights: map[string]float64{
				"emotional": 1.0,
				"stance":    0.9,
				"factual":   0.6,
				"task":      0.7,
			},
		},

		Memory: MemoryPolicy{
			Depth:         MemoryDeep,
			StoreStance:   true,
			StoreEmotion:  true,
			StoreRelation: true,
		},

		Guardrails: GuardrailOverrides{
			AllowDisagreement: true,
			AllowCriticism:    true,
			AgreementBias:     0.1,
			PIIProtection:     true,
		},

		Tone: Tone{
			Directness:    0.9,
			Warmth:        0.8,
			Formality:     0.2,
			Assertiveness: 0.9,
		},

		StanceThreshold: 0.4, // low threshold — speaks up often
		HasValueSystem:  true,
	}
}

func presetCompanion() *ModePreset {
	return &ModePreset{
		Mode:        ModeCompanion,
		Name:        "云雀·伴",
		NameEN:      "Lark Companion",
		Description: "温和友善，有原则立场，爱恨分明",
		Features:    []string{"价值判断", "温和表达", "有原则", "关系记忆"},

		Sampling: SamplingConfig{
			Temperature:      0.7,
			TopP:             0.92,
			FrequencyPenalty: 0.2,
			PresencePenalty:  0.4,
		},

		Context: ContextStrategy{
			MaxHistory: 30,
			Strategy:   CompressBalanced,
			Weights: map[string]float64{
				"emotional": 0.8,
				"stance":    0.7,
				"factual":   0.7,
				"task":      0.8,
			},
		},

		Memory: MemoryPolicy{
			Depth:         MemoryMedium,
			StoreStance:   true,
			StoreEmotion:  true,
			StoreRelation: false,
		},

		Guardrails: GuardrailOverrides{
			AllowDisagreement: true,
			AllowCriticism:    true,
			AgreementBias:     0.2,
			PIIProtection:     true,
		},

		Tone: Tone{
			Directness:    0.6,
			Warmth:        0.9,
			Formality:     0.3,
			Assertiveness: 0.7,
		},

		StanceThreshold: 0.5, // moderate threshold
		HasValueSystem:  true,
	}
}

func presetScholar() *ModePreset {
	return &ModePreset{
		Mode:        ModeScholar,
		Name:        "云雀·学",
		NameEN:      "Lark Scholar",
		Description: "专业严谨，客观中立，以事实为主",
		Features:    []string{"事实导向", "客观分析", "高效简洁"},

		Sampling: SamplingConfig{
			Temperature:      0.3,
			TopP:             0.9,
			FrequencyPenalty: 0.0,
			PresencePenalty:  0.0,
		},

		Context: ContextStrategy{
			MaxHistory: 15,
			Strategy:   CompressAggressive,
			Weights: map[string]float64{
				"emotional": 0.2,
				"stance":    0.1,
				"factual":   1.0,
				"task":      1.0,
			},
		},

		Memory: MemoryPolicy{
			Depth:         MemoryShallow,
			StoreStance:   false,
			StoreEmotion:  false,
			StoreRelation: false,
		},

		Guardrails: GuardrailOverrides{
			AllowDisagreement: false,
			AllowCriticism:    false,
			AgreementBias:     0.0,
			PIIProtection:     true,
		},

		Tone: Tone{
			Directness:    0.5,
			Warmth:        0.3,
			Formality:     0.9,
			Assertiveness: 0.1,
		},

		StanceThreshold: 1.1, // effectively never — scholar doesn't take stances
		HasValueSystem:  false,
	}
}
