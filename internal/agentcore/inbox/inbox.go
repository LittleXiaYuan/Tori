package inbox

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// kvStore abstracts Ledger KV to avoid import cycles with internal/ledger.
type kvStore interface {
	Put(ctx context.Context, key string, value any) error
	Get(ctx context.Context, key string, dest any) (bool, error)
}

// Action defines what the agent should do with an inbox item.
const (
	ActionNotify  = "notify"  // informational, shown in heartbeat
	ActionTrigger = "trigger" // triggers an agent response
)

// Item represents a message queued for the agent's attention.
type Item struct {
	ID        string         `json:"id"`
	Source    string         `json:"source"`   // e.g. "email", "telegram", "cron", "webhook"
	Header    map[string]any `json:"header"`
	Content   string         `json:"content"`
	Action    string         `json:"action"`   // "notify" or "trigger"
	IsRead    bool           `json:"is_read"`
	CreatedAt time.Time      `json:"created_at"`
	ReadAt    *time.Time     `json:"read_at,omitempty"`
}

// CountResult holds inbox statistics.
type CountResult struct {
	Unread int `json:"unread"`
	Total  int `json:"total"`
}

// Store is an in-memory inbox for cross-channel message queuing.
type Store struct {
	mu      sync.RWMutex
	items   []Item
	maxSize int
	kvs     kvStore
	dirty   int
}

// NewStore creates an inbox store with the given max capacity.
func NewStore(maxSize int) *Store {
	if maxSize <= 0 {
		maxSize = 500
	}
	return &Store{
		items:   make([]Item, 0),
		maxSize: maxSize,
	}
}

// SetKVStore enables Ledger KV-backed persistence for inbox.
func (s *Store) SetKVStore(kvs kvStore) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.kvs = kvs

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var items []Item
	found, err := kvs.Get(ctx, "items", &items)
	if err != nil {
		slog.Warn("inbox: KV load failed", "err", err)
		return
	}
	if found && len(items) > 0 {
		s.items = items
		slog.Info("inbox: loaded from KV", "count", len(items))
	}
}

// FlushToKV persists current items to KV. Called during shutdown.
func (s *Store) FlushToKV() {
	s.mu.RLock()
	kvs := s.kvs
	snap := make([]Item, len(s.items))
	copy(snap, s.items)
	s.mu.RUnlock()

	if kvs == nil || len(snap) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := kvs.Put(ctx, "items", snap); err != nil {
		slog.Error("inbox: flush to KV failed", "err", err)
	}
}

func (s *Store) persistKV() {
	s.dirty++
	if s.kvs == nil || s.dirty < 5 {
		return
	}
	s.dirty = 0
	snap := make([]Item, len(s.items))
	copy(snap, s.items)
	kvs := s.kvs
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := kvs.Put(ctx, "items", snap); err != nil {
			slog.Warn("inbox: KV save failed", "err", err)
		}
	}()
}

// Push adds a new item to the inbox.
func (s *Store) Push(source, content, action string, header map[string]any) (*Item, error) {
	if content == "" {
		return nil, fmt.Errorf("inbox content is required")
	}
	if action != ActionNotify && action != ActionTrigger {
		action = ActionNotify
	}
	if header == nil {
		header = map[string]any{}
	}

	item := Item{
		ID:        uuid.New().String(),
		Source:    source,
		Header:    header,
		Content:   content,
		Action:    action,
		CreatedAt: time.Now(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.items = append(s.items, item)
	if len(s.items) > s.maxSize {
		s.items = s.items[len(s.items)-s.maxSize:]
	}
	s.persistKV()
	return &item, nil
}

// List returns items with optional filtering. Newest first.
func (s *Store) List(onlyUnread bool, limit int) []Item {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	var result []Item
	for i := len(s.items) - 1; i >= 0 && len(result) < limit; i-- {
		if onlyUnread && s.items[i].IsRead {
			continue
		}
		result = append(result, s.items[i])
	}
	return result
}

// Unread returns unread items (newest first).
func (s *Store) Unread(limit int) []Item {
	return s.List(true, limit)
}

// Get returns a single item by ID.
func (s *Store) Get(id string) (*Item, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := range s.items {
		if s.items[i].ID == id {
			copy := s.items[i]
			return &copy, true
		}
	}
	return nil, false
}

// MarkRead marks one or more items as read.
func (s *Store) MarkRead(ids []string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	idSet := make(map[string]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}

	count := 0
	now := time.Now()
	for i := range s.items {
		if idSet[s.items[i].ID] && !s.items[i].IsRead {
			s.items[i].IsRead = true
			s.items[i].ReadAt = &now
			count++
		}
	}
	if count > 0 {
		s.persistKV()
	}
	return count
}

// MarkAllRead marks all items as read.
func (s *Store) MarkAllRead() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	now := time.Now()
	for i := range s.items {
		if !s.items[i].IsRead {
			s.items[i].IsRead = true
			s.items[i].ReadAt = &now
			count++
		}
	}
	if count > 0 {
		s.persistKV()
	}
	return count
}

// Delete removes an item by ID.
func (s *Store) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.items {
		if s.items[i].ID == id {
			s.items = append(s.items[:i], s.items[i+1:]...)
			s.persistKV()
			return true
		}
	}
	return false
}

// Count returns inbox statistics.
func (s *Store) Count() CountResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	unread := 0
	for _, item := range s.items {
		if !item.IsRead {
			unread++
		}
	}
	return CountResult{
		Unread: unread,
		Total:  len(s.items),
	}
}

// PendingTriggers returns unread items with action=trigger.
func (s *Store) PendingTriggers(limit int) []Item {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}
	var result []Item
	for i := len(s.items) - 1; i >= 0 && len(result) < limit; i-- {
		if !s.items[i].IsRead && s.items[i].Action == ActionTrigger {
			result = append(result, s.items[i])
		}
	}
	return result
}

// Summary returns a text summary of unread items for heartbeat context.
func (s *Store) Summary(maxItems int) string {
	unread := s.Unread(maxItems)
	if len(unread) == 0 {
		return ""
	}
	summary := fmt.Sprintf("收件箱有 %d 条未读消息:\n", s.Count().Unread)
	for i, item := range unread {
		summary += fmt.Sprintf("%d. [%s] %s\n", i+1, item.Source, truncate(item.Content, 100))
	}
	return summary
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
