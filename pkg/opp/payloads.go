package opp

type IntentEnvelope struct {
	Name      string `json:"name"`                 // e.g. "ops.deploy"
	Version   string `json:"version"`              // semver
	SchemaURI string `json:"schema_uri,omitempty"`
	Payload   any    `json:"payload"`
}

type IntentPayload struct {
	Intent         IntentEnvelope `json:"intent"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
	Priority       string         `json:"priority,omitempty"` // low|normal|high|critical
	TimeoutMs      int            `json:"timeout_ms,omitempty"`
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
