package task

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// Template — reusable task blueprint with variable placeholders
//
// Templates define pre-configured step sequences that can be
// instantiated with variable substitution. Stored in data/templates/.
// ──────────────────────────────────────────────

// Template is a reusable task blueprint.
type Template struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"` // supports {{var}} placeholders
	Variables   []TemplateVar  `json:"variables"`   // declared variables
	Steps       []TemplateStep `json:"steps"`
	Tags        []string       `json:"tags,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

// TemplateVar describes a variable in the template.
type TemplateVar struct {
	Name        string `json:"name"`                  // variable name (used as {{name}})
	Description string `json:"description,omitempty"` // human-readable hint
	Default     string `json:"default,omitempty"`     // default value
	Required    bool   `json:"required"`
}

// TemplateStep is a step definition within a template.
type TemplateStep struct {
	Action    string         `json:"action"`               // supports {{var}} placeholders
	SkillName string         `json:"skill_name,omitempty"` // supports {{var}} placeholders
	Args      map[string]any `json:"args,omitempty"`       // values support {{var}} placeholders
	Group     int            `json:"group,omitempty"`      // parallel group (0=sequential)
}

// TemplateStore manages task templates.
type TemplateStore struct {
	mu   sync.RWMutex
	data map[string]*Template
	dir  string
	kvs  kvStore
}

// NewTemplateStore creates a template store persisting to the given directory.
func NewTemplateStore(dir string) *TemplateStore {
	ts := &TemplateStore{data: make(map[string]*Template), dir: dir}
	ts.load()
	return ts
}

// SetKVStore enables Ledger KV-backed persistence for templates.
// Migrates existing file-based templates to KV on first call.
func (ts *TemplateStore) SetKVStore(kvs kvStore) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.kvs = kvs

	if len(ts.data) == 0 {
		ts.loadFromKV()
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := kvs.Put(ctx, "templates", ts.data); err != nil {
		slog.Warn("template store: KV migration failed", "err", err)
		return
	}
	slog.Info("template store: migrated to Ledger KV", "count", len(ts.data))
}

func (ts *TemplateStore) loadFromKV() {
	if ts.kvs == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var data map[string]*Template
	found, err := ts.kvs.Get(ctx, "templates", &data)
	if err != nil {
		slog.Warn("template store: KV load failed", "err", err)
		return
	}
	if found && len(data) > 0 {
		ts.data = data
		slog.Info("template store: loaded from KV", "count", len(data))
	}
}

// Create adds a new template.
func (ts *TemplateStore) Create(t *Template) error {
	if t.Name == "" {
		return fmt.Errorf("template name is required")
	}
	if t.ID == "" {
		t.ID = fmt.Sprintf("tpl-%d", time.Now().UnixMilli())
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now()
	}

	ts.mu.Lock()
	ts.data[t.ID] = t
	ts.mu.Unlock()

	return ts.persistAll()
}

// Get retrieves a template by ID.
func (ts *TemplateStore) Get(id string) (*Template, bool) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	t, ok := ts.data[id]
	return t, ok
}

// List returns all templates.
func (ts *TemplateStore) List() []*Template {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	var result []*Template
	for _, t := range ts.data {
		result = append(result, t)
	}
	return result
}

// Delete removes a template by ID.
func (ts *TemplateStore) Delete(id string) bool {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if _, ok := ts.data[id]; !ok {
		return false
	}
	delete(ts.data, id)
	os.Remove(filepath.Join(ts.dir, id+".json"))
	_ = ts.persistAllLocked()
	return true
}

// Instantiate creates a Task from a template with variable values.
func (ts *TemplateStore) Instantiate(templateID string, vars map[string]string, tenantID string) (*Task, error) {
	ts.mu.RLock()
	tpl, ok := ts.data[templateID]
	ts.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("template %s not found", templateID)
	}

	// Validate required variables
	for _, v := range tpl.Variables {
		val, exists := vars[v.Name]
		if !exists || val == "" {
			if v.Required && v.Default == "" {
				return nil, fmt.Errorf("variable %q is required", v.Name)
			}
			if v.Default != "" {
				vars[v.Name] = v.Default
			}
		}
	}

	// Build steps with variable substitution
	steps := make([]Step, len(tpl.Steps))
	for i, ts := range tpl.Steps {
		steps[i] = Step{
			ID:         i + 1,
			Action:     substituteVars(ts.Action, vars),
			SkillName:  substituteVars(ts.SkillName, vars),
			Args:       substituteArgsVars(ts.Args, vars),
			Status:     StepPending,
			MaxRetries: DefaultMaxRetries,
			Group:      ts.Group,
		}
	}

	now := time.Now()
	task := &Task{
		ID:          fmt.Sprintf("%x", now.UnixMilli()),
		Title:       substituteVars(tpl.Name, vars),
		Description: substituteVars(tpl.Description, vars),
		Status:      StatusPending,
		Steps:       steps,
		TenantID:    tenantID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	return task, nil
}

func substituteVars(s string, vars map[string]string) string {
	for k, v := range vars {
		s = strings.ReplaceAll(s, "{{"+k+"}}", v)
	}
	return s
}

func substituteArgsVars(args map[string]any, vars map[string]string) map[string]any {
	if args == nil {
		return nil
	}
	result := make(map[string]any, len(args))
	for k, v := range args {
		if s, ok := v.(string); ok {
			result[k] = substituteVars(s, vars)
		} else {
			result[k] = v
		}
	}
	return result
}

func (ts *TemplateStore) persistAll() error {
	ts.mu.RLock()
	snap := make(map[string]*Template, len(ts.data))
	for k, v := range ts.data {
		snap[k] = v
	}
	kvs := ts.kvs
	ts.mu.RUnlock()

	if kvs != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := kvs.Put(ctx, "templates", snap); err != nil {
			slog.Warn("template store: KV save failed, falling back to file", "err", err)
		} else {
			return nil
		}
	}

	for _, t := range snap {
		if err := ts.saveOne(t); err != nil {
			return err
		}
	}
	return nil
}

// persistAllLocked is like persistAll but assumes mu is already held.
func (ts *TemplateStore) persistAllLocked() error {
	if ts.kvs != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := ts.kvs.Put(ctx, "templates", ts.data); err != nil {
			slog.Warn("template store: KV save failed", "err", err)
		} else {
			return nil
		}
	}
	return nil
}

func (ts *TemplateStore) load() {
	os.MkdirAll(ts.dir, 0755)
	entries, err := os.ReadDir(ts.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(ts.dir, e.Name()))
		if err != nil {
			continue
		}
		var t Template
		if json.Unmarshal(data, &t) == nil && t.ID != "" {
			ts.data[t.ID] = &t
		}
	}
}

func (ts *TemplateStore) saveOne(t *Template) error {
	os.MkdirAll(ts.dir, 0755)
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(ts.dir, t.ID+".json"), data, 0644)
}
