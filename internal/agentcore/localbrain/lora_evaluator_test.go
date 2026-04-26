package localbrain

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEvaluator_NoSamples(t *testing.T) {
	e := NewLoRAEvaluator(EvaluatorConfig{MinScore: 0.7})
	result, err := e.evaluate(context.Background(), "adapter-v1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Passed {
		t.Error("should auto-pass with no samples")
	}
	if result.Score != 1.0 {
		t.Errorf("Score = %f, want 1.0", result.Score)
	}
}

func TestEvaluator_OfflineMode_ExactMatch(t *testing.T) {
	e := NewLoRAEvaluator(EvaluatorConfig{MinScore: 0.5})

	samples := []EvalSample{
		{Input: "hello", Expected: "world"},
	}

	result, err := e.evaluate(context.Background(), "adapter-v1", samples)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Samples != 1 {
		t.Errorf("Samples = %d, want 1", result.Samples)
	}
}

func TestEvaluator_WithInference(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req chatCompletionRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.Model != "qwen-3.5-4b:test-adapter" {
			t.Errorf("model = %s, want qwen-3.5-4b:test-adapter", req.Model)
		}

		resp := chatCompletionResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{Message: struct {
					Content string `json:"content"`
				}{Content: "world"}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	e := NewLoRAEvaluator(EvaluatorConfig{
		InferenceURL: srv.URL,
		BaseModel:    "qwen-3.5-4b",
		MinScore:     0.5,
	})

	samples := []EvalSample{
		{Input: "hello", Expected: "world"},
	}

	result, err := e.evaluate(context.Background(), "test-adapter", samples)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Accuracy != 1.0 {
		t.Errorf("Accuracy = %f, want 1.0 for exact match", result.Accuracy)
	}
	if !result.Passed {
		t.Errorf("should pass with exact match, score=%f", result.Score)
	}
}

func TestEvaluator_InferenceError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	e := NewLoRAEvaluator(EvaluatorConfig{
		InferenceURL: srv.URL,
		BaseModel:    "qwen-3.5-4b",
		MinScore:     0.5,
	})

	samples := []EvalSample{
		{Input: "hello", Expected: "world"},
	}

	result, err := e.evaluate(context.Background(), "bad-adapter", samples)
	if err != nil {
		t.Fatalf("should not return error: %v", err)
	}
	if result.Regression == 0 {
		t.Error("expected non-zero error rate")
	}
}

func TestComputeAccuracy(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		actual   string
		minScore float64
	}{
		{"exact match", "hello", "hello", 1.0},
		{"case insensitive", "Hello", "hello", 0.9},
		{"substring", "hello world", "hello", 0.4},
		{"empty expected", "", "", 1.0},
		{"totally different", "abc", "xyz", 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := computeAccuracy(tt.expected, tt.actual)
			if score < tt.minScore {
				t.Errorf("computeAccuracy(%q, %q) = %f, want >= %f", tt.expected, tt.actual, score, tt.minScore)
			}
		})
	}
}

func TestComputeConsistency(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		actual   string
	}{
		{"same length", "hello", "world"},
		{"json format match", `{"a":1}`, `{"b":2}`},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := computeConsistency(tt.expected, tt.actual)
			if score < 0 || score > 1 {
				t.Errorf("consistency out of range: %f", score)
			}
		})
	}
}

func TestJaccardSimilarity(t *testing.T) {
	if s := jaccardSimilarity("the cat sat", "the cat"); s < 0.5 {
		t.Errorf("jaccard = %f, expected >= 0.5", s)
	}
	if s := jaccardSimilarity("abc", "xyz"); s > 0.01 {
		t.Errorf("jaccard = %f, expected ~0", s)
	}
}

func TestCompareJSON(t *testing.T) {
	a := map[string]interface{}{"decision": "use_tool", "reason": "need data"}
	b := map[string]interface{}{"decision": "use_tool", "reason": "different"}
	score := compareJSON(a, b)
	if score != 0.5 {
		t.Errorf("compareJSON = %f, want 0.5 (1 of 2 fields match)", score)
	}
}

func TestEvalFuncMethod(t *testing.T) {
	e := NewLoRAEvaluator(EvaluatorConfig{MinScore: 0.7})
	fn := e.EvalFunc()
	if fn == nil {
		t.Fatal("EvalFunc() returned nil")
	}
}

func TestEvaluator_WithAPIKey(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		resp := chatCompletionResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{Message: struct {
					Content string `json:"content"`
				}{Content: "ok"}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	e := NewLoRAEvaluator(EvaluatorConfig{
		InferenceURL: srv.URL,
		APIKey:       "sk-eval-123",
		BaseModel:    "m",
		MinScore:     0.1,
	})

	e.evaluate(context.Background(), "a", []EvalSample{{Input: "x", Expected: "ok"}})

	if gotAuth != "Bearer sk-eval-123" {
		t.Errorf("Authorization = %s, want Bearer sk-eval-123", gotAuth)
	}
}
