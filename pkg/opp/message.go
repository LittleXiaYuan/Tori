package opp

import (
	"encoding/json"
	"fmt"
)

// Message is the OPP envelope. Payload uses json.RawMessage so callers
// decode it into the concrete type matching msg.Type.
type Message struct {
	V         int             `json:"v"`
	ID        string          `json:"id"`
	SessionID string          `json:"session_id"`
	TaskID    string          `json:"task_id,omitempty"`
	Source    string          `json:"source"`
	Target    string          `json:"target"`
	Type      MessageType     `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp int64           `json:"ts"`
	ReplyTo   string          `json:"reply_to,omitempty"`

	TraceID      string `json:"trace_id,omitempty"`
	SpanID       string `json:"span_id,omitempty"`
	ParentSpanID string `json:"parent_span_id,omitempty"`
}

func ParseMessage(data []byte) (*Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("opp: parse: %w", err)
	}
	return &msg, nil
}

func (m *Message) Bytes() ([]byte, error) { return json.Marshal(m) }

func (m *Message) DecodePayload(dst any) error {
	if len(m.Payload) == 0 {
		return nil
	}
	return json.Unmarshal(m.Payload, dst)
}

func (m *Message) DecodeIntent() (*IntentPayload, error) {
	var p IntentPayload
	return &p, m.DecodePayload(&p)
}

func (m *Message) DecodeResult() (*ResultPayload, error) {
	var p ResultPayload
	return &p, m.DecodePayload(&p)
}

func (m *Message) DecodeProblem() (*ProblemPayload, error) {
	var p ProblemPayload
	return &p, m.DecodePayload(&p)
}

func (m *Message) DecodeQuestion() (*QuestionPayload, error) {
	var p QuestionPayload
	return &p, m.DecodePayload(&p)
}

func (m *Message) DecodeAnswer() (*AnswerPayload, error) {
	var p AnswerPayload
	return &p, m.DecodePayload(&p)
}

func (m *Message) DecodeDecide() (*DecidePayload, error) {
	var p DecidePayload
	return &p, m.DecodePayload(&p)
}

func (m *Message) DecodeProgress() (*ProgressPayload, error) {
	var p ProgressPayload
	return &p, m.DecodePayload(&p)
}

func (m *Message) DecodeHeartbeat() (*HeartbeatPayload, error) {
	var p HeartbeatPayload
	return &p, m.DecodePayload(&p)
}

func (m *Message) DecodeNotify() (*NotifyPayload, error) {
	var p NotifyPayload
	return &p, m.DecodePayload(&p)
}

func (m *Message) DecodeError() (*OPPError, error) {
	var p OPPError
	return &p, m.DecodePayload(&p)
}

func (m *Message) DecodeCapabilities() (*CapabilitiesPayload, error) {
	var p CapabilitiesPayload
	return &p, m.DecodePayload(&p)
}

func (m *Message) DecodeDiscover() (*DiscoverPayload, error) {
	var p DiscoverPayload
	return &p, m.DecodePayload(&p)
}

func (m *Message) DecodeDelegate() (*DelegatePayload, error) {
	var p DelegatePayload
	return &p, m.DecodePayload(&p)
}

func (m *Message) DecodeDelegateResult() (*DelegateResultPayload, error) {
	var p DelegateResultPayload
	return &p, m.DecodePayload(&p)
}

func (m *Message) DecodeFeedback() (*FeedbackPayload, error) {
	var p FeedbackPayload
	return &p, m.DecodePayload(&p)
}

func (m *Message) DecodeSubscribe() (*SubscribePayload, error) {
	var p SubscribePayload
	return &p, m.DecodePayload(&p)
}

func (m *Message) DecodeEvent() (*EventPayload, error) {
	var p EventPayload
	return &p, m.DecodePayload(&p)
}
