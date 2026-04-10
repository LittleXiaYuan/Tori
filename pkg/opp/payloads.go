package opp

type IntentEnvelope struct {
	Name      string `json:"name"`                 // e.g. "ops.deploy"
	Version   string `json:"version"`              // semver
	SchemaURI string `json:"schema_uri,omitempty"`
	Payload   any    `json:"payload"`
}

type IntentPayload struct {
	Intent            IntentEnvelope     `json:"intent"`
	IdempotencyKey    string             `json:"idempotency_key,omitempty"`
	Priority          string             `json:"priority,omitempty"` // low|normal|high|critical
	TimeoutMs         int                `json:"timeout_ms,omitempty"`
	ModelRequirements *ModelRequirements `json:"model_requirements,omitempty"`
}

type ResultPayload struct {
	Status    string             `json:"status"` // success|partial|failed|cancelled
	Output    any                `json:"output,omitempty"`
	Artifacts []Artifact         `json:"artifacts,omitempty"`
	Metrics   map[string]float64 `json:"metrics,omitempty"`
	Error     *OPPError          `json:"error,omitempty"`
}

type Artifact struct {
	Type     string         `json:"type"` // url|file|config|service
	Name     string         `json:"name"`
	URI      string         `json:"uri,omitempty"`
	Value    any            `json:"value,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type ProblemPayload struct {
	ProblemID          string          `json:"problem_id"`
	Severity           string          `json:"severity"` // info|warning|error|critical
	Category           string          `json:"category"`
	Description        string          `json:"description"`
	Context            map[string]any  `json:"context,omitempty"`
	Options            []ProblemOption `json:"options"`
	AutoResolveAfterMs int             `json:"auto_resolve_after_ms,omitempty"`
	DefaultOption      string          `json:"default_option,omitempty"`
}

type ProblemOption struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Risk        string `json:"risk"` // safe|moderate|dangerous
}

type QuestionPayload struct {
	QuestionID string         `json:"question_id"`
	Text       string         `json:"text"`
	InputMode  map[string]any `json:"input_mode"`
	Required   bool           `json:"required"`
	Default    any            `json:"default,omitempty"`
	TimeoutMs  int            `json:"timeout_ms,omitempty"`
}

type AnswerPayload struct {
	QuestionID string `json:"question_id"`
	Value      any    `json:"value"`
}

type DecidePayload struct {
	ProblemID string `json:"problem_id"`
	Choice    string `json:"choice"`
	Reason    string `json:"reason,omitempty"`
}

type ProgressPayload struct {
	TaskID   string  `json:"task_id"`
	Phase    string  `json:"phase"`
	Progress float64 `json:"progress"` // 0.0–1.0
	Message  string  `json:"message"`
}

type HeartbeatPayload struct {
	TaskID              string  `json:"task_id"`
	State               string  `json:"state"`
	Phase               string  `json:"phase"`
	Progress            float64 `json:"progress"`
	Message             string  `json:"message"`
	ExpectedRemainingMs int     `json:"expected_remaining_ms,omitempty"`
	NextHeartbeatInMs   int     `json:"next_heartbeat_in_ms,omitempty"`
	MaxSilenceMs        int     `json:"max_silence_ms,omitempty"`
	CPU                 float64 `json:"cpu,omitempty"`
	Memory              float64 `json:"memory,omitempty"`
	DiskIO              bool    `json:"disk_io,omitempty"`
}

type NotifyPayload struct {
	Topic           string          `json:"topic"`
	Severity        string          `json:"severity"`
	Title           string          `json:"title"`
	Body            string          `json:"body"`
	Data            map[string]any  `json:"data,omitempty"`
	SuggestedIntent *IntentEnvelope `json:"suggested_intent,omitempty"`
	ExpiresAt       int64           `json:"expires_at,omitempty"`
}

// Permission declares a capability. Evaluation is the host's job.
type Permission struct {
	Action   string   `json:"action"` // e.g. "fs.read", "process.exec"
	Paths    []string `json:"paths,omitempty"`
	Commands []string `json:"commands,omitempty"`
	Cwd      []string `json:"cwd,omitempty"`
	Ports    []int    `json:"ports,omitempty"`
}

// ── Agent Network (v3) ──

// CapabilitiesPayload advertises what an agent can do, including its model
// stack, LoRA adapters, supported intents, and resource limits.
type CapabilitiesPayload struct {
	AgentID     string          `json:"agent_id"`
	DisplayName string          `json:"display_name,omitempty"`
	Version     string          `json:"version,omitempty"`
	Intents     []string        `json:"intents,omitempty"`     // supported intent names
	Skills      []string        `json:"skills,omitempty"`      // available skill names
	Models      []ModelInfo     `json:"models,omitempty"`      // model stack
	Adapters    []AdapterInfo   `json:"adapters,omitempty"`    // LoRA / fine-tuned adapters
	Permissions []Permission    `json:"permissions,omitempty"` // declared permissions
	Tags        map[string]string `json:"tags,omitempty"`      // free-form metadata
	MaxConcurrency int          `json:"max_concurrency,omitempty"`
}

// ModelInfo describes a single model available to the agent.
type ModelInfo struct {
	ID       string   `json:"id"`                  // e.g. "qwen-7b", "glm-4"
	Provider string   `json:"provider,omitempty"`   // "ollama", "vllm", "api"
	Tier     string   `json:"tier,omitempty"`       // "fast", "smart", "expert"
	Features []string `json:"features,omitempty"`   // "vision", "code", "function_calling", "long_context"
	MaxCtx   int      `json:"max_ctx,omitempty"`    // max context window tokens
	Local    bool     `json:"local,omitempty"`      // running locally vs remote API
	Quantization string `json:"quantization,omitempty"` // "q4_0", "q8_0", "fp16", "bf16"
}

// AdapterInfo describes a LoRA or fine-tuned adapter.
type AdapterInfo struct {
	ID       string   `json:"id"`                  // unique adapter identifier
	Name     string   `json:"name"`                // human-readable name
	BaseModel string  `json:"base_model"`          // which model this adapter is for
	Type     string   `json:"type"`                // "lora", "qlora", "full_ft", "prefix_tuning"
	Domain   string   `json:"domain,omitempty"`    // "finance", "legal", "medical", "code"
	Version  string   `json:"version,omitempty"`
	Rank     int      `json:"rank,omitempty"`      // LoRA rank (e.g. 8, 16, 32, 64)
	Metrics  map[string]float64 `json:"metrics,omitempty"` // training metrics: loss, accuracy, etc.
	Tags     []string `json:"tags,omitempty"`
}

// DiscoverPayload is sent to query the agent network for agents matching criteria.
type DiscoverPayload struct {
	Query       string   `json:"query,omitempty"`       // natural language query
	IntentName  string   `json:"intent_name,omitempty"` // find agents supporting this intent
	RequiredFeatures []string `json:"required_features,omitempty"` // e.g. ["vision", "code"]
	PreferLocal bool     `json:"prefer_local,omitempty"`
	PreferAdapter string `json:"prefer_adapter,omitempty"` // prefer agents with this LoRA domain
	MaxLatencyMs int     `json:"max_latency_ms,omitempty"`
}

// DelegatePayload requests another agent to execute a task.
type DelegatePayload struct {
	Intent           IntentEnvelope   `json:"intent"`
	ModelRequirements *ModelRequirements `json:"model_requirements,omitempty"`
	FallbackAgents   []string         `json:"fallback_agents,omitempty"` // try in order if primary rejects
	ContextMessages  []DelegateMsg    `json:"context,omitempty"`          // conversation context to pass
}

// DelegateMsg is a minimal message passed as context in delegation.
type DelegateMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// DelegateResultPayload wraps the result from a delegated task.
type DelegateResultPayload struct {
	DelegatedTo string        `json:"delegated_to"` // agent that actually executed
	Result      ResultPayload `json:"result"`
	ModelUsed   string        `json:"model_used,omitempty"`   // which model was used
	AdapterUsed string        `json:"adapter_used,omitempty"` // which LoRA adapter (if any)
}

// ModelRequirements specifies what kind of model the intent needs.
// Attached to INTENT or DELEGATE to enable smart routing.
type ModelRequirements struct {
	MinTier      string   `json:"min_tier,omitempty"`       // "fast", "smart", "expert"
	Features     []string `json:"features,omitempty"`       // "vision", "code", "function_calling"
	PreferLocal  bool     `json:"prefer_local,omitempty"`   // prefer local model if available
	PreferAdapter string  `json:"prefer_adapter,omitempty"` // LoRA domain preference
	MaxLatencyMs int      `json:"max_latency_ms,omitempty"`
	MaxTokens    int      `json:"max_tokens,omitempty"`     // output token budget
}

// FeedbackPayload provides post-task feedback for LoRA training data collection.
// Sent after RESULT to enable continuous learning from real task outcomes.
type FeedbackPayload struct {
	TaskID    string  `json:"task_id"`
	Rating    float64 `json:"rating"`             // 0.0–1.0 quality score
	Correct   *bool   `json:"correct,omitempty"`  // was the result correct?
	Preferred string  `json:"preferred,omitempty"` // what the ideal output would be
	Domain    string  `json:"domain,omitempty"`    // for LoRA domain classification
	Notes     string  `json:"notes,omitempty"`
	// TrainingPair holds a (input, output) example suitable for LoRA fine-tuning.
	// The host decides whether to actually use this for training.
	TrainingPair *TrainingPair `json:"training_pair,omitempty"`
}

// TrainingPair is a single training example for adapter fine-tuning.
type TrainingPair struct {
	System string `json:"system,omitempty"` // system prompt
	Input  string `json:"input"`            // user input
	Output string `json:"output"`           // ideal response
}

// SubscribePayload requests event subscriptions from another agent.
type SubscribePayload struct {
	Topics []string `json:"topics"` // event topics to subscribe to
	Filter string   `json:"filter,omitempty"` // optional event filter expression
}

// EventPayload carries a published event.
type EventPayload struct {
	Topic     string         `json:"topic"`
	Data      map[string]any `json:"data,omitempty"`
	Timestamp int64          `json:"timestamp"`
}
