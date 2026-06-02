package memory

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ──────────────────────────────────────────────
// MemoryBlock — an editable memory unit
// ──────────────────────────────────────────────

// Block represents an in-context, agent-editable memory block (inspired by Letta).
type Block struct {
	ID        string    `json:"id"`
	Label     string    `json:"label"`     // e.g. "persona", "human", "notes"
	Content   string    `json:"content"`
	MaxChars  int       `json:"max_chars"` // 0 = unlimited
	ReadOnly  bool      `json:"read_only"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Version   int       `json:"version"`
}

// Lines returns the block content split by newline.
func (b *Block) Lines() []string {
	if b.Content == "" {
		return nil
	}
	return strings.Split(b.Content, "\n")
}

// LineCount returns the number of lines.
func (b *Block) LineCount() int {
	if b.Content == "" {
		return 0
	}
	return len(b.Lines())
}

// ──────────────────────────────────────────────
// EditOp — edit operation types
// ──────────────────────────────────────────────

type EditOp string

const (
	OpReplace EditOp = "replace"  // replace a specific string
	OpInsert  EditOp = "insert"   // insert at line number
	OpPatch   EditOp = "patch"    // apply unified-diff style patch
	OpRethink EditOp = "rethink"  // full rewrite
	OpDelete  EditOp = "delete"   // delete lines
)

// EditRequest describes a memory edit operation.
type EditRequest struct {
	BlockLabel string `json:"block_label"`
	Op         EditOp `json:"op"`
	OldText    string `json:"old_text,omitempty"`    // for replace
	NewText    string `json:"new_text,omitempty"`    // for replace, insert, rethink
	LineNumber int    `json:"line_number,omitempty"` // for insert, delete (1-indexed)
	LineCount  int    `json:"line_count,omitempty"`  // for delete: how many lines
}

// EditResult captures the outcome of an edit.
type EditResult struct {
	BlockLabel string `json:"block_label"`
	Op         EditOp `json:"op"`
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`
	OldVersion int    `json:"old_version"`
	NewVersion int    `json:"new_version"`
	CharsBefore int   `json:"chars_before"`
	CharsAfter  int   `json:"chars_after"`
}

// ──────────────────────────────────────────────
// EditableMemory — manages editable blocks
// ──────────────────────────────────────────────

// EditableMemory manages a set of named, agent-editable memory blocks.
type EditableMemory struct {
	mu     sync.RWMutex
	blocks map[string]*Block
	history []EditResult
	dirty   bool // set on mutation; consumed by interval persistence
}

// NewEditableMemory creates an editable memory store.
func NewEditableMemory() *EditableMemory {
	return &EditableMemory{
		blocks: make(map[string]*Block),
	}
}

// Dirty reports whether editable memory has unsaved mutations.
func (em *EditableMemory) Dirty() bool {
	em.mu.RLock()
	defer em.mu.RUnlock()
	return em.dirty
}

// ClearDirty resets the dirty flag (call before persisting a snapshot).
func (em *EditableMemory) ClearDirty() {
	em.mu.Lock()
	defer em.mu.Unlock()
	em.dirty = false
}

// AddBlock creates a new memory block.
func (em *EditableMemory) AddBlock(label, content string, maxChars int) *Block {
	em.mu.Lock()
	defer em.mu.Unlock()
	em.dirty = true

	b := &Block{
		ID:        uuid.New().String(),
		Label:     label,
		Content:   content,
		MaxChars:  maxChars,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Version:   1,
	}
	em.blocks[label] = b
	return b
}

// GetBlock returns a block by label.
func (em *EditableMemory) GetBlock(label string) (*Block, bool) {
	em.mu.RLock()
	defer em.mu.RUnlock()
	b, ok := em.blocks[label]
	if !ok {
		return nil, false
	}
	cp := *b
	return &cp, true
}

// AllBlocks returns all blocks.
func (em *EditableMemory) AllBlocks() []*Block {
	em.mu.RLock()
	defer em.mu.RUnlock()
	out := make([]*Block, 0, len(em.blocks))
	for _, b := range em.blocks {
		cp := *b
		out = append(out, &cp)
	}
	return out
}

// RemoveBlock deletes a block.
func (em *EditableMemory) RemoveBlock(label string) bool {
	em.mu.Lock()
	defer em.mu.Unlock()
	if _, ok := em.blocks[label]; !ok {
		return false
	}
	delete(em.blocks, label)
	em.dirty = true
	return true
}

// RenameBlock changes a block's label.
func (em *EditableMemory) RenameBlock(oldLabel, newLabel string) error {
	em.mu.Lock()
	defer em.mu.Unlock()
	b, ok := em.blocks[oldLabel]
	if !ok {
		return fmt.Errorf("block %q not found", oldLabel)
	}
	if _, exists := em.blocks[newLabel]; exists {
		return fmt.Errorf("block %q already exists", newLabel)
	}
	delete(em.blocks, oldLabel)
	b.Label = newLabel
	em.blocks[newLabel] = b
	em.dirty = true
	return nil
}

// ──────────────────────────────────────────────
// Edit operations
// ──────────────────────────────────────────────

// Edit applies an edit operation to a memory block.
func (em *EditableMemory) Edit(req EditRequest) EditResult {
	em.mu.Lock()
	defer em.mu.Unlock()

	result := EditResult{
		BlockLabel: req.BlockLabel,
		Op:         req.Op,
	}

	b, ok := em.blocks[req.BlockLabel]
	if !ok {
		result.Error = fmt.Sprintf("block %q not found", req.BlockLabel)
		em.history = append(em.history, result)
		return result
	}

	if b.ReadOnly {
		result.Error = fmt.Sprintf("block %q is read-only", req.BlockLabel)
		em.history = append(em.history, result)
		return result
	}

	result.OldVersion = b.Version
	result.CharsBefore = len(b.Content)

	var err error
	switch req.Op {
	case OpReplace:
		err = em.doReplace(b, req.OldText, req.NewText)
	case OpInsert:
		err = em.doInsert(b, req.LineNumber, req.NewText)
	case OpPatch:
		err = em.doPatch(b, req.OldText, req.NewText)
	case OpRethink:
		err = em.doRethink(b, req.NewText)
	case OpDelete:
		err = em.doDelete(b, req.LineNumber, req.LineCount)
	default:
		err = fmt.Errorf("unknown op: %s", req.Op)
	}

	if err != nil {
		result.Error = err.Error()
		em.history = append(em.history, result)
		return result
	}

	// Enforce max chars
	if b.MaxChars > 0 && len(b.Content) > b.MaxChars {
		result.Error = fmt.Sprintf("content exceeds max %d chars (got %d)", b.MaxChars, len(b.Content))
		em.history = append(em.history, result)
		return result
	}

	b.Version++
	b.UpdatedAt = time.Now()
	em.dirty = true
	result.Success = true
	result.NewVersion = b.Version
	result.CharsAfter = len(b.Content)
	em.history = append(em.history, result)

	slog.Debug("memory: edited", "block", req.BlockLabel, "op", req.Op, "v", b.Version)
	return result
}

func (em *EditableMemory) doReplace(b *Block, oldText, newText string) error {
	if oldText == "" {
		return fmt.Errorf("replace: old_text is empty")
	}
	if !strings.Contains(b.Content, oldText) {
		return fmt.Errorf("replace: old_text not found in block")
	}
	b.Content = strings.Replace(b.Content, oldText, newText, 1)
	return nil
}

func (em *EditableMemory) doInsert(b *Block, lineNum int, text string) error {
	if lineNum < 1 {
		return fmt.Errorf("insert: line_number must be >= 1")
	}
	lines := b.Lines()
	if lineNum > len(lines)+1 {
		lineNum = len(lines) + 1
	}
	// Insert at position
	newLines := make([]string, 0, len(lines)+1)
	newLines = append(newLines, lines[:lineNum-1]...)
	newLines = append(newLines, text)
	if lineNum-1 < len(lines) {
		newLines = append(newLines, lines[lineNum-1:]...)
	}
	b.Content = strings.Join(newLines, "\n")
	return nil
}

func (em *EditableMemory) doPatch(b *Block, oldText, newText string) error {
	// Simple content-anchored patch: find oldText, replace with newText
	if oldText == "" {
		return fmt.Errorf("patch: old_text anchor is empty")
	}
	if !strings.Contains(b.Content, oldText) {
		return fmt.Errorf("patch: anchor text not found")
	}
	b.Content = strings.Replace(b.Content, oldText, newText, 1)
	return nil
}

func (em *EditableMemory) doRethink(b *Block, newContent string) error {
	b.Content = newContent
	return nil
}

func (em *EditableMemory) doDelete(b *Block, lineNum, count int) error {
	if lineNum < 1 {
		return fmt.Errorf("delete: line_number must be >= 1")
	}
	if count < 1 {
		count = 1
	}
	lines := b.Lines()
	if lineNum > len(lines) {
		return fmt.Errorf("delete: line %d out of range (total %d)", lineNum, len(lines))
	}
	end := lineNum - 1 + count
	if end > len(lines) {
		end = len(lines)
	}
	newLines := make([]string, 0, len(lines)-count)
	newLines = append(newLines, lines[:lineNum-1]...)
	newLines = append(newLines, lines[end:]...)
	b.Content = strings.Join(newLines, "\n")
	return nil
}

// ──────────────────────────────────────────────
// Query & compile
// ──────────────────────────────────────────────

// Compile combines all blocks into a system prompt snippet.
func (em *EditableMemory) Compile() string {
	em.mu.RLock()
	defer em.mu.RUnlock()
	var sb strings.Builder
	for _, b := range em.blocks {
		sb.WriteString(fmt.Sprintf("<%s>\n%s\n</%s>\n\n", b.Label, b.Content, b.Label))
	}
	return sb.String()
}

// History returns edit history.
func (em *EditableMemory) History(limit int) []EditResult {
	em.mu.RLock()
	defer em.mu.RUnlock()
	if limit <= 0 || limit > len(em.history) {
		limit = len(em.history)
	}
	start := len(em.history) - limit
	out := make([]EditResult, limit)
	copy(out, em.history[start:])
	return out
}
