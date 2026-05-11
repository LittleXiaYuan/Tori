package reflect

import (
	"context"
	"testing"

	"yunque-agent/pkg/jsonutil"
)

// TestExtractJSONContract locks in the specific jsonutil function this
// package migrated to (ExtractObject), so that a future refactor that
// accidentally drops the "{}" fallback behaviour flips this red. The
// broader jsonutil behaviour is covered by pkg/jsonutil/extract_test.go;
// here we only verify the exact shape Engine.Evaluate relies on.
func TestExtractJSONContract(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`{"satisfied": true}`, `{"satisfied": true}`},
		{`some text {"quality": 8} more`, `{"quality": 8}`},
		{`no json here`, `{}`},
		{`{"nested": {"a": 1}}`, `{"nested": {"a": 1}}`},
	}
	for _, tt := range tests {
		got := jsonutil.ExtractObject(tt.input)
		if got != tt.want {
			t.Errorf("jsonutil.ExtractObject(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestEvaluationShouldRetry(t *testing.T) {
	e1 := &Evaluation{Satisfied: false, Quality: 3}
	if !e1.ShouldRetry() {
		t.Fatal("low quality unsatisfied should retry")
	}
	e2 := &Evaluation{Satisfied: true, Quality: 8}
	if e2.ShouldRetry() {
		t.Fatal("satisfied should not retry")
	}
	e3 := &Evaluation{Satisfied: false, Quality: 6}
	if e3.ShouldRetry() {
		t.Fatal("quality >= 5 should not retry")
	}
}

func TestTruncateStr(t *testing.T) {
	if truncateStr("hello", 10) != "hello" {
		t.Fatal("short string should not be truncated")
	}
	result := truncateStr("这是一段很长的中文文本用于测试截断功能", 5)
	if len([]rune(result)) > 8 { // 5 chars + "..."
		t.Fatalf("truncated too long: %q", result)
	}
}

func TestJoinStr(t *testing.T) {
	if joinStr(nil) != "none" {
		t.Fatal("empty should return none")
	}
	if joinStr([]string{"a", "b"}) != "a, b" {
		t.Fatal("unexpected join result")
	}
}

func TestIntToStr(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{7, "7"},
		{10, "10"},
		{123, "123"},
	}
	for _, tt := range tests {
		got := intToStr(tt.input)
		if got != tt.want {
			t.Errorf("intToStr(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestLearningLoopReflectAccessor(t *testing.T) {
	ll := NewLearningLoop(nil, nil)
	if ll.Reflect() == nil {
		t.Fatal("Reflect() should return non-nil engine")
	}
}

func TestLearningLoopHighQualityLessonTagsQuality(t *testing.T) {
	ll := NewLearningLoop(nil, nil)
	var gotOutcome string
	var gotTags []string
	ll.SetOnLesson(func(category, outcome, lesson, context string, tags []string) {
		gotOutcome = outcome
		gotTags = tags
	})

	ll.AfterInteraction(context.Background(), "请审查代码", "代码审查完成", []string{"review"}, 9)

	if gotOutcome != "success" {
		t.Fatalf("outcome = %q, want success", gotOutcome)
	}
	for _, want := range []string{"high_quality", "review", "quality:9", "outcome:success", "satisfied:true"} {
		if !hasString(gotTags, want) {
			t.Fatalf("tags %v missing %q", gotTags, want)
		}
	}
}

func hasString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
