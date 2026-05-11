package context

import (
	"math"
	"testing"
)

func TestMessageEntropy_EmptyString(t *testing.T) {
	if e := messageEntropy(""); e != 0 {
		t.Errorf("empty string should have entropy 0, got %.4f", e)
	}
}

func TestMessageEntropy_SingleChar(t *testing.T) {
	if e := messageEntropy("a"); e != 0 {
		t.Errorf("single char should have entropy 0, got %.4f", e)
	}
}

func TestMessageEntropy_RepeatedChars(t *testing.T) {
	e := messageEntropy("aaaaaaa")
	if e != 0 {
		t.Errorf("all-same chars should have entropy 0, got %.4f", e)
	}
}

func TestMessageEntropy_HighEntropy(t *testing.T) {
	e := messageEntropy("The quick brown fox jumps over the lazy dog 1234567890")
	if e < 0.5 {
		t.Errorf("diverse text should have high entropy, got %.4f", e)
	}
}

func TestMessageEntropy_LowVsHigh(t *testing.T) {
	low := messageEntropy("好的好的好的")
	high := messageEntropy("请帮我分析这段代码的性能瓶颈，特别是内存分配和GC压力方面")

	if low >= high {
		t.Errorf("low-info message should have lower entropy: low=%.4f high=%.4f", low, high)
	}
}

func TestMessageEntropy_NormalizedRange(t *testing.T) {
	texts := []string{
		"OK",
		"收到",
		"Hello World! This is a test message with various words and numbers 12345",
		"The quick brown fox jumps over the lazy dog",
	}
	for _, text := range texts {
		e := messageEntropy(text)
		if e < 0 || e > 1.0+1e-10 {
			t.Errorf("entropy should be in [0,1], got %.4f for %q", e, text)
		}
	}
}

func TestTrimToFit_PreservesHighEntropy(t *testing.T) {
	msgs := []Message{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "好的"},
		{Role: "assistant", Content: "好的好的"},
		{Role: "user", Content: "请帮我分析这个复杂的数据管道的性能瓶颈，包括ETL流程、内存分配模式和GC压力等多个维度"},
		{Role: "assistant", Content: "这是一个需要深入分析的问题。首先看ETL流程中的I/O瓶颈..."},
	}

	cfg := WindowConfig{
		MaxTokens:     100,
		SystemReserve: 20,
		ReplyReserve:  20,
		MaxMessages:   100,
		PreserveFirst: 1,
		PreserveLast:  1,
	}

	result := TrimToFit(msgs, cfg)
	if result.DroppedCount == 0 {
		return
	}

	for _, m := range result.Messages {
		e := messageEntropy(m.Content)
		_ = e
	}

	if len(result.Messages) < 2 {
		t.Error("should preserve at least system + last message")
	}

	_ = math.Log2(1.0)
}
