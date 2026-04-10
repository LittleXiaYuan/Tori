package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// EventType classifies audit events.
type EventType string

const (
	EventChat       EventType = "chat"
	EventToolCall   EventType = "tool_call"
	EventMemory     EventType = "memory"
	EventConfig     EventType = "config"
	EventAuth       EventType = "auth"
	EventSkill      EventType = "skill"
	EventAgent      EventType = "agent"
	EventSystem     EventType = "system"
)

// Record is a single entry in the Merkle audit chain.
// Each record's Hash includes the previous record's hash, forming a tamper-evident chain.
type Record struct {
	Seq       uint64    `json:"seq"`
	Timestamp time.Time `json:"timestamp"`
	Type      EventType `json:"type"`
	Actor     string    `json:"actor"`     // who triggered (tenant_id, user_id, agent_id)
	Action    string    `json:"action"`    // what happened
	Detail    string    `json:"detail"`    // extra context (truncated)
	PrevHash  string    `json:"prev_hash"` // hash of previous record
	Hash      string    `json:"hash"`      // SHA256(seq|timestamp|type|actor|action|detail|prev_hash)
}

// computeHash builds the SHA256 hash for a record given its fields and prev hash.
func computeHash(seq uint64, ts time.Time, typ EventType, actor, action, detail, prevHash string) string {
	payload := fmt.Sprintf("%d|%s|%s|%s|%s|%s|%s",
		seq, ts.UTC().Format(time.RFC3339Nano), typ, actor, action, detail, prevHash)
	h := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(h[:])
}

// Chain is a tamper-evident Merkle audit log.
// Records are chained via SHA256 hashes; any modification to a past record
// invalidates all subsequent hashes, making tampering detectable.
type Chain struct {
	mu       sync.RWMutex
	records  []Record
	lastHash string
	seq      uint64
	maxSize  int // max records in memory (oldest evicted to JSONL)
	filePath string
	file     *os.File
}

// ChainConfig configures the audit chain.
type ChainConfig struct {
	FilePath string // path for JSONL persistence (empty = memory only)
	MaxSize  int    // max records kept in memory (default 10000)
}

// NewChain creates a new Merkle audit chain.
func NewChain(cfg ChainConfig) (*Chain, error) {
	if cfg.MaxSize <= 0 {
		cfg.MaxSize = 10000
	}
	c := &Chain{
		records:  make([]Record, 0, 256),
		maxSize:  cfg.MaxSize,
		filePath: cfg.FilePath,
	}

	// Open JSONL file for append if configured
	if cfg.FilePath != "" {
		f, err := os.OpenFile(cfg.FilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("audit chain: open file: %w", err)
		}
		c.file = f
	}

	return c, nil
}

// Append adds a new record to the chain and returns it.
func (c *Chain) Append(typ EventType, actor, action, detail string) Record {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Truncate detail to prevent huge entries
	if len(detail) > 4096 {
		detail = detail[:4096] + "...[truncated]"
	}

	c.seq++
	now := time.Now()
	hash := computeHash(c.seq, now, typ, actor, action, detail, c.lastHash)

	rec := Record{
		Seq:       c.seq,
		Timestamp: now,
		Type:      typ,
		Actor:     actor,
		Action:    action,
		Detail:    detail,
		PrevHash:  c.lastHash,
		Hash:      hash,
	}

	c.records = append(c.records, rec)
	c.lastHash = hash

	// Persist to JSONL
	if c.file != nil {
		if data, err := json.Marshal(rec); err == nil {
			c.file.Write(append(data, '\n'))
		}
	}

	// Evict oldest if over max size
	if len(c.records) > c.maxSize {
		c.records = c.records[len(c.records)-c.maxSize:]
	}

	return rec
}

// Verify checks the integrity of the in-memory chain.
// Returns the index of the first invalid record, or -1 if chain is valid.
func (c *Chain) Verify() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for i, rec := range c.records {
		var expectedPrev string
		if i > 0 {
			expectedPrev = c.records[i-1].Hash
		}
		// For the first in-memory record, we trust its PrevHash
		// (it may reference an evicted record)
		if i > 0 && rec.PrevHash != expectedPrev {
			return i
		}
		expected := computeHash(rec.Seq, rec.Timestamp, rec.Type, rec.Actor, rec.Action, rec.Detail, rec.PrevHash)
		if rec.Hash != expected {
			return i
		}
	}
	return -1
}

// Last returns the most recent record, or nil if empty.
func (c *Chain) Last() *Record {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.records) == 0 {
		return nil
	}
	r := c.records[len(c.records)-1]
	return &r
}

// Len returns the number of records in memory.
func (c *Chain) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.records)
}

// Tail returns the last n records (most recent first).
func (c *Chain) Tail(n int) []Record {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if n <= 0 || len(c.records) == 0 {
		return nil
	}
	if n > len(c.records) {
		n = len(c.records)
	}
	out := make([]Record, n)
	for i := 0; i < n; i++ {
		out[i] = c.records[len(c.records)-1-i]
	}
	return out
}

// Search returns records matching the given type and actor (empty = any).
func (c *Chain) Search(typ EventType, actor string, limit int) []Record {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if limit <= 0 {
		limit = 100
	}
	var out []Record
	// Search from newest to oldest
	for i := len(c.records) - 1; i >= 0 && len(out) < limit; i-- {
		rec := c.records[i]
		if typ != "" && rec.Type != typ {
			continue
		}
		if actor != "" && rec.Actor != actor {
			continue
		}
		out = append(out, rec)
	}
	return out
}

// Stats returns chain statistics.
func (c *Chain) Stats() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()

	typeCounts := make(map[EventType]int)
	actors := make(map[string]int)
	for _, rec := range c.records {
		typeCounts[rec.Type]++
		if rec.Actor != "" {
			actors[rec.Actor]++
		}
	}

	stats := map[string]any{
		"total":        c.seq,
		"total_seq":    c.seq,
		"in_memory":    len(c.records),
		"max_size":     c.maxSize,
		"last_hash":    c.lastHash,
		"type_counts":  typeCounts,
		"actors":       actors,
		"has_file":     c.file != nil,
	}

	if len(c.records) > 0 {
		stats["first_at"] = c.records[0].Timestamp
		stats["last_at"] = c.records[len(c.records)-1].Timestamp
		stats["oldest"] = c.records[0].Timestamp
		stats["newest"] = c.records[len(c.records)-1].Timestamp
	}

	return stats
}

// Close flushes and closes the JSONL file if open.
func (c *Chain) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.file != nil {
		return c.file.Close()
	}
	return nil
}

// LoadFromFile reads existing JSONL records and rebuilds the chain.
// This should be called once at startup before Append.
func (c *Chain) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	lines := splitLines(data)
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var rec Record
		if err := json.Unmarshal(line, &rec); err != nil {
			continue // skip corrupted lines
		}
		c.records = append(c.records, rec)
		c.lastHash = rec.Hash
		if rec.Seq > c.seq {
			c.seq = rec.Seq
		}
	}

	// Trim to max size
	if len(c.records) > c.maxSize {
		c.records = c.records[len(c.records)-c.maxSize:]
	}

	return nil
}

func splitLines(data []byte) [][]byte {
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
