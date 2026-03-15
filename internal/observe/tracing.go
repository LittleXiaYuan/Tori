package observe

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Span represents a trace span for request-level observability.
type Span struct {
	TraceID   string            `json:"trace_id"`
	SpanID    string            `json:"span_id"`
	ParentID  string            `json:"parent_id,omitempty"`
	Name      string            `json:"name"`
	StartTime time.Time         `json:"start_time"`
	EndTime   time.Time         `json:"end_time,omitempty"`
	Duration  time.Duration     `json:"duration_ms,omitempty"`
	Status    string            `json:"status"` // "ok", "error"
	Attrs     map[string]string `json:"attrs,omitempty"`
	Error     string            `json:"error,omitempty"`
}

type traceContextKey struct{}

// TraceContext carries trace info through context.
type TraceContext struct {
	TraceID string
	SpanID  string
}

var spanCounter uint64
var spanMu sync.Mutex

func nextSpanID() string {
	spanMu.Lock()
	spanCounter++
	id := spanCounter
	spanMu.Unlock()
	return fmt.Sprintf("span-%d-%d", time.Now().UnixMilli(), id)
}

// StartTrace creates a new root trace and returns a context with trace info.
func StartTrace(ctx context.Context, name string) (context.Context, *Span) {
	traceID := fmt.Sprintf("trace-%d", time.Now().UnixNano())
	span := &Span{
		TraceID:   traceID,
		SpanID:    nextSpanID(),
		Name:      name,
		StartTime: time.Now(),
		Status:    "ok",
		Attrs:     make(map[string]string),
	}
	tc := &TraceContext{TraceID: traceID, SpanID: span.SpanID}
	return context.WithValue(ctx, traceContextKey{}, tc), span
}

// StartSpan creates a child span under the current trace context.
func StartSpan(ctx context.Context, name string) (context.Context, *Span) {
	tc, ok := ctx.Value(traceContextKey{}).(*TraceContext)
	if !ok {
		return StartTrace(ctx, name)
	}
	span := &Span{
		TraceID:   tc.TraceID,
		SpanID:    nextSpanID(),
		ParentID:  tc.SpanID,
		Name:      name,
		StartTime: time.Now(),
		Status:    "ok",
		Attrs:     make(map[string]string),
	}
	childTC := &TraceContext{TraceID: tc.TraceID, SpanID: span.SpanID}
	return context.WithValue(ctx, traceContextKey{}, childTC), span
}

// EndSpan finalizes a span and logs it.
func EndSpan(span *Span, err error) {
	span.EndTime = time.Now()
	span.Duration = span.EndTime.Sub(span.StartTime)
	if err != nil {
		span.Status = "error"
		span.Error = err.Error()
	}
	slog.Info("trace.span",
		"trace_id", span.TraceID,
		"span_id", span.SpanID,
		"parent_id", span.ParentID,
		"name", span.Name,
		"duration_ms", span.Duration.Milliseconds(),
		"status", span.Status,
		"error", span.Error,
	)
}

// TraceIDFromContext extracts the trace ID from context.
func TraceIDFromContext(ctx context.Context) string {
	tc, ok := ctx.Value(traceContextKey{}).(*TraceContext)
	if !ok {
		return ""
	}
	return tc.TraceID
}
