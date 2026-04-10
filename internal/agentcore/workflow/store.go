package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Store provides persistence for workflow definitions and instances.
type Store interface {
	// Definitions
	SaveDefinition(def *Definition) error
	GetDefinition(id string) (*Definition, error)
	ListDefinitions(tenantID string) ([]*Definition, error)
	DeleteDefinition(id string) error

	// Instances
	CreateInstance(definitionID, tenantID string, variables map[string]any) (*Instance, error)
	GetInstance(id string) (*Instance, error)
	SaveInstance(inst *Instance) error
	ListInstances(tenantID string, limit int) ([]*Instance, error)
}

// kvStore abstracts Ledger KV to avoid import cycles with internal/ledger.
type kvStore interface {
	Put(ctx context.Context, key string, value any) error
	Get(ctx context.Context, key string, dest any) (bool, error)
	Delete(ctx context.Context, key string) error
}

// JSONStore persists workflows and instances as JSON files,
// with optional Ledger KV backing when SetKVStore is called.
type JSONStore struct {
	mu      sync.RWMutex
	baseDir string
	defs    map[string]*Definition
	insts   map[string]*Instance
	kvs     kvStore
}

// NewJSONStore creates a workflow store rooted at dir.
func NewJSONStore(dir string) *JSONStore {
	s := &JSONStore{
		baseDir: dir,
		defs:    make(map[string]*Definition),
		insts:   make(map[string]*Instance),
	}
	s.loadAll()
	return s
}

// SetKVStore enables Ledger KV-backed persistence.
// Existing file data is automatically migrated on first call.
func (s *JSONStore) SetKVStore(kvs kvStore) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.kvs = kvs

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	migrated := 0
	for id, def := range s.defs {
		if err := kvs.Put(ctx, "def:"+id, def); err != nil {
			slog.Warn("workflow store: migrate def failed", "id", id, "err", err)
		} else {
			migrated++
		}
	}
	for id, inst := range s.insts {
		if err := kvs.Put(ctx, "inst:"+id, inst); err != nil {
			slog.Warn("workflow store: migrate inst failed", "id", id, "err", err)
		} else {
			migrated++
		}
	}
	if migrated > 0 {
		slog.Info("workflow store: migrated to Ledger KV", "count", migrated)
	}
}

// ── Definition operations ──

func (s *JSONStore) SaveDefinition(def *Definition) error {
	if def.ID == "" {
		def.ID = uuid.New().String()[:8]
		def.CreatedAt = time.Now()
	}
	def.UpdatedAt = time.Now()

	s.mu.Lock()
	s.defs[def.ID] = def
	kvs := s.kvs
	s.mu.Unlock()

	if kvs != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := kvs.Put(ctx, "def:"+def.ID, def); err != nil {
			slog.Warn("workflow store: KV save def failed", "id", def.ID, "err", err)
			return s.saveDef(def)
		}
		return nil
	}
	return s.saveDef(def)
}

func (s *JSONStore) GetDefinition(id string) (*Definition, error) {
	s.mu.RLock()
	kvs := s.kvs
	def, ok := s.defs[id]
	s.mu.RUnlock()

	if ok {
		return def, nil
	}

	if kvs != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		var d Definition
		found, err := kvs.Get(ctx, "def:"+id, &d)
		if err == nil && found {
			s.mu.Lock()
			s.defs[id] = &d
			s.mu.Unlock()
			return &d, nil
		}
	}

	s.mu.Lock()
	s.loadAll()
	s.mu.Unlock()

	s.mu.RLock()
	defer s.mu.RUnlock()
	def, ok = s.defs[id]
	if !ok {
		return nil, fmt.Errorf("workflow definition %s not found", id)
	}
	return def, nil
}

func (s *JSONStore) ListDefinitions(tenantID string) ([]*Definition, error) {
	s.mu.Lock()
	s.loadAll()
	s.mu.Unlock()

	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []*Definition
	for _, def := range s.defs {
		if tenantID == "" || def.TenantID == tenantID {
			out = append(out, def)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out, nil
}

func (s *JSONStore) DeleteDefinition(id string) error {
	s.mu.Lock()
	_, ok := s.defs[id]
	if ok {
		delete(s.defs, id)
	}
	kvs := s.kvs
	s.mu.Unlock()

	if ok {
		os.Remove(filepath.Join(s.baseDir, "defs", id+".json"))
	}
	if kvs != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = kvs.Delete(ctx, "def:"+id)
	}
	return nil
}

// ── Instance operations ──

func (s *JSONStore) CreateInstance(definitionID, tenantID string, variables map[string]any) (*Instance, error) {
	def, err := s.GetDefinition(definitionID)
	if err != nil {
		return nil, err
	}
	if variables == nil {
		variables = make(map[string]any)
	}
	// Apply default values for missing variables
	for _, v := range def.Variables {
		if _, exists := variables[v.Name]; !exists && v.DefaultValue != nil {
			variables[v.Name] = v.DefaultValue
		}
	}

	inst := &Instance{
		ID:           uuid.New().String()[:8],
		DefinitionID: definitionID,
		Version:      def.Version,
		Status:       InstancePending,
		Variables:    variables,
		NodeStates:   make(map[string]*NodeState),
		TenantID:     tenantID,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	s.mu.Lock()
	s.insts[inst.ID] = inst
	s.mu.Unlock()

	if err := s.saveInst(inst); err != nil {
		return nil, err
	}
	return inst, nil
}

func (s *JSONStore) GetInstance(id string) (*Instance, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	inst, ok := s.insts[id]
	if !ok {
		return nil, fmt.Errorf("workflow instance %s not found", id)
	}
	return inst, nil
}

func (s *JSONStore) SaveInstance(inst *Instance) error {
	inst.UpdatedAt = time.Now()
	s.mu.Lock()
	s.insts[inst.ID] = inst
	kvs := s.kvs
	s.mu.Unlock()

	if kvs != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := kvs.Put(ctx, "inst:"+inst.ID, inst); err != nil {
			slog.Warn("workflow store: KV save inst failed", "id", inst.ID, "err", err)
			return s.saveInst(inst)
		}
		return nil
	}
	return s.saveInst(inst)
}

func (s *JSONStore) ListInstances(tenantID string, limit int) ([]*Instance, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []*Instance
	for _, inst := range s.insts {
		if tenantID == "" || inst.TenantID == tenantID {
			out = append(out, inst)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// ── Persistence ──

func (s *JSONStore) saveDef(def *Definition) error {
	dir := filepath.Join(s.baseDir, "defs")
	os.MkdirAll(dir, 0o755)
	data, err := json.MarshalIndent(def, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, def.ID+".json"), data, 0o644)
}

func (s *JSONStore) saveInst(inst *Instance) error {
	dir := filepath.Join(s.baseDir, "instances")
	os.MkdirAll(dir, 0o755)
	data, err := json.MarshalIndent(inst, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, inst.ID+".json"), data, 0o644)
}

func (s *JSONStore) loadAll() {
	// Load definitions
	defDir := filepath.Join(s.baseDir, "defs")
	entries, _ := os.ReadDir(defDir)
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(defDir, e.Name()))
		if err != nil {
			continue
		}
		var def Definition
		if err := json.Unmarshal(data, &def); err != nil {
			continue
		}
		s.defs[def.ID] = &def
	}

	// Load instances
	instDir := filepath.Join(s.baseDir, "instances")
	entries, _ = os.ReadDir(instDir)
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(instDir, e.Name()))
		if err != nil {
			continue
		}
		var inst Instance
		if err := json.Unmarshal(data, &inst); err != nil {
			continue
		}
		s.insts[inst.ID] = &inst
	}
}
