package trigger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ──────────────────────────────────────────────
// Store — 触发器持久化存储
// ──────────────────────────────────────────────

// Store 触发器存储
type Store struct {
	mu       sync.RWMutex
	triggers map[string]*TriggerDef // id -> trigger
	runs     map[string]*TriggerRun // run_id -> run
	events   []TriggerEvent         // 事件日志（环形缓冲区）

	dataDir   string
	maxEvents int // 最大事件数（默认 1000）
	maxRuns   int // 最大执行记录数（默认 500）
}

// NewStore 创建触发器存储
func NewStore(dataDir string) *Store {
	if dataDir == "" {
		dataDir = "data/triggers"
	}
	os.MkdirAll(dataDir, 0o755)

	s := &Store{
		triggers:  make(map[string]*TriggerDef),
		runs:      make(map[string]*TriggerRun),
		events:    make([]TriggerEvent, 0, 128),
		dataDir:   dataDir,
		maxEvents: 1000,
		maxRuns:   500,
	}

	s.load()
	return s
}

// ──────────────────────────────────────────────
// Trigger CRUD
// ──────────────────────────────────────────────

// Create 创建触发器
func (s *Store) Create(t *TriggerDef) error {
	if t.ID == "" {
		t.ID = "trg_" + uuid.New().String()[:8]
	}
	if t.TenantID == "" {
		return fmt.Errorf("tenant_id is required")
	}
	if len(t.Actions) == 0 {
		return fmt.Errorf("at least one action is required")
	}

	now := time.Now()
	t.CreatedAt = now
	t.UpdatedAt = now
	if t.Status == "" {
		t.Status = TriggerStatusActive
	}

	s.mu.Lock()
	s.triggers[t.ID] = t
	s.mu.Unlock()

	s.persist()
	s.logEvent(TriggerEvent{
		TriggerID: t.ID,
		TenantID:  t.TenantID,
		EventType: EventTypeEnabled,
		Message:   fmt.Sprintf("Trigger created: %s", t.Name),
	})

	return nil
}

// Get 获取触发器
func (s *Store) Get(id string) (*TriggerDef, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.triggers[id]
	if !ok {
		return nil, false
	}
	cp := *t
	return &cp, true
}

// Update 更新触发器
func (s *Store) Update(t *TriggerDef) error {
	s.mu.Lock()
	existing, ok := s.triggers[t.ID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("trigger not found: %s", t.ID)
	}

	t.UpdatedAt = time.Now()
	t.CreatedAt = existing.CreatedAt // 保留创建时间
	s.triggers[t.ID] = t
	s.mu.Unlock()

	s.persist()
	s.logEvent(TriggerEvent{
		TriggerID: t.ID,
		TenantID:  t.TenantID,
		EventType: EventTypeUpdated,
		Message:   fmt.Sprintf("Trigger updated: %s", t.Name),
	})

	return nil
}

// Delete 删除触发器
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	t, ok := s.triggers[id]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("trigger not found: %s", id)
	}

	delete(s.triggers, id)
	s.mu.Unlock()

	s.persist()
	s.logEvent(TriggerEvent{
		TriggerID: id,
		TenantID:  t.TenantID,
		EventType: EventTypeDeleted,
		Message:   fmt.Sprintf("Trigger deleted: %s", t.Name),
	})

	return nil
}

// List 列出触发器
func (s *Store) List(tenantID string, filter func(*TriggerDef) bool) []*TriggerDef {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*TriggerDef
	for _, t := range s.triggers {
		if tenantID != "" && t.TenantID != tenantID {
			continue
		}
		if filter != nil && !filter(t) {
			continue
		}
		cp := *t
		result = append(result, &cp)
	}
	return result
}

// ──────────────────────────────────────────────
// TriggerRun CRUD
// ──────────────────────────────────────────────

// CreateRun 创建执行记录
func (s *Store) CreateRun(run *TriggerRun) error {
	if run.ID == "" {
		run.ID = "run_" + uuid.New().String()[:8]
	}

	s.mu.Lock()
	s.runs[run.ID] = run
	// 限制最大记录数
	if len(s.runs) > s.maxRuns {
		s.cleanOldRuns()
	}
	s.mu.Unlock()

	return nil
}

// GetRun 获取执行记录
func (s *Store) GetRun(id string) (*TriggerRun, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	run, ok := s.runs[id]
	if !ok {
		return nil, false
	}
	cp := *run
	return &cp, true
}

// UpdateRun 更新执行记录
func (s *Store) UpdateRun(run *TriggerRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[run.ID] = run
	return nil
}

// ListRuns 列出执行记录
func (s *Store) ListRuns(triggerID string, limit int) []*TriggerRun {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*TriggerRun
	for _, run := range s.runs {
		if triggerID != "" && run.TriggerID != triggerID {
			continue
		}
		cp := *run
		result = append(result, &cp)
	}

	// 按时间倒序排序
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].StartedAt.Before(result[j].StartedAt) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result
}

// ──────────────────────────────────────────────
// Event Log
// ──────────────────────────────────────────────

// logEvent 记录事件
func (s *Store) logEvent(event TriggerEvent) {
	if event.ID == "" {
		event.ID = "evt_" + uuid.New().String()[:8]
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	s.mu.Lock()
	s.events = append(s.events, event)
	if len(s.events) > s.maxEvents {
		s.events = s.events[len(s.events)-s.maxEvents:]
	}
	s.mu.Unlock()
}

// ListEvents 列出事件
func (s *Store) ListEvents(triggerID string, limit int) []TriggerEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []TriggerEvent
	for i := len(s.events) - 1; i >= 0; i-- {
		evt := s.events[i]
		if triggerID != "" && evt.TriggerID != triggerID {
			continue
		}
		result = append(result, evt)
		if limit > 0 && len(result) >= limit {
			break
		}
	}

	return result
}

// ──────────────────────────────────────────────
// Persistence
// ──────────────────────────────────────────────

type persistData struct {
	Triggers map[string]*TriggerDef `json:"triggers"`
	Runs     map[string]*TriggerRun `json:"runs"`
	Events   []TriggerEvent         `json:"events"`
}

func (s *Store) persist() {
	s.mu.RLock()
	data := persistData{
		Triggers: s.triggers,
		Runs:     s.runs,
		Events:   s.events,
	}
	s.mu.RUnlock()

	b, _ := json.MarshalIndent(data, "", "  ")
	os.WriteFile(filepath.Join(s.dataDir, "triggers.json"), b, 0o644)
}

func (s *Store) load() {
	path := filepath.Join(s.dataDir, "triggers.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}

	var data persistData
	if err := json.Unmarshal(b, &data); err != nil {
		return
	}

	s.mu.Lock()
	s.triggers = data.Triggers
	s.runs = data.Runs
	s.events = data.Events
	s.mu.Unlock()
}

func (s *Store) cleanOldRuns() {
	// 保留最近的 maxRuns 条记录
	var runs []*TriggerRun
	for _, run := range s.runs {
		runs = append(runs, run)
	}

	// 按时间排序
	for i := 0; i < len(runs)-1; i++ {
		for j := i + 1; j < len(runs); j++ {
			if runs[i].StartedAt.Before(runs[j].StartedAt) {
				runs[i], runs[j] = runs[j], runs[i]
			}
		}
	}

	// 删除旧记录
	if len(runs) > s.maxRuns {
		for i := s.maxRuns; i < len(runs); i++ {
			delete(s.runs, runs[i].ID)
		}
	}
}
