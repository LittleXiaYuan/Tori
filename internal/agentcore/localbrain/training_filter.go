package localbrain

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// TrainingFilter processes JSONL training data to improve quality before
// LoRA fine-tuning. It performs deduplication, length validation, content
// quality checks, and anomaly detection.
type TrainingFilter struct {
	cfg FilterConfig
}

// FilterConfig controls training data quality thresholds.
type FilterConfig struct {
	MinInputLen     int     // minimum input character count (default 5)
	MaxInputLen     int     // maximum input character count (default 4096)
	MinOutputLen    int     // minimum output character count (default 10)
	MaxOutputLen    int     // maximum output character count (default 8192)
	MinReward       float64 // minimum reward score to keep (default 0.0)
	MaxDupRatio     float64 // max ratio of kept duplicates (0 = strict dedup, default 0)
	RemoveEmptyJSON bool    // remove records that are just "{}" (default true)
}

// DefaultFilterConfig returns production-ready filter settings.
func DefaultFilterConfig() FilterConfig {
	return FilterConfig{
		MinInputLen:     5,
		MaxInputLen:     4096,
		MinOutputLen:    10,
		MaxOutputLen:    8192,
		MinReward:       0.0,
		MaxDupRatio:     0,
		RemoveEmptyJSON: true,
	}
}

func NewTrainingFilter(cfg FilterConfig) *TrainingFilter {
	return &TrainingFilter{cfg: cfg}
}

// FilterStats reports what the filter did to the data.
type FilterStats struct {
	TotalRead        int `json:"total_read"`
	Kept             int `json:"kept"`
	DroppedEmpty     int `json:"dropped_empty"`
	DroppedTooShort  int `json:"dropped_too_short"`
	DroppedTooLong   int `json:"dropped_too_long"`
	DroppedDup       int `json:"dropped_duplicate"`
	DroppedLowScore  int `json:"dropped_low_score"`
	DroppedMalformed int `json:"dropped_malformed"`
	DroppedGarbage   int `json:"dropped_garbage"`
}

// FilterFile reads a JSONL file, applies quality filters, and writes a
// filtered version. Returns the path to the filtered file and stats.
func (tf *TrainingFilter) FilterFile(inputPath string) (string, *FilterStats, error) {
	return tf.processFile(inputPath, true)
}

// PreviewFile reads a JSONL file and returns filter stats without writing a
// filtered output file. Use this for training readiness checks before starting
// an expensive LoRA job.
func (tf *TrainingFilter) PreviewFile(inputPath string) (*FilterStats, error) {
	return tf.PreviewFiles([]string{inputPath})
}

// PreviewFiles reads one or more JSONL files and returns filter stats without
// writing filtered output files. Deduplication is applied across all files.
func (tf *TrainingFilter) PreviewFiles(inputPaths []string) (*FilterStats, error) {
	stats := &FilterStats{}
	seen := make(map[string]struct{})
	for _, inputPath := range inputPaths {
		if err := tf.processFileInto(inputPath, nil, stats, seen); err != nil {
			return stats, err
		}
	}
	return stats, nil
}

func (tf *TrainingFilter) processFile(inputPath string, writeOutput bool) (string, *FilterStats, error) {
	in, err := os.Open(inputPath)
	if err != nil {
		return "", nil, fmt.Errorf("open input: %w", err)
	}
	defer in.Close()

	outPath := ""
	var enc *json.Encoder
	if writeOutput {
		dir := filepath.Dir(inputPath)
		out, err := createTrainingJSONLOutput(dir, "filtered")
		if err != nil {
			return "", nil, fmt.Errorf("create output: %w", err)
		}
		outPath = out.Name()
		defer out.Close()
		enc = json.NewEncoder(out)
	}

	stats := &FilterStats{}
	seen := make(map[string]struct{})
	if err := tf.processOpenFileInto(in, enc, stats, seen); err != nil {
		return outPath, stats, err
	}

	slog.Info("training_filter: complete",
		"input", inputPath,
		"output", outPath,
		"write_output", writeOutput,
		"total", stats.TotalRead,
		"kept", stats.Kept,
		"dropped_dup", stats.DroppedDup,
		"dropped_short", stats.DroppedTooShort,
		"dropped_long", stats.DroppedTooLong,
		"dropped_score", stats.DroppedLowScore,
		"dropped_garbage", stats.DroppedGarbage,
		"dropped_malformed", stats.DroppedMalformed,
	)

	return outPath, stats, nil
}

func (tf *TrainingFilter) processFileInto(inputPath string, enc *json.Encoder, stats *FilterStats, seen map[string]struct{}) error {
	in, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("open input: %w", err)
	}
	defer in.Close()

	if err := tf.processOpenFileInto(in, enc, stats, seen); err != nil {
		return fmt.Errorf("%s: %w", inputPath, err)
	}
	return nil
}

func (tf *TrainingFilter) processOpenFileInto(in *os.File, enc *json.Encoder, stats *FilterStats, seen map[string]struct{}) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		stats.TotalRead++

		if line == "" {
			stats.DroppedEmpty++
			continue
		}

		if tf.cfg.RemoveEmptyJSON && (line == "{}" || line == "{ }") {
			stats.DroppedEmpty++
			continue
		}

		var record map[string]interface{}
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			stats.DroppedMalformed++
			continue
		}

		reason := tf.shouldDrop(record)
		switch reason {
		case "":
			// pass
		case "too_short":
			stats.DroppedTooShort++
			continue
		case "too_long":
			stats.DroppedTooLong++
			continue
		case "low_score":
			stats.DroppedLowScore++
			continue
		case "garbage":
			stats.DroppedGarbage++
			continue
		default:
			stats.DroppedMalformed++
			continue
		}

		hash := contentHash(line)
		if _, dup := seen[hash]; dup {
			stats.DroppedDup++
			continue
		}
		seen[hash] = struct{}{}

		if enc != nil {
			if err := enc.Encode(record); err != nil {
				continue
			}
		}
		stats.Kept++
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan error: %w", err)
	}
	return nil
}

func (tf *TrainingFilter) shouldDrop(record map[string]interface{}) string {
	// SFT format: instruction + input + output
	if instruction, ok := record["instruction"]; ok {
		input := toString(record["input"])
		output := toString(record["output"])
		instStr := toString(instruction)

		combined := instStr + " " + input
		if utf8.RuneCountInString(strings.TrimSpace(combined)) < tf.cfg.MinInputLen {
			return "too_short"
		}
		if utf8.RuneCountInString(combined) > tf.cfg.MaxInputLen {
			return "too_long"
		}
		if utf8.RuneCountInString(strings.TrimSpace(output)) < tf.cfg.MinOutputLen {
			return "too_short"
		}
		if utf8.RuneCountInString(output) > tf.cfg.MaxOutputLen {
			return "too_long"
		}
		if isGarbage(output) {
			return "garbage"
		}
	}

	// Trajectory format: task_id + trajectory + reward
	if _, ok := record["trajectory"]; ok {
		if reward, ok := record["reward"]; ok {
			if r, ok := reward.(float64); ok && r < tf.cfg.MinReward {
				return "low_score"
			}
		}

		if success, ok := record["task_success"]; ok {
			if s, ok := success.(bool); ok && !s {
				if reward, ok := record["reward"]; ok {
					if r, ok := reward.(float64); ok && r <= 0 {
						return "low_score"
					}
				}
			}
		}

		traj, ok := record["trajectory"].([]interface{})
		if ok && len(traj) == 0 {
			return "too_short"
		}
	}

	return ""
}

func contentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:16])
}

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func isGarbage(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}

	runeCount := utf8.RuneCountInString(s)
	if runeCount == 0 {
		return true
	}

	// High ratio of non-printable or control characters
	nonPrintable := 0
	for _, r := range s {
		if r < 32 && r != '\n' && r != '\t' {
			nonPrintable++
		}
	}
	if float64(nonPrintable)/float64(runeCount) > 0.3 {
		return true
	}

	// Repetitive content: same char repeated many times
	if runeCount > 20 {
		runes := []rune(s)
		maxRepeat := 1
		currentRepeat := 1
		for i := 1; i < len(runes); i++ {
			if runes[i] == runes[i-1] {
				currentRepeat++
				if currentRepeat > maxRepeat {
					maxRepeat = currentRepeat
				}
			} else {
				currentRepeat = 1
			}
		}
		if float64(maxRepeat)/float64(runeCount) > 0.5 {
			return true
		}
	}

	return false
}
