package skillmarket

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

	"yunque-agent/pkg/skills"
)

// SkillFileLoader watches data/skills/ for user-created SKILL.md skill packages
// and hot-loads them into the SkillRegistry.
//
// Directory structure:
//
//	data/skills/
//	  my-skill/
//	    SKILL.md          (required — LLM instructions)
//	    meta.json         (optional — name, description, parameters, tags)
//	    scripts/          (optional — helper scripts referenced by SKILL.md)
//	      helper.py
//	    templates/        (optional — output templates)
type SkillFileLoader struct {
	dir      string
	registry *skills.Registry
	mu       sync.Mutex
	stopCh   chan struct{}
	onChange func()

	loaded map[string]time.Time // slug → manifest mod time
}

// SkillMeta is the optional meta.json file in a skill directory.
type SkillFileMeta struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Version     string            `json:"version,omitempty"`
	Author      string            `json:"author,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Permissions []string          `json:"permissions,omitempty"`
	Parameters  map[string]SkillFileParam `json:"parameters,omitempty"`
}

// SkillFileParam defines a parameter in meta.json.
type SkillFileParam struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required,omitempty"`
}

// NewSkillFileLoader creates a loader that scans skillsDir for SKILL.md packages.
func NewSkillFileLoader(skillsDir string, registry *skills.Registry, onChange func()) *SkillFileLoader {
	if skillsDir == "" {
		skillsDir = "data/skills"
	}
	os.MkdirAll(skillsDir, 0755)
	return &SkillFileLoader{
		dir:      skillsDir,
		registry: registry,
		stopCh:   make(chan struct{}),
		onChange: onChange,
		loaded:   make(map[string]time.Time),
	}
}

// LoadAll scans the skills directory and registers all valid SKILL.md packages.
func (l *SkillFileLoader) LoadAll() int {
	l.mu.Lock()
	defer l.mu.Unlock()

	entries, err := os.ReadDir(l.dir)
	if err != nil {
		slog.Error("read skills dir", "err", err, "dir", l.dir)
		return 0
	}

	loaded := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		skillDir := filepath.Join(l.dir, e.Name())
		skillMDPath := filepath.Join(skillDir, "SKILL.md")

		if _, err := os.Stat(skillMDPath); err != nil {
			continue
		}

		sk, err := l.loadSkillFromDir(skillDir, e.Name())
		if err != nil {
			slog.Error("load skill", "dir", skillDir, "err", err)
			continue
		}

		l.registry.Register(sk)
		info, _ := os.Stat(skillMDPath)
		l.loaded[e.Name()] = info.ModTime()
		loaded++
	}

	slog.Info("file-based skills loaded", "count", loaded, "dir", l.dir)
	return loaded
}

// loadSkillFromDir creates a Skill from a directory containing SKILL.md.
func (l *SkillFileLoader) loadSkillFromDir(dir, slug string) (skills.Skill, error) {
	content, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return nil, fmt.Errorf("read SKILL.md: %w", err)
	}

	meta := SkillFileMeta{
		Name:        slug,
		Description: fmt.Sprintf("User-defined skill: %s", slug),
	}

	metaPath := filepath.Join(dir, "meta.json")
	if data, err := os.ReadFile(metaPath); err == nil {
		_ = json.Unmarshal(data, &meta)
	}

	if meta.Name == "" {
		meta.Name = slug
	}

	params := buildParamsSchema(meta.Parameters)

	var scriptFiles []string
	scriptsDir := filepath.Join(dir, "scripts")
	if entries, err := os.ReadDir(scriptsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				scriptFiles = append(scriptFiles, e.Name())
			}
		}
	}

	return &mdSkill{
		slug:        slug,
		name:        meta.Name,
		description: meta.Description,
		params:      params,
		content:     string(content),
		dir:         dir,
		scripts:     scriptFiles,
	}, nil
}

func buildParamsSchema(params map[string]SkillFileParam) map[string]any {
	if len(params) == 0 {
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "The input or query for this skill",
				},
			},
		}
	}

	props := map[string]any{}
	var required []string
	for k, v := range params {
		props[k] = map[string]any{"type": v.Type, "description": v.Description}
		if v.Required {
			required = append(required, k)
		}
	}
	schema := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

// mdSkill wraps a SKILL.md document as an executable Skill.
// When invoked, it returns the SKILL.md content along with any context
// so the LLM can follow the instructions.
type mdSkill struct {
	slug        string
	name        string
	description string
	params      map[string]any
	content     string
	dir         string
	scripts     []string
}

func (s *mdSkill) Name() string        { return s.name }
func (s *mdSkill) Description() string { return s.description }
func (s *mdSkill) Parameters() map[string]any { return s.params }

func (s *mdSkill) Execute(_ context.Context, args map[string]any, _ *skills.Environment) (string, error) {
	var sb strings.Builder
	sb.WriteString("=== SKILL INSTRUCTIONS ===\n")
	sb.WriteString(s.content)
	sb.WriteString("\n=== END INSTRUCTIONS ===\n")

	if query, ok := args["query"].(string); ok && query != "" {
		sb.WriteString("\nUser query: ")
		sb.WriteString(query)
		sb.WriteString("\n")
	}

	if len(s.scripts) > 0 {
		sb.WriteString("\nAvailable helper scripts in this skill package:\n")
		for _, f := range s.scripts {
			sb.WriteString("  - scripts/")
			sb.WriteString(f)
			sb.WriteString("\n")
		}
		sb.WriteString("Use the code_gen or computer_use skill to execute these scripts.\n")
		sb.WriteString("Scripts directory: ")
		sb.WriteString(filepath.Join(s.dir, "scripts"))
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// Watch starts a background goroutine that checks for skill changes.
func (l *SkillFileLoader) Watch(interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-l.stopCh:
				return
			case <-ticker.C:
				if l.hasChanges() {
					slog.Info("skill file changes detected, reloading")
					l.LoadAll()
					if l.onChange != nil {
						l.onChange()
					}
				}
			}
		}
	}()
}

// Stop halts the background watcher.
func (l *SkillFileLoader) Stop() {
	select {
	case l.stopCh <- struct{}{}:
	default:
	}
}

// Dir returns the skills directory path.
func (l *SkillFileLoader) Dir() string { return l.dir }

// hasChanges checks if any skill directories have been added, removed, or modified.
func (l *SkillFileLoader) hasChanges() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	entries, err := os.ReadDir(l.dir)
	if err != nil {
		return false
	}

	seen := make(map[string]bool)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		skillMD := filepath.Join(l.dir, e.Name(), "SKILL.md")
		info, err := os.Stat(skillMD)
		if err != nil {
			continue
		}
		seen[e.Name()] = true
		prev, exists := l.loaded[e.Name()]
		if !exists || !prev.Equal(info.ModTime()) {
			return true
		}
	}

	for slug := range l.loaded {
		if !seen[slug] {
			return true
		}
	}

	return false
}
