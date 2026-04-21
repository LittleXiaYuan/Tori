package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// TrailEntry is a structured audit record for a single operation.
type TrailEntry struct {
	Timestamp    time.Time `json:"timestamp"`
	Operation    string    `json:"operation"`
	InputSummary string    `json:"input_summary"`
	Result       string    `json:"result"`
	ModelUsed    string    `json:"model_used,omitempty"`
	Tokens       int       `json:"tokens,omitempty"`
	RiskLevel    string    `json:"risk_level"`
	ReviewResult string    `json:"review_result,omitempty"`
	Actor        string    `json:"actor,omitempty"` // tenant or agent ID
}

// Trail manages daily append-only audit files.
type Trail struct {
	mu     sync.Mutex
	dir    string // base dir (data/audit/)
	buffer []TrailEntry
}

// NewTrail creates a task audit trail writer.
func NewTrail(dir string) *Trail {
	os.MkdirAll(dir, 0755)
	return &Trail{dir: dir}
}

// Record appends an entry to today's audit file.
func (t *Trail) Record(entry TrailEntry) {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	t.buffer = append(t.buffer, entry)

	// Cap in-memory buffer to prevent unbounded growth
	const maxBuffer = 5000
	if len(t.buffer) > maxBuffer {
		t.buffer = t.buffer[len(t.buffer)-maxBuffer:]
	}

	// Write immediately to daily file (append-only)
	path := t.dailyPath(entry.Timestamp)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	data, _ := json.Marshal(entry)
	f.Write(append(data, '\n'))
}

// Query returns entries for a given date and optional operation type filter.
func (t *Trail) Query(date time.Time, opFilter string) []TrailEntry {
	path := t.dailyPath(date)
	data, err := os.ReadFile(path)
	if err != nil {
		// Backwards compatibility: trails written before the .jsonl rename
		// still live under the .json extension.
		legacy := filepath.Join(t.dir, date.Format("2006-01-02")+".json")
		data, err = os.ReadFile(legacy)
		if err != nil {
			return nil
		}
	}

	var entries []TrailEntry
	for _, line := range trailSplitLines(data) {
		if len(line) == 0 {
			continue
		}
		var e TrailEntry
		if json.Unmarshal(line, &e) == nil {
			if opFilter == "" || e.Operation == opFilter {
				entries = append(entries, e)
			}
		}
	}
	return entries
}

// Recent returns the last N entries from the in-memory buffer.
func (t *Trail) Recent(n int) []TrailEntry {
	t.mu.Lock()
	defer t.mu.Unlock()
	if n <= 0 || len(t.buffer) == 0 {
		return nil
	}
	if n > len(t.buffer) {
		n = len(t.buffer)
	}
	out := make([]TrailEntry, n)
	for i := 0; i < n; i++ {
		out[i] = t.buffer[len(t.buffer)-1-i]
	}
	return out
}

// dailyPath returns the path for the day's audit file. The on-disk format is
// newline-delimited JSON, so the .jsonl extension is used to be self-describing.
// Query falls back to the legacy .json path for files written before the
// extension change.
func (t *Trail) dailyPath(date time.Time) string {
	return filepath.Join(t.dir, date.Format("2006-01-02")+".jsonl")
}

func trailSplitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			lines = append(lines, data[start:i])
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}
