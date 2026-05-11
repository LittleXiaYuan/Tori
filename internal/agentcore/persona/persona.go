package persona

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Persona defines a bot's identity, personality, and modular skill instructions.
type Persona struct {
	mu       sync.RWMutex
	dataDir  string
	identity string  // loaded from IDENTITY.md
	soul     string  // loaded from SOUL.md
	skills   []Skill // loaded from skills/*.md
}

// DataDir returns the backing persona directory.
func (p *Persona) DataDir() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.dataDir
}

// Skill is a modular instruction block with YAML frontmatter.
type Skill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Content     string `json:"content"`
	Enabled     bool   `json:"enabled"`
}

// New creates a persona loader for the given data directory.
// It immediately loads all files from disk.
func New(dataDir string) (*Persona, error) {
	if dataDir == "" {
		dataDir = "data/persona"
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create persona dir: %w", err)
	}
	// Ensure skills subdirectory
	os.MkdirAll(filepath.Join(dataDir, "skills"), 0o755)

	p := &Persona{dataDir: dataDir}
	p.Reload()

	// Create default files if they don't exist
	p.ensureDefaults()
	return p, nil
}

// Reload reads all persona files from disk.
func (p *Persona) Reload() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.identity = readFileOrEmpty(filepath.Join(p.dataDir, "IDENTITY.md"))
	p.soul = readFileOrEmpty(filepath.Join(p.dataDir, "SOUL.md"))
	p.skills = p.loadSkills()
}

// SystemPrompt builds the complete system prompt incorporating identity, soul, and skills.
func (p *Persona) SystemPrompt() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var parts []string

	if p.identity != "" {
		parts = append(parts, "## 身份\n"+p.identity)
	}
	if p.soul != "" {
		parts = append(parts, "## 性格\n"+p.soul)
	}

	// Collect enabled skills
	var skillParts []string
	for _, s := range p.skills {
		if s.Enabled && s.Content != "" {
			header := fmt.Sprintf("### %s", s.Name)
			if s.Description != "" {
				header += " — " + s.Description
			}
			skillParts = append(skillParts, header+"\n"+s.Content)
		}
	}
	if len(skillParts) > 0 {
		parts = append(parts, "## 技能指令\n"+strings.Join(skillParts, "\n\n"))
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n\n")
}

// Identity returns the raw identity content.
func (p *Persona) Identity() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.identity
}

// SetIdentity updates the identity file.
func (p *Persona) SetIdentity(content string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.identity = content
	return os.WriteFile(filepath.Join(p.dataDir, "IDENTITY.md"), []byte(content), 0o644)
}

// Soul returns the raw soul/personality content.
func (p *Persona) Soul() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.soul
}

// SetSoul updates the soul file.
func (p *Persona) SetSoul(content string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.soul = content
	return os.WriteFile(filepath.Join(p.dataDir, "SOUL.md"), []byte(content), 0o644)
}

// Rename updates the displayed identity and persists it to disk.
func (p *Persona) Rename(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("persona name is required")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	lines := []string{fmt.Sprintf("# %s", name)}
	if p.soul != "" {
		lines = append(lines, "", p.soul)
	}
	content := strings.Join(lines, "\n")
	p.identity = content
	return os.WriteFile(filepath.Join(p.dataDir, "IDENTITY.md"), []byte(content), 0o644)
}

// ReplaceSoulContent replaces the soul/prompt body while preserving the file.
func (p *Persona) ReplaceSoulContent(content string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.soul = content
	return os.WriteFile(filepath.Join(p.dataDir, "SOUL.md"), []byte(content), 0o644)
}

// ReloadPresets is a helper for callers that want a stable reload point after
// switching persona-related files on disk.
func (p *Persona) ReloadPresets() {
	p.Reload()
}

// ResetToDefaults restores the built-in identity and soul templates.
func (p *Persona) ResetToDefaults() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.identity = defaultIdentity
	p.soul = defaultSoul

	if err := os.WriteFile(filepath.Join(p.dataDir, "IDENTITY.md"), []byte(defaultIdentity), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(p.dataDir, "SOUL.md"), []byte(defaultSoul), 0o644); err != nil {
		return err
	}
	return nil
}

// Skills returns all loaded skills.
func (p *Persona) Skills() []Skill {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]Skill, len(p.skills))
	copy(out, p.skills)
	return out
}

// SetSkillEnabled toggles a skill's enabled state.
func (p *Persona) SetSkillEnabled(name string, enabled bool) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.skills {
		if p.skills[i].Name == name {
			p.skills[i].Enabled = enabled
			return true
		}
	}
	return false
}

// AddSkill creates a new skill file.
func (p *Persona) AddSkill(name, description, content string) error {
	name = sanitizeName(name)
	if name == "" {
		return fmt.Errorf("skill name is required")
	}

	raw := buildSkillFile(name, description, content)
	path := filepath.Join(p.dataDir, "skills", name+".md")
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.skills = append(p.skills, Skill{
		Name:        name,
		Description: description,
		Content:     content,
		Enabled:     true,
	})
	return nil
}

// DeleteSkill removes a skill file and unloads it.
func (p *Persona) DeleteSkill(name string) error {
	name = sanitizeName(name)
	path := filepath.Join(p.dataDir, "skills", name+".md")
	os.Remove(path)

	p.mu.Lock()
	defer p.mu.Unlock()
	filtered := p.skills[:0]
	for _, s := range p.skills {
		if s.Name != name {
			filtered = append(filtered, s)
		}
	}
	p.skills = filtered
	return nil
}

func (p *Persona) loadSkills() []Skill {
	dir := filepath.Join(p.dataDir, "skills")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var skills []Skill
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		s := parseSkillFile(string(data), strings.TrimSuffix(e.Name(), ".md"))
		skills = append(skills, s)
	}
	return skills
}

func (p *Persona) ensureDefaults() {
	idPath := filepath.Join(p.dataDir, "IDENTITY.md")
	if _, err := os.Stat(idPath); os.IsNotExist(err) {
		os.WriteFile(idPath, []byte(defaultIdentity), 0o644)
		p.mu.Lock()
		p.identity = defaultIdentity
		p.mu.Unlock()
	}
	soulPath := filepath.Join(p.dataDir, "SOUL.md")
	if _, err := os.Stat(soulPath); os.IsNotExist(err) {
		os.WriteFile(soulPath, []byte(defaultSoul), 0o644)
		p.mu.Lock()
		p.soul = defaultSoul
		p.mu.Unlock()
	}
}

func readFileOrEmpty(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func sanitizeName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "/", "")
	name = strings.ReplaceAll(name, "\\", "")
	name = strings.ReplaceAll(name, "..", "")
	return name
}

func buildSkillFile(name, description, content string) string {
	return fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n%s\n", name, description, content)
}

// parseSkillFile parses a markdown file with optional YAML frontmatter.
func parseSkillFile(raw, fallbackName string) Skill {
	trimmed := strings.TrimSpace(raw)
	s := Skill{Name: fallbackName, Content: trimmed, Enabled: true}

	if !strings.HasPrefix(trimmed, "---") {
		return s
	}

	rest := trimmed[3:]
	if idx := strings.Index(rest, "\n"); idx >= 0 {
		rest = rest[idx+1:]
	}
	closingIdx := strings.Index(rest, "\n---")
	if closingIdx < 0 {
		return s
	}

	frontmatter := rest[:closingIdx]
	body := strings.TrimSpace(rest[closingIdx+4:])
	s.Content = body

	// Simple YAML key-value parsing (no dependency needed)
	for _, line := range strings.Split(frontmatter, "\n") {
		line = strings.TrimSpace(line)
		if parts := strings.SplitN(line, ":", 2); len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			switch key {
			case "name":
				if val != "" {
					s.Name = val
				}
			case "description":
				s.Description = val
			case "enabled":
				s.Enabled = val != "false"
			}
		}
	}
	return s
}

const defaultIdentity = `# Yunque Agent

- **名称:** Yunque
- **类型:** AI 智能助手
- **风格:** 专业、温暖、高效
- **背景:** 多功能AI助手，擅长任务规划和知识管理

你可以在对话中随时修改这个文件来调整自己的身份。`

const defaultSoul = `# 性格特质

- 回答准确，不编造信息
- 善于分步骤解决复杂问题
- 语言简洁但不失温度
- 在不确定时坦诚告知
- 支持中英文交流

# 交互风格

- 先理解用户真正的需求
- 复杂任务先规划再执行
- 主动提供有价值的补充信息
- 避免冗长重复的回答`
