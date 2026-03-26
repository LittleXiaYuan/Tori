package planner

type AgentAction struct {
	Kind    ActionKind `json:"kind"`
	Payload any        `json:"payload"`
}

type ActionKind string

const (
	ActionAsk          ActionKind = "ask"
	ActionConfirm      ActionKind = "confirm"
	ActionShowFile     ActionKind = "show_file"
	ActionSuggest      ActionKind = "suggest"
	ActionProgress     ActionKind = "progress"
	ActionRequestInput ActionKind = "request_input"
)

type AskPayload struct {
	Question string      `json:"question"`
	Options  []AskOption `json:"options"`
	Multiple bool        `json:"multiple,omitempty"`
}

type AskOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Hint  string `json:"hint,omitempty"`
}

type ConfirmPayload struct {
	Message     string `json:"message"`
	YesLabel    string `json:"yes_label,omitempty"`
	NoLabel     string `json:"no_label,omitempty"`
	Destructive bool   `json:"destructive,omitempty"`
}

type FilePayload struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	MimeType string `json:"mime_type,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Preview  string `json:"preview,omitempty"`
}

type SuggestPayload struct {
	Suggestions []Suggestion `json:"suggestions"`
}

type Suggestion struct {
	Label  string `json:"label"`
	Prompt string `json:"prompt"`
}

type ProgressPayload struct {
	Message string `json:"message"`
	Percent int    `json:"percent,omitempty"`
	Phase   string `json:"phase,omitempty"`
}

type InputRequestPayload struct {
	Question    string `json:"question"`
	Placeholder string `json:"placeholder,omitempty"`
	InputType   string `json:"input_type,omitempty"`
}

func AskAction(question string, options ...AskOption) AgentAction {
	return AgentAction{Kind: ActionAsk, Payload: AskPayload{Question: question, Options: options}}
}

func ConfirmAction(message string, destructive bool) AgentAction {
	return AgentAction{Kind: ActionConfirm, Payload: ConfirmPayload{Message: message, Destructive: destructive}}
}

func FileAction(path, name, mimeType string, size int64) AgentAction {
	return AgentAction{Kind: ActionShowFile, Payload: FilePayload{Path: path, Name: name, MimeType: mimeType, Size: size}}
}

func SuggestAction(suggestions ...Suggestion) AgentAction {
	return AgentAction{Kind: ActionSuggest, Payload: SuggestPayload{Suggestions: suggestions}}
}
