package opp

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

func newMsg(src, dst, sess string, t MessageType, payload any) *Message {
	var raw json.RawMessage
	if payload != nil {
		b, _ := json.Marshal(payload)
		raw = b
	} else {
		raw = json.RawMessage("{}")
	}
	return &Message{
		V:         Version,
		ID:        uuid.New().String(),
		SessionID: sess,
		Source:    src,
		Target:    dst,
		Type:      t,
		Payload:   raw,
		Timestamp: time.Now().UnixMilli(),
	}
}

func (m *Message) WithTask(id string) *Message    { m.TaskID = id; return m }
func (m *Message) WithReplyTo(id string) *Message  { m.ReplyTo = id; return m }
func (m *Message) WithTrace(trace, span, parent string) *Message {
	m.TraceID = trace
	m.SpanID = span
	m.ParentSpanID = parent
	return m
}

func NewIntent(src, dst, sess string, intent IntentEnvelope) *Message {
	return newMsg(src, dst, sess, MsgIntent, IntentPayload{Intent: intent})
}

func NewAccept(src, dst, sess, task string) *Message {
	return newMsg(src, dst, sess, MsgAccept, nil).WithTask(task)
}

func NewReject(src, dst, sess, task, reason string) *Message {
	return newMsg(src, dst, sess, MsgReject, map[string]string{"reason": reason}).WithTask(task)
}

func NewResult(src, dst, sess, task, status string, output any, err *OPPError) *Message {
	return newMsg(src, dst, sess, MsgResult, ResultPayload{Status: status, Output: output, Error: err}).WithTask(task)
}

func NewQuestion(src, dst, sess, task string, q QuestionPayload) *Message {
	if q.QuestionID == "" {
		q.QuestionID = uuid.New().String()
	}
	return newMsg(src, dst, sess, MsgQuestion, q).WithTask(task)
}

func NewAnswer(src, dst, sess, task, questionID string, value any) *Message {
	return newMsg(src, dst, sess, MsgAnswer, AnswerPayload{QuestionID: questionID, Value: value}).WithTask(task)
}

func NewProblem(src, dst, sess, task string, p ProblemPayload) *Message {
	if p.ProblemID == "" {
		p.ProblemID = uuid.New().String()
	}
	return newMsg(src, dst, sess, MsgProblem, p).WithTask(task)
}

func NewDecide(src, dst, sess, task, problemID, choice, reason string) *Message {
	return newMsg(src, dst, sess, MsgDecide, DecidePayload{ProblemID: problemID, Choice: choice, Reason: reason}).WithTask(task)
}

func NewProgress(src, dst, sess, task, phase string, pct float64, msg string) *Message {
	return newMsg(src, dst, sess, MsgProgress, ProgressPayload{TaskID: task, Phase: phase, Progress: pct, Message: msg}).WithTask(task)
}

func NewHeartbeat(src, dst, sess, task string, hb HeartbeatPayload) *Message {
	hb.TaskID = task
	return newMsg(src, dst, sess, MsgHeartbeat, hb).WithTask(task)
}

func NewError(src, dst, sess string, code ErrorCode, msg string, retryable bool) *Message {
	return newMsg(src, dst, sess, MsgError, OPPError{Code: code, Message: msg, Retryable: retryable})
}

// ── Agent Network (v3) builders ──

func NewCapabilities(src, dst, sess string, caps CapabilitiesPayload) *Message {
	return newMsg(src, dst, sess, MsgCapabilities, caps)
}

func NewDiscover(src, dst, sess string, d DiscoverPayload) *Message {
	return newMsg(src, dst, sess, MsgDiscover, d)
}

func NewDelegate(src, dst, sess string, d DelegatePayload) *Message {
	return newMsg(src, dst, sess, MsgDelegate, d)
}

func NewDelegateResult(src, dst, sess, task string, dr DelegateResultPayload) *Message {
	return newMsg(src, dst, sess, MsgDelegateResult, dr).WithTask(task)
}

func NewFeedback(src, dst, sess, task string, fb FeedbackPayload) *Message {
	fb.TaskID = task
	return newMsg(src, dst, sess, MsgFeedback, fb).WithTask(task)
}

func NewNotify(src, dst, sess string, n NotifyPayload) *Message {
	return newMsg(src, dst, sess, MsgNotify, n)
}

func NewSubscribe(src, dst, sess string, topics []string) *Message {
	return newMsg(src, dst, sess, MsgSubscribe, SubscribePayload{Topics: topics})
}

func NewEvent(src, dst, sess, topic string, data map[string]any) *Message {
	return newMsg(src, dst, sess, MsgEvent, EventPayload{Topic: topic, Data: data, Timestamp: time.Now().UnixMilli()})
}

// NewIntentWithModel creates an INTENT with model routing requirements.
func NewIntentWithModel(src, dst, sess string, intent IntentEnvelope, reqs ModelRequirements) *Message {
	return newMsg(src, dst, sess, MsgIntent, IntentPayload{
		Intent:            intent,
		ModelRequirements: &reqs,
	})
}
