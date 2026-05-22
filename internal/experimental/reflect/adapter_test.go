package reflect

import (
	"context"
	"testing"
)

func TestEngineAsReflectEvalFuncAdaptsDisabledEngine(t *testing.T) {
	engine := NewEngine(nil)
	engine.SetEnabled(false)

	evalFn := engine.AsReflectEvalFunc()
	got, err := evalFn(context.Background(), "用户意图", "回复", []string{"skill-a"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil adapted result")
	}
	if !got.Satisfied || got.Quality != 8 {
		t.Fatalf("adapted result = satisfied:%v quality:%d, want satisfied:true quality:8", got.Satisfied, got.Quality)
	}
}

func TestToKernelReflectEvalResultCopiesFields(t *testing.T) {
	got := toKernelReflectEvalResult(&Evaluation{
		Satisfied:   false,
		Quality:     4,
		Issues:      []string{"issue"},
		Suggestions: []string{"suggestion"},
		MemoryUpdates: []MemoryUpdate{
			{Action: "add", Key: "k", Value: "v"},
		},
	})

	if got == nil {
		t.Fatal("expected non-nil result")
	}
	if got.Satisfied || got.Quality != 4 {
		t.Fatalf("adapted result = satisfied:%v quality:%d, want satisfied:false quality:4", got.Satisfied, got.Quality)
	}
	if len(got.Issues) != 1 || got.Issues[0] != "issue" {
		t.Fatalf("issues not copied: %#v", got.Issues)
	}
	if len(got.Suggestions) != 1 || got.Suggestions[0] != "suggestion" {
		t.Fatalf("suggestions not copied: %#v", got.Suggestions)
	}
	if len(got.MemoryUpdates) != 1 {
		t.Fatalf("memory updates not copied: %#v", got.MemoryUpdates)
	}
	if got.MemoryUpdates[0].Action != "add" || got.MemoryUpdates[0].Key != "k" || got.MemoryUpdates[0].Value != "v" {
		t.Fatalf("unexpected memory update: %#v", got.MemoryUpdates[0])
	}
}
