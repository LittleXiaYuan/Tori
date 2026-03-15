package observe

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestStartTrace_CreatesRootSpan(t *testing.T) {
	ctx, span := StartTrace(context.Background(), "test.root")
	if span.TraceID == "" {
		t.Fatal("expected non-empty trace ID")
	}
	if span.SpanID == "" {
		t.Fatal("expected non-empty span ID")
	}
	if span.ParentID != "" {
		t.Fatal("root span should have no parent")
	}
	if span.Name != "test.root" {
		t.Fatalf("expected name test.root, got %s", span.Name)
	}
	if span.Status != "ok" {
		t.Fatalf("expected status ok, got %s", span.Status)
	}
	if span.Attrs == nil {
		t.Fatal("attrs should be initialized")
	}

	// Context should carry trace info
	traceID := TraceIDFromContext(ctx)
	if traceID != span.TraceID {
		t.Fatalf("context trace ID mismatch: %s vs %s", traceID, span.TraceID)
	}
}

func TestStartSpan_CreatesChildSpan(t *testing.T) {
	ctx, root := StartTrace(context.Background(), "root")
	_, child := StartSpan(ctx, "child")

	if child.TraceID != root.TraceID {
		t.Fatalf("child should share root trace ID")
	}
	if child.ParentID != root.SpanID {
		t.Fatalf("child parent should be root span ID, got %s", child.ParentID)
	}
	if child.SpanID == root.SpanID {
		t.Fatal("child should have different span ID")
	}
}

func TestStartSpan_WithoutTrace_CreatesRoot(t *testing.T) {
	_, span := StartSpan(context.Background(), "orphan")
	if span.ParentID != "" {
		t.Fatal("orphan span should have no parent")
	}
	if !strings.HasPrefix(span.TraceID, "trace-") {
		t.Fatalf("should auto-create trace ID, got %s", span.TraceID)
	}
}

func TestStartSpan_DeepNesting(t *testing.T) {
	ctx, root := StartTrace(context.Background(), "root")
	ctx, mid := StartSpan(ctx, "mid")
	_, leaf := StartSpan(ctx, "leaf")

	if leaf.TraceID != root.TraceID {
		t.Fatal("leaf should share root trace ID")
	}
	if leaf.ParentID != mid.SpanID {
		t.Fatal("leaf parent should be mid span")
	}
}

func TestEndSpan_Success(t *testing.T) {
	_, span := StartTrace(context.Background(), "test")
	EndSpan(span, nil)
	if span.Status != "ok" {
		t.Fatalf("expected ok, got %s", span.Status)
	}
	if span.Error != "" {
		t.Fatalf("expected empty error, got %s", span.Error)
	}
	if span.Duration < 0 {
		t.Fatal("duration should be non-negative")
	}
	if span.EndTime.IsZero() {
		t.Fatal("end time should be set")
	}
}

func TestEndSpan_Error(t *testing.T) {
	_, span := StartTrace(context.Background(), "test")
	EndSpan(span, errors.New("something broke"))
	if span.Status != "error" {
		t.Fatalf("expected error status, got %s", span.Status)
	}
	if span.Error != "something broke" {
		t.Fatalf("expected error message, got %s", span.Error)
	}
}

func TestTraceIDFromContext_Empty(t *testing.T) {
	id := TraceIDFromContext(context.Background())
	if id != "" {
		t.Fatalf("expected empty trace ID from bare context, got %s", id)
	}
}

func TestSpanAttrs(t *testing.T) {
	_, span := StartTrace(context.Background(), "test")
	span.Attrs["tenant"] = "t1"
	span.Attrs["model"] = "gpt-4"
	EndSpan(span, nil)
	if span.Attrs["tenant"] != "t1" || span.Attrs["model"] != "gpt-4" {
		t.Fatal("attrs should be preserved")
	}
}
