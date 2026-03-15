package models

import (
	"fmt"
	"sync"
)

// ModelType distinguishes chat vs embedding models.
type ModelType string

const (
	TypeChat      ModelType = "chat"
	TypeEmbedding ModelType = "embedding"
)

// ClientType identifies the LLM API protocol.
type ClientType string

const (
	ClientOpenAI    ClientType = "openai"
	ClientAnthropic ClientType = "anthropic"
	ClientGoogle    ClientType = "google"
	ClientOllama    ClientType = "ollama"
)

// Model represents a registered LLM model.
type Model struct {
	ID                string     `json:"id"`
	ModelID           string     `json:"model_id"`
	Name              string     `json:"name"`
	Type              ModelType  `json:"type"`
	ClientType        ClientType `json:"client_type"`
	BaseURL           string     `json:"base_url,omitempty"`
	APIKey            string     `json:"-"` // never expose
	InputModalities   []string   `json:"input_modalities,omitempty"`
	SupportsReasoning bool       `json:"supports_reasoning"`
	Dimensions        int        `json:"dimensions,omitempty"` // for embedding models
}

// HasModality checks if the model supports a given input type.
func (m *Model) HasModality(mod string) bool {
	for _, v := range m.InputModalities {
		if v == mod {
			return true
		}
	}
	return false
}

// IsMultimodal returns true if model supports beyond text.
func (m *Model) IsMultimodal() bool {
	for _, v := range m.InputModalities {
		if v != "text" {
			return true
		}
	}
	return false
}

// Registry manages available LLM models.
type Registry struct {
	mu      sync.RWMutex
	models  map[string]*Model // keyed by ID
	primary string            // default model ID
}

// NewRegistry creates a model registry.
func NewRegistry() *Registry {
	return &Registry{models: make(map[string]*Model)}
}

// Register adds a model.
func (r *Registry) Register(m Model) error {
	if m.ID == "" || m.ModelID == "" {
		return fmt.Errorf("model id and model_id required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.models[m.ID] = &m
	if r.primary == "" && m.Type == TypeChat {
		r.primary = m.ID
	}
	return nil
}

// Get returns a model by ID.
func (r *Registry) Get(id string) (*Model, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.models[id]
	if !ok {
		return nil, false
	}
	copy := *m
	return &copy, true
}

// GetByModelID finds a model by its provider model ID string.
func (r *Registry) GetByModelID(modelID string) (*Model, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, m := range r.models {
		if m.ModelID == modelID {
			copy := *m
			return &copy, true
		}
	}
	return nil, false
}

// Primary returns the default chat model.
func (r *Registry) Primary() (*Model, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.primary == "" {
		return nil, false
	}
	m, ok := r.models[r.primary]
	if !ok {
		return nil, false
	}
	copy := *m
	return &copy, true
}

// SetPrimary sets the default model.
func (r *Registry) SetPrimary(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.models[id]; !ok {
		return false
	}
	r.primary = id
	return true
}

// List returns all models, optionally filtered by type.
func (r *Registry) List(filterType ModelType) []Model {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []Model
	for _, m := range r.models {
		if filterType != "" && m.Type != filterType {
			continue
		}
		result = append(result, *m)
	}
	return result
}

// Remove deletes a model.
func (r *Registry) Remove(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.models[id]; !ok {
		return false
	}
	delete(r.models, id)
	if r.primary == id {
		r.primary = ""
	}
	return true
}

// Count returns total model count.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.models)
}
