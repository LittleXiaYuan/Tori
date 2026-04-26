package localbrain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultFilterConfig(t *testing.T) {
	cfg := DefaultFilterConfig()
	if cfg.MinInputLen != 5 {
		t.Errorf("MinInputLen = %d", cfg.MinInputLen)
	}
	if cfg.MinOutputLen != 10 {
		t.Errorf("MinOutputLen = %d", cfg.MinOutputLen)
	}
	if !cfg.RemoveEmptyJSON {
		t.Error("RemoveEmptyJSON should be true by default")
	}
}

func TestFilterFile_Deduplication(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "train.jsonl")

	lines := []string{
		`{"instruction":"hello","input":"world","output":"greeting response here"}`,
		`{"instruction":"hello","input":"world","output":"greeting response here"}`,
		`{"instruction":"different","input":"query","output":"different response here"}`,
	}
	os.WriteFile(input, []byte(strings.Join(lines, "\n")+"\n"), 0644)

	f := NewTrainingFilter(DefaultFilterConfig())
	outPath, stats, err := f.FilterFile(input)
	if err != nil {
		t.Fatalf("FilterFile failed: %v", err)
	}

	if stats.TotalRead != 3 {
		t.Errorf("TotalRead = %d, want 3", stats.TotalRead)
	}
	if stats.Kept != 2 {
		t.Errorf("Kept = %d, want 2", stats.Kept)
	}
	if stats.DroppedDup != 1 {
		t.Errorf("DroppedDup = %d, want 1", stats.DroppedDup)
	}

	data, _ := os.ReadFile(outPath)
	outputLines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(outputLines) != 2 {
		t.Errorf("output has %d lines, want 2", len(outputLines))
	}
}

func TestFilterFile_EmptyJSON(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "train.jsonl")

	lines := []string{
		`{}`,
		`{ }`,
		`{"instruction":"valid","input":"test input","output":"valid output here"}`,
	}
	os.WriteFile(input, []byte(strings.Join(lines, "\n")+"\n"), 0644)

	f := NewTrainingFilter(DefaultFilterConfig())
	_, stats, err := f.FilterFile(input)
	if err != nil {
		t.Fatalf("FilterFile failed: %v", err)
	}

	if stats.DroppedEmpty != 2 {
		t.Errorf("DroppedEmpty = %d, want 2", stats.DroppedEmpty)
	}
	if stats.Kept != 1 {
		t.Errorf("Kept = %d, want 1", stats.Kept)
	}
}

func TestFilterFile_TooShort(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "train.jsonl")

	lines := []string{
		`{"instruction":"hi","input":"","output":"ok"}`,
		`{"instruction":"valid instruction text","input":"valid input","output":"valid output text"}`,
	}
	os.WriteFile(input, []byte(strings.Join(lines, "\n")+"\n"), 0644)

	f := NewTrainingFilter(DefaultFilterConfig())
	_, stats, err := f.FilterFile(input)
	if err != nil {
		t.Fatalf("FilterFile failed: %v", err)
	}

	if stats.DroppedTooShort != 1 {
		t.Errorf("DroppedTooShort = %d, want 1", stats.DroppedTooShort)
	}
}

func TestFilterFile_TooLong(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "train.jsonl")

	cfg := DefaultFilterConfig()
	cfg.MaxOutputLen = 20

	longOutput := strings.Repeat("a", 30)
	lines := []string{
		`{"instruction":"test instruction","input":"test","output":"` + longOutput + `"}`,
		`{"instruction":"test instruction","input":"test","output":"short output ok"}`,
	}
	os.WriteFile(input, []byte(strings.Join(lines, "\n")+"\n"), 0644)

	f := NewTrainingFilter(cfg)
	_, stats, err := f.FilterFile(input)
	if err != nil {
		t.Fatalf("FilterFile failed: %v", err)
	}

	if stats.DroppedTooLong != 1 {
		t.Errorf("DroppedTooLong = %d, want 1", stats.DroppedTooLong)
	}
}

func TestFilterFile_LowRewardTrajectory(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "train.jsonl")

	cfg := DefaultFilterConfig()
	cfg.MinReward = 0.3

	traj := map[string]interface{}{
		"task_id":      "t1",
		"trajectory":   []interface{}{map[string]interface{}{"step_type": "decide"}},
		"reward":       0.1,
		"task_success": false,
	}
	good := map[string]interface{}{
		"task_id":      "t2",
		"trajectory":   []interface{}{map[string]interface{}{"step_type": "decide"}},
		"reward":       0.8,
		"task_success": true,
	}
	line1, _ := json.Marshal(traj)
	line2, _ := json.Marshal(good)
	os.WriteFile(input, []byte(string(line1)+"\n"+string(line2)+"\n"), 0644)

	f := NewTrainingFilter(cfg)
	_, stats, err := f.FilterFile(input)
	if err != nil {
		t.Fatalf("FilterFile failed: %v", err)
	}

	if stats.DroppedLowScore != 1 {
		t.Errorf("DroppedLowScore = %d, want 1", stats.DroppedLowScore)
	}
	if stats.Kept != 1 {
		t.Errorf("Kept = %d, want 1", stats.Kept)
	}
}

func TestFilterFile_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "train.jsonl")

	lines := []string{
		`not json at all`,
		`{"instruction":"valid","input":"test input","output":"valid output here"}`,
		`{broken: json`,
	}
	os.WriteFile(input, []byte(strings.Join(lines, "\n")+"\n"), 0644)

	f := NewTrainingFilter(DefaultFilterConfig())
	_, stats, err := f.FilterFile(input)
	if err != nil {
		t.Fatalf("FilterFile failed: %v", err)
	}

	if stats.DroppedMalformed != 2 {
		t.Errorf("DroppedMalformed = %d, want 2", stats.DroppedMalformed)
	}
}

func TestFilterFile_Garbage(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "train.jsonl")

	garbage := strings.Repeat("a", 50)
	lines := []string{
		`{"instruction":"test instruction","input":"test","output":"` + garbage + `"}`,
		`{"instruction":"test instruction","input":"test","output":"normal response text"}`,
	}
	os.WriteFile(input, []byte(strings.Join(lines, "\n")+"\n"), 0644)

	f := NewTrainingFilter(DefaultFilterConfig())
	_, stats, err := f.FilterFile(input)
	if err != nil {
		t.Fatalf("FilterFile failed: %v", err)
	}

	if stats.DroppedGarbage != 1 {
		t.Errorf("DroppedGarbage = %d, want 1", stats.DroppedGarbage)
	}
}

func TestFilterFile_EmptyInput(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "train.jsonl")
	os.WriteFile(input, []byte(""), 0644)

	f := NewTrainingFilter(DefaultFilterConfig())
	_, stats, err := f.FilterFile(input)
	if err != nil {
		t.Fatalf("FilterFile failed: %v", err)
	}

	if stats.TotalRead != 0 {
		t.Errorf("TotalRead = %d, want 0", stats.TotalRead)
	}
	if stats.Kept != 0 {
		t.Errorf("Kept = %d, want 0", stats.Kept)
	}
}

func TestIsGarbage(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect bool
	}{
		{"empty", "", true},
		{"normal", "This is a normal response", false},
		{"repeated chars", strings.Repeat("x", 50), true},
		{"short repeated", "aaa", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isGarbage(tt.input); got != tt.expect {
				t.Errorf("isGarbage(%q) = %v, want %v", tt.input, got, tt.expect)
			}
		})
	}
}

func TestContentHash(t *testing.T) {
	h1 := contentHash("hello")
	h2 := contentHash("hello")
	h3 := contentHash("world")

	if h1 != h2 {
		t.Error("same content should produce same hash")
	}
	if h1 == h3 {
		t.Error("different content should produce different hash")
	}
}
