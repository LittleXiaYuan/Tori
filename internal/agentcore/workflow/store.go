package workflow

import (
	"encoding/json"
	"fmt"
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

// JSONStore persists workflows and instances as JSON files.
type JSONStore struct {
	mu      sync.RWMutex
	baseDir string
	defs    map[string]*Definition
	insts   map[string]*Instance
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

// ── Definition operations ──

func (s *JSONStore) SaveDefinition(def *Definition) error {
	if def.ID == "" {
		def.ID = uuid.New().String()[:8]
		def.CreatedAt = time.Now()
	}
	def.UpdatedAt = time.Now()

	s.mu.Lock()
	s.defs[def.ID] = def
	s.mu.Unlock()

	return s.saveDef(def)
}

func (s *JSONStore) GetDefinition(id string) (*Definition, error) {
	s.mu.Lock()
	s.loadAll()
	s.mu.Unlock()

	s.mu.RLock()
	defer s.mu.RUnlock()
	def, ok := s.defs[id]
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
	s.mu.Unlock()
	if ok {
		os.Remove(filepath.Join(s.baseDir, "defs", id+".json"))
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
	s.mu.Unlock()
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
