package main

import "time"

// ── Application Constants ──
// Extracted from magic numbers spread across the codebase.

const (
	// LLM temperature settings
	DefaultLLMTemperature = 0.7
	LowLLMTemperature     = 0.3
	MinimalLLMTemperature = 0.1
	IterateLLMTemperature = 0.5

	// Context window
	MaxContextTokens = 128000
	MaxContextTurns  = 50

	// Memory
	ShortTermMemoryDuration = 30 * time.Minute
	MemoryGCInterval        = 5 * time.Minute
	MemoryPromoteInterval   = 10 * time.Minute

	// Session / Inbox
	DefaultSessionCapacity = 50
	DefaultInboxCapacity   = 500

	// Emotion
	DefaultEmotionHistoryCapacity = 2000
	EmotionConfidenceThreshold    = 0.5

	// Reverie
	ReverieThoughtMaxRunes = 200
	ReverieCooldownMinutes = 5
	ReverieMinSignificance = 0.6

	// Knowledge
	DefaultKnowledgeChunkSize = 800
	DefaultKnowledgeTopK      = 10
	DefaultCodeTopK           = 20
	MaxKnowledgeResults       = 5

	// Network / HTTP
	DefaultHTTPReadTimeout = 30 * time.Second
	// WriteTimeout caps the entire response write, including long SSE streams
	// (agentic chat). Multi-subagent tasks — research_exec then file_exec
	// generating PPT/Word — can stream for several minutes; the old 150s cut the
	// stream mid-task before the deliverable arrived. 900s comfortably covers a
	// chained multi-agent run while still bounding stuck connections.
	DefaultHTTPWriteTimeout = 900 * time.Second
	DefaultHTTPIdleTimeout  = 60 * time.Second
	GracefulShutdownTimeout = 15 * time.Second

	// Circuit breaker
	BreakerFailureThreshold = 3
	BreakerRecoveryTime     = 30 * time.Second
	BreakerHalfOpenMax      = 2

	// Misc
	DefaultRateLimit        = 30
	LocalProbeTimeout       = 3 * time.Second
	LearningLoopTimeout     = 30 * time.Second
	PluginWatchInterval     = 5 * time.Second
	MCPConnectTimeout       = 30 * time.Second
)
