package planner

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// WebSearchResult is a single web search result used by the skill generator.
type WebSearchResult struct {
	Title   string
	URL     string
	Snippet string
}

// WebSearchFunc searches the web and returns results.
type WebSearchFunc func(ctx context.Context, query string, limit int) ([]WebSearchResult, error)

// SkillRegisterFunc registers a generated skill and returns its registered name.
type SkillRegisterFunc func(slug, name, description, content string) (string, error)

// SkillFile represents one file in a generated skill package.
type SkillFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// SkillPackageRegisterFunc registers a multi-file skill package. Returns registered name.
type SkillPackageRegisterFunc func(slug, name, description string, files []SkillFile) (string, error)

// SkillGenerator creates new skills by searching authoritative web sources
// and using the LLM to synthesize a skill package (SKILL.md + optional scripts/templates).
type SkillGenerator struct {
	webSearch       WebSearchFunc
	llmCall         func(ctx context.Context, system, user string) (string, error)
	register        SkillRegisterFunc
	registerPackage SkillPackageRegisterFunc
	timeout         time.Duration
}

func NewSkillGenerator(timeout time.Duration) *SkillGenerator {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &SkillGenerator{timeout: timeout}
}

func (g *SkillGenerator) SetWebSearch(fn WebSearchFunc) { g.webSearch = fn }
func (g *SkillGenerator) SetLLMCall(fn func(ctx context.Context, system, user string) (string, error)) {
	g.llmCall = fn
}
func (g *SkillGenerator) SetRegister(fn SkillRegisterFunc)               { g.register = fn }
func (g *SkillGenerator) SetRegisterPackage(fn SkillPackageRegisterFunc) { g.registerPackage = fn }

func (g *SkillGenerator) Ready() bool {
	return g.webSearch != nil && g.llmCall != nil && (g.register != nil || g.registerPackage != nil)
}

// Generate searches the web for authoritative documentation on the requested capability,
// synthesizes a skill package via the LLM, and registers it. Returns the registered skill name.
func (g *SkillGenerator) Generate(ctx context.Context, capabilityDesc string, failureContext string) (string, error) {
	if !g.Ready() {
		return "", fmt.Errorf("skill generator not fully configured")
	}

	genCtx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	query := capabilityDesc + " official documentation API usage"
	results, err := g.webSearch(genCtx, query, 5)
	if err != nil {
		return "", fmt.Errorf("web search failed: %w", err)
	}
	if len(results) == 0 {
		return "", fmt.Errorf("no web results for %q", capabilityDesc)
	}

	var sources strings.Builder
	for i, r := range results {
		fmt.Fprintf(&sources, "Source %d: %s\nURL: %s\nContent: %s\n\n", i+1, r.Title, r.URL, r.Snippet)
	}

	reply, err := g.llmCall(genCtx, skillGenPackagePrompt, fmt.Sprintf(
		"Capability needed: %s\n\nFailure context: %s\n\nAuthoritative sources:\n%s\nGenerate a skill package based on these sources.",
		capabilityDesc, failureContext, sources.String(),
	))
	if err != nil {
		return "", fmt.Errorf("LLM generation failed: %w", err)
	}

	if strings.Contains(reply, "INSUFFICIENT_SOURCES") {
		return "", fmt.Errorf("insufficient authoritative sources for %q", capabilityDesc)
	}

	pkg, err := parseSkillPackage(reply)
	if err != nil {
		return "", err
	}

	// Multi-file registration if available, otherwise fall back to single SKILL.md
	if g.registerPackage != nil && len(pkg.Files) > 0 {
		registeredName, regErr := g.registerPackage(pkg.Slug, pkg.Name, pkg.Desc, pkg.Files)
		if regErr != nil {
			return "", fmt.Errorf("register skill package: %w", regErr)
		}
		slog.Info("skill_generator: created multi-file skill",
			"slug", pkg.Slug, "name", pkg.Name, "files", len(pkg.Files), "sources_used", len(results))
		return registeredName, nil
	}

	// Fallback: extract SKILL.md content for single-file registration
	skillMD := ""
	for _, f := range pkg.Files {
		if f.Path == "SKILL.md" {
			skillMD = f.Content
			break
		}
	}
	if skillMD == "" && len(pkg.Files) > 0 {
		skillMD = pkg.Files[0].Content
	}
	if skillMD == "" {
		return "", fmt.Errorf("no SKILL.md content generated")
	}

	registeredName, err := g.register(pkg.Slug, pkg.Name, pkg.Desc, skillMD)
	if err != nil {
		return "", fmt.Errorf("register generated skill: %w", err)
	}

	slog.Info("skill_generator: created skill from web sources",
		"slug", pkg.Slug, "name", pkg.Name, "sources_used", len(results))
	return registeredName, nil
}

type skillPackage struct {
	Slug  string
	Name  string
	Desc  string
	Files []SkillFile
}

func parseSkillPackage(reply string) (*skillPackage, error) {
	pkg := &skillPackage{}

	for _, line := range strings.Split(reply, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "SKILL_NAME:"):
			pkg.Slug = strings.TrimSpace(line[len("SKILL_NAME:"):])
		case strings.HasPrefix(line, "SKILL_DISPLAY:"):
			pkg.Name = strings.TrimSpace(line[len("SKILL_DISPLAY:"):])
		case strings.HasPrefix(line, "SKILL_DESC:"):
			pkg.Desc = strings.TrimSpace(line[len("SKILL_DESC:"):])
		}
	}

	// Parse multi-file blocks: ---FILE: path/to/file--- ... ---END_FILE---
	remaining := reply
	for {
		fileStart := strings.Index(remaining, "---FILE:")
		if fileStart < 0 {
			break
		}
		headerEnd := strings.Index(remaining[fileStart:], "---\n")
		if headerEnd < 0 {
			headerEnd = strings.Index(remaining[fileStart:], "---\r\n")
		}
		if headerEnd < 0 {
			break
		}
		filePath := strings.TrimSpace(remaining[fileStart+len("---FILE:") : fileStart+headerEnd])

		contentStart := fileStart + headerEnd + 4 // skip "---\n"
		fileEnd := strings.Index(remaining[contentStart:], "---END_FILE---")
		if fileEnd < 0 {
			break
		}
		content := strings.TrimSpace(remaining[contentStart : contentStart+fileEnd])
		pkg.Files = append(pkg.Files, SkillFile{Path: filePath, Content: content})
		remaining = remaining[contentStart+fileEnd+len("---END_FILE---"):]
	}

	// Fallback: old single-content format
	if len(pkg.Files) == 0 {
		if start := strings.Index(reply, "---SKILL_CONTENT---"); start >= 0 {
			if end := strings.Index(reply, "---END_SKILL---"); end > start {
				content := strings.TrimSpace(reply[start+len("---SKILL_CONTENT---") : end])
				pkg.Files = append(pkg.Files, SkillFile{Path: "SKILL.md", Content: content})
			}
		}
	}

	if pkg.Slug == "" || pkg.Name == "" || len(pkg.Files) == 0 {
		return nil, fmt.Errorf("incomplete skill definition: slug=%q name=%q files=%d", pkg.Slug, pkg.Name, len(pkg.Files))
	}
	if pkg.Desc == "" {
		pkg.Desc = pkg.Name
	}
	return pkg, nil
}

const skillGenPackagePrompt = `You are a skill generator for an AI agent platform.
Given web search results about a capability, generate a complete skill package.

A skill package contains multiple files:
1. SKILL.md (required) — clear step-by-step instructions the agent follows
2. scripts/ (optional) — helper scripts (Python, Shell, etc.) for automation
3. templates/ (optional) — template files (JSON, Markdown, etc.) for structured output
4. references/ (optional) — format specifications, guidelines, or schemas

Requirements:
- Base your instructions ONLY on the provided authoritative sources — do NOT hallucinate
- The SKILL.md should reference the other files in the package when relevant
- Scripts should be production-ready with error handling
- If the sources are insufficient, respond with exactly: INSUFFICIENT_SOURCES

Output format:
SKILL_NAME: <short-identifier-with-hyphens>
SKILL_DISPLAY: <Human Readable Name>
SKILL_DESC: <one-line description>
---FILE: SKILL.md---
<the main SKILL.md content>
---END_FILE---
---FILE: scripts/helper.py---
<optional script content>
---END_FILE---
---FILE: templates/output.md---
<optional template>
---END_FILE---

You may include as many ---FILE: ...--- blocks as needed. At minimum, generate SKILL.md.`
