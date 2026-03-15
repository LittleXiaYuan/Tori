package subagent

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Subagent is a spawned child agent with its own context and skills.
type Subagent struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	ParentID    string           `json:"parent_id"` // bot or agent that spawned it
	Messages    []map[string]any `json:"messages"`
	Skills      []string         `json:"skills"`
	Metadata    map[string]any   `json:"metadata"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

// Manager manages subagent lifecycles.
type Manager struct {
	mu     sync.RWMutex
	agents map[string]*Subagent
}

// NewManager creates a subagent manager.
func NewManager() *Manager {
	return &Manager{agents: make(map[string]*Subagent)}
}

// Spawn creates a new subagent.
func (m *Manager) Spawn(parentID, name, description string, skills []string) (*Subagent, error) {
	if name == "" {
		return nil, fmt.Errorf("subagent name required")
	}
	if skills == nil {
		skills = []string{}
	}
	sa := &Subagent{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		ParentID:    parentID,
		Messages:    []map[string]any{},
		Skills:      skills,
		Metadata:    map[string]any{},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	m.mu.Lock()
	m.agents[sa.ID] = sa
	m.mu.Unlock()
	return sa, nil
}

// Get returns a subagent by ID.
func (m *Manager) Get(id string) (*Subagent, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sa, ok := m.agents[id]
	if !ok {
		return nil, false
	}
	copy := *sa
	return &copy, true
}

// List returns all subagents for a parent.
func (m *Manager) List(parentID string) []Subagent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []Subagent
	for _, sa := range m.agents {
		if parentID == "" || sa.ParentID == parentID {
			result = append(result, *sa)
		}
	}
	return result
}

// AppendMessages adds messages to a subagent's context.
func (m *Manager) AppendMessages(id string, msgs []map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	sa, ok := m.agents[id]
	if !ok {
		return fmt.Errorf("subagent not found: %s", id)
	}
	sa.Messages = append(sa.Messages, msgs...)
	sa.UpdatedAt = time.Now()
	return nil
}

// SetSkills replaces the subagent's skill list.
func (m *Manager) SetSkills(id string, skills []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	sa, ok := m.agents[id]
	if !ok {
		return fmt.Errorf("subagent not found: %s", id)
	}
	sa.Skills = skills
	sa.UpdatedAt = time.Now()
	return nil
}

// Destroy removes a subagent.
func (m *Manager) Destroy(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.agents[id]; !ok {
		return false
	}
	delete(m.agents, id)
	return true
}

// Count returns total subagent count.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.agents)
}
