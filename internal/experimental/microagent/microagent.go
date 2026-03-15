package microagent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ──────────────────────────────────────────────
// Scope — where a microagent applies
// ──────────────────────────────────────────────

type Scope string

const (
	ScopeGlobal Scope = "global" // applies everywhere
	ScopeRepo   Scope = "repo"   // repo-specific (.openhands/microagents/)
	ScopeTask   Scope = "task"   // task-specific, temporary
)

// ──────────────────────────────────────────────
// MicroAgent — a specialized prompt enhancement
// ──────────────────────────────────────────────

// MicroAgent is a domain/repo-specific prompt snippet that enhances agent behavior.
type MicroAgent struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Scope       Scope             `json:"scope"`
	Trigger     string            `json:"trigger,omitempty"`   // keyword trigger (empty = always active)
	Content     string            `json:"content"`              // the prompt content
	Enabled     bool              `json:"enabled"`
	Priority    int               `json:"priority"`             // higher = injected first
	Tags        []string          `json:"tags,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// ──────────────────────────────────────────────
// Registry — manages microagents
// ──────────────────────────────────────────────

// Registry holds all loaded microagents.
type Registry struct {
	mu     sync.RWMutex
	agents map[string]*MicroAgent
}

// NewRegistry creates a microagent registry.
func NewRegistry() *Registry {
	return &Registry{agents: make(map[string]*MicroAgent)}
}

// Register adds a microagent.
func (r *Registry) Register(ma *MicroAgent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if ma.Metadata == nil {
		ma.Metadata = make(map[string]string)
	}
	r.agents[ma.Name] = ma
}

// Get returns a microagent by name.
func (r *Registry) Get(name string) (*MicroAgent, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ma, ok := r.agents[name]
	return ma, ok
}

// Remove unregisters a microagent.
func (r *Registry) Remove(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.agents[name]; !ok {
		return false
	}
	delete(r.agents, name)
	return true
}

// All returns all microagents.
func (r *Registry) All() []*MicroAgent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*MicroAgent, 0, len(r.agents))
	for _, ma := range r.agents {
		out = append(out, ma)
	}
	return out
}

// ──────────────────────────────────────────────
// Resolve — select microagents for a context
// ──────────────────────────────────────────────

// Resolve returns microagents relevant to the given context message.
// It selects: always-active agents + keyword-triggered agents.
func (r *Registry) Resolve(message string) []*MicroAgent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	lower := strings.ToLower(message)
	var matched []*MicroAgent

	for _, ma := range r.agents {
		if !ma.Enabled {
			continue
		}
		if ma.Trigger == "" {
			// Always active
			matched = append(matched, ma)
		} else if strings.Contains(lower, strings.ToLower(ma.Trigger)) {
			matched = append(matched, ma)
		}
	}

	// Sort by priority (higher first)
	sortByPriority(matched)
	return matched
}

// ResolveByScope returns microagents for a specific scope.
func (r *Registry) ResolveByScope(scope Scope) []*MicroAgent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*MicroAgent
	for _, ma := range r.agents {
		if ma.Enabled && ma.Scope == scope {
			out = append(out, ma)
		}
	}
	sortByPriority(out)
	return out
}

// CompilePrompt generates a system prompt snippet from resolved microagents.
func (r *Registry) CompilePrompt(message string) string {
	agents := r.Resolve(message)
	if len(agents) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, ma := range agents {
		sb.WriteString(fmt.Sprintf("## %s\n%s\n\n", ma.Name, ma.Content))
	}
	return sb.String()
}

// ──────────────────────────────────────────────
// Loader — discover microagents from filesystem
// ──────────────────────────────────────────────

// LoadFromDirectory loads microagent .md files from a directory.
// Each .md file becomes a microagent. Frontmatter is parsed for metadata.
func LoadFromDirectory(dir string, scope Scope, registry *Registry) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	loaded := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".md")
		ma := parseMicroAgentMD(name, string(data), scope)
		registry.Register(ma)
		loaded++
	}
	return loaded, nil
}

// parseMicroAgentMD extracts frontmatter and content from a markdown file.
func parseMicroAgentMD(name, content string, scope Scope) *MicroAgent {
	ma := &MicroAgent{
		Name:    name,
		Scope:   scope,
		Enabled: true,
	}

	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content[3:], "---", 2)
		if len(parts) == 2 {
			// Parse frontmatter
			for _, line := range strings.Split(parts[0], "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				kv := strings.SplitN(line, ":", 2)
				if len(kv) != 2 {
					continue
				}
				key := strings.TrimSpace(kv[0])
				val := strings.TrimSpace(kv[1])
				switch key {
				case "name":
					ma.Name = val
				case "description":
					ma.Description = val
				case "trigger":
					ma.Trigger = val
				case "enabled":
					ma.Enabled = val != "false"
				}
			}
			ma.Content = strings.TrimSpace(parts[1])
		} else {
			ma.Content = content
		}
	} else {
		ma.Content = content
	}

	return ma
}

// ──────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────

func sortByPriority(agents []*MicroAgent) {
	for i := 0; i < len(agents); i++ {
		for j := i + 1; j < len(agents); j++ {
			if agents[j].Priority > agents[i].Priority {
				agents[i], agents[j] = agents[j], agents[i]
			}
		}
	}
}
