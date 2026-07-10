package planner

import "testing"

func TestCogniTraceDetailSummaryIncludesDroppedForBudget(t *testing.T) {
	d := CogniTraceDetail{
		Activated:        []string{"教育领域插件"},
		ContextBytes:     500,
		DroppedForBudget: []string{"low-priority-cogni"},
	}
	got := d.summary()
	want := "Cogni 已激活：教育领域插件，注入上下文 500 字节，因预算丢弃：low-priority-cogni"
	if got != want {
		t.Fatalf("summary() = %q, want %q", got, want)
	}
}

func TestCogniTraceDetailSummaryOmitsBudgetNoteWhenNothingDropped(t *testing.T) {
	d := CogniTraceDetail{
		Activated:    []string{"教育领域插件"},
		ContextBytes: 500,
	}
	got := d.summary()
	want := "Cogni 已激活：教育领域插件，注入上下文 500 字节"
	if got != want {
		t.Fatalf("summary() = %q, want %q", got, want)
	}
}
