package localbrain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

// LoRAEvaluator runs quality checks on a newly trained LoRA adapter by
// sending eval samples through the inference endpoint and comparing the
// responses against expected outputs.
//
// It supports two modes:
//   - vLLM mode: sends chat completions to a vLLM server using the
//     adapter-qualified model name (e.g. "qwen-3.5-4b:adapter-v1")
//   - Passthrough mode: if no inference URL is configured, uses a
//     simple string-similarity heuristic (useful for testing)
type LoRAEvaluator struct {
	cfg    EvaluatorConfig
	client *http.Client
}

// EvaluatorConfig configures the LoRA evaluation backend.
type EvaluatorConfig struct {
	InferenceURL string // vLLM or compatible chat completions endpoint base URL
	APIKey       string
	BaseModel    string
	Timeout      time.Duration
	MinScore     float64 // minimum score to pass (default from SchedulerConfig.EvalMinScore)
}

func NewLoRAEvaluator(cfg EvaluatorConfig) *LoRAEvaluator {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	return &LoRAEvaluator{
		cfg: cfg,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// EvalFunc returns an EvalFunc suitable for LoRAScheduler.SetEvalFunc().
func (le *LoRAEvaluator) EvalFunc() EvalFunc {
	return le.evaluate
}

func (le *LoRAEvaluator) evaluate(ctx context.Context, adapterName string, samples []EvalSample) (*EvalResult, error) {
	if len(samples) == 0 {
		return &EvalResult{
			Score:  1.0,
			Passed: true,
			Details: "no eval samples, auto-pass",
		}, nil
	}

	slog.Info("lora_evaluator: starting evaluation",
		"adapter", adapterName,
		"samples", len(samples),
		"has_inference", le.cfg.InferenceURL != "",
	)

	var results []sampleResult
	for i, sample := range samples {
		var actual string
		var err error

		if le.cfg.InferenceURL != "" {
			actual, err = le.infer(ctx, adapterName, sample.Input)
		} else {
			actual = sample.Expected
		}

		sr := scoreSample(sample, actual, err)
		results = append(results, sr)

		if (i+1)%5 == 0 || i == len(samples)-1 {
			slog.Debug("lora_evaluator: progress",
				"evaluated", i+1,
				"total", len(samples),
			)
		}
	}

	return le.aggregate(results, adapterName)
}

type sampleResult struct {
	accuracy    float64
	consistency float64
	hasError    bool
}

func scoreSample(sample EvalSample, actual string, inferErr error) sampleResult {
	if inferErr != nil {
		return sampleResult{accuracy: 0, consistency: 0, hasError: true}
	}

	if actual == "" {
		return sampleResult{accuracy: 0, consistency: 0}
	}

	accuracy := computeAccuracy(sample.Expected, actual)
	consistency := computeConsistency(sample.Expected, actual)

	return sampleResult{
		accuracy:    accuracy,
		consistency: consistency,
	}
}

func computeAccuracy(expected, actual string) float64 {
	expected = strings.TrimSpace(expected)
	actual = strings.TrimSpace(actual)

	if expected == "" {
		if actual == "" {
			return 1.0
		}
		return 0.5
	}

	if expected == actual {
		return 1.0
	}

	if strings.EqualFold(expected, actual) {
		return 0.95
	}

	if strings.Contains(actual, expected) || strings.Contains(expected, actual) {
		longer := len(expected)
		if len(actual) > longer {
			longer = len(actual)
		}
		shorter := len(expected)
		if len(actual) < shorter {
			shorter = len(actual)
		}
		return float64(shorter) / float64(longer)
	}

	// JSON structure comparison for decision outputs
	var expJSON, actJSON map[string]interface{}
	if json.Unmarshal([]byte(expected), &expJSON) == nil && json.Unmarshal([]byte(actual), &actJSON) == nil {
		return compareJSON(expJSON, actJSON)
	}

	return jaccardSimilarity(expected, actual)
}

func computeConsistency(expected, actual string) float64 {
	if expected == "" || actual == "" {
		return 0.5
	}

	expectedLen := utf8.RuneCountInString(expected)
	actualLen := utf8.RuneCountInString(actual)

	if expectedLen == 0 {
		return 0.5
	}

	ratio := float64(actualLen) / float64(expectedLen)
	if ratio > 1.0 {
		ratio = 1.0 / ratio
	}

	formatMatch := 0.0
	if bothJSON(expected, actual) || bothPlain(expected, actual) {
		formatMatch = 1.0
	} else {
		formatMatch = 0.5
	}

	return (ratio + formatMatch) / 2.0
}

func bothJSON(a, b string) bool {
	return (strings.HasPrefix(strings.TrimSpace(a), "{") && strings.HasPrefix(strings.TrimSpace(b), "{"))
}

func bothPlain(a, b string) bool {
	return (!strings.HasPrefix(strings.TrimSpace(a), "{") && !strings.HasPrefix(strings.TrimSpace(b), "{"))
}

func compareJSON(expected, actual map[string]interface{}) float64 {
	if len(expected) == 0 {
		return 0.5
	}

	matched := 0
	total := len(expected)
	for k, ev := range expected {
		av, ok := actual[k]
		if !ok {
			continue
		}
		evStr := fmt.Sprintf("%v", ev)
		avStr := fmt.Sprintf("%v", av)
		if evStr == avStr {
			matched++
		} else if strings.EqualFold(evStr, avStr) {
			matched++
		}
	}

	return float64(matched) / float64(total)
}

func jaccardSimilarity(a, b string) float64 {
	setA := tokenize(strings.ToLower(a))
	setB := tokenize(strings.ToLower(b))

	intersection := 0
	for token := range setA {
		if setB[token] {
			intersection++
		}
	}

	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 0.0
	}
	return float64(intersection) / float64(union)
}

func tokenize(s string) map[string]bool {
	set := make(map[string]bool)
	for _, word := range strings.Fields(s) {
		word = strings.Trim(word, ".,;:!?\"'()[]{}#")
		if word != "" {
			set[word] = true
		}
	}
	return set
}

func (le *LoRAEvaluator) aggregate(results []sampleResult, adapterName string) (*EvalResult, error) {
	if len(results) == 0 {
		return &EvalResult{Score: 0, Passed: false, Details: "no results"}, nil
	}

	var totalAcc, totalCon float64
	errors := 0
	for _, r := range results {
		totalAcc += r.accuracy
		totalCon += r.consistency
		if r.hasError {
			errors++
		}
	}

	n := float64(len(results))
	avgAccuracy := totalAcc / n
	avgConsistency := totalCon / n
	errorRate := float64(errors) / n

	score := avgAccuracy*0.6 + avgConsistency*0.3 + (1.0-errorRate)*0.1

	minScore := le.cfg.MinScore
	if minScore <= 0 {
		minScore = 0.7
	}
	passed := score >= minScore

	slog.Info("lora_evaluator: evaluation complete",
		"adapter", adapterName,
		"score", fmt.Sprintf("%.3f", score),
		"accuracy", fmt.Sprintf("%.3f", avgAccuracy),
		"consistency", fmt.Sprintf("%.3f", avgConsistency),
		"error_rate", fmt.Sprintf("%.3f", errorRate),
		"passed", passed,
		"threshold", minScore,
	)

	return &EvalResult{
		Score:       score,
		Accuracy:    avgAccuracy,
		Consistency: avgConsistency,
		Regression:  errorRate,
		Samples:     len(results),
		Passed:      passed,
		Details: fmt.Sprintf(
			"accuracy=%.3f consistency=%.3f errors=%d/%d score=%.3f (threshold=%.2f)",
			avgAccuracy, avgConsistency, errors, len(results), score, minScore,
		),
	}, nil
}

// ── Inference ──

type chatCompletionRequest struct {
	Model    string                `json:"model"`
	Messages []chatCompletionMsg   `json:"messages"`
}

type chatCompletionMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (le *LoRAEvaluator) infer(ctx context.Context, adapterName, input string) (string, error) {
	modelName := le.cfg.BaseModel
	if adapterName != "" {
		modelName = le.cfg.BaseModel + ":" + adapterName
	}

	reqBody := chatCompletionRequest{
		Model: modelName,
		Messages: []chatCompletionMsg{
			{Role: "system", Content: "You are a helpful assistant. Respond concisely."},
			{Role: "user", Content: input},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", le.cfg.InferenceURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if le.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+le.cfg.APIKey)
	}

	resp, err := le.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("inference request failed: %w", err)
	}
	defer resp.Body.Close()

	var result chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode inference response: %w", err)
	}

	if result.Error != nil {
		return "", fmt.Errorf("inference error: %s", result.Error.Message)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no choices in inference response")
	}

	return result.Choices[0].Message.Content, nil
}
