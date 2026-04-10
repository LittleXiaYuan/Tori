package research

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/websearch"
	"yunque-agent/pkg/skills"
)

// BrowserCtrl is the interface for optional browser content extraction.
type BrowserCtrl interface {
	Connected() bool
	SendAction(ctx context.Context, action any) (any, error)
}

type researchSource struct {
	Query   string
	Results []websearch.Result
	Content string
}

// DeepResearchSkill performs multi-round web search and synthesizes a report.
type DeepResearchSkill struct {
	searchReg *websearch.Registry
	llmClient *llm.Client
	browser   BrowserCtrl
	outputDir string
}

func NewDeepResearchSkill(sr *websearch.Registry, lc *llm.Client, bc BrowserCtrl, outDir string) *DeepResearchSkill {
	return &DeepResearchSkill{
		searchReg: sr,
		llmClient: lc,
		browser:   bc,
		outputDir: outDir,
	}
}

func (s *DeepResearchSkill) Name() string { return "deep_research" }
func (s *DeepResearchSkill) Description() string {
	return "Conduct deep research on a topic: perform multiple web searches, read pages, and synthesize a comprehensive report with citations. Output is saved as a Markdown file."
}
func (s *DeepResearchSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"topic": map[string]any{
				"type":        "string",
				"description": "The research topic or question to investigate",
			},
			"depth": map[string]any{
				"type":        "string",
				"description": "Research depth: quick (3 searches), standard (6 searches), deep (10+ searches)",
			},
			"language": map[string]any{
				"type":        "string",
				"description": "Output language: zh (Chinese) or en (English). Default: zh",
			},
		},
		"required": []string{"topic"},
	}
}

func (s *DeepResearchSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	topic, _ := args["topic"].(string)
	if topic == "" {
		return "", fmt.Errorf("topic is required")
	}

	depth, _ := args["depth"].(string)
	if depth == "" {
		depth = "standard"
	}
	lang, _ := args["language"].(string)
	if lang == "" {
		lang = "zh"
	}

	maxSearches := 6
	switch depth {
	case "quick":
		maxSearches = 3
	case "deep":
		maxSearches = 10
	}

	slog.Info("deep research started", "topic", topic, "depth", depth, "max_searches", maxSearches)

	// Phase 1: Generate sub-questions
	subQuestions, err := s.generateSubQuestions(ctx, topic, lang, maxSearches)
	if err != nil {
		return "", fmt.Errorf("failed to generate sub-questions: %w", err)
	}
	slog.Info("research sub-questions generated", "count", len(subQuestions))

	// Phase 2: Search and collect sources
	var sources []researchSource

	for i, q := range subQuestions {
		if i >= maxSearches {
			break
		}

		results, err := s.searchReg.Search(ctx, q, 5)
		if err != nil {
			slog.Warn("search failed for sub-question", "query", q, "err", err)
			continue
		}

		var content string
		if s.browser != nil && s.browser.Connected() && len(results) > 0 {
			content = s.extractBrowserContent(ctx, results[0].URL)
		}

		sources = append(sources, researchSource{
			Query:   q,
			Results: results,
			Content: content,
		})
	}

	if len(sources) == 0 {
		return "No search results found. Please check your search provider configuration.", nil
	}

	// Phase 3: Synthesize report
	report, err := s.synthesizeReport(ctx, topic, sources, lang)
	if err != nil {
		return "", fmt.Errorf("failed to synthesize report: %w", err)
	}

	// Phase 4: Save to file
	filename, err := s.saveReport(topic, report)
	if err != nil {
		return "", fmt.Errorf("failed to save report: %w", err)
	}

	result := fmt.Sprintf("研究报告已生成并保存到: %s\n\n%s", filename, truncate(report, 8000))
	return result, nil
}

func (s *DeepResearchSkill) chatLLM(ctx context.Context, prompt string) (string, error) {
	msgs := []llm.Message{{Role: "user", Content: prompt}}
	return s.llmClient.Chat(ctx, msgs, 0.3)
}

func (s *DeepResearchSkill) generateSubQuestions(ctx context.Context, topic, lang string, count int) ([]string, error) {
	langNote := "用中文回答"
	if lang == "en" {
		langNote = "Answer in English"
	}

	prompt := fmt.Sprintf(
		`You are a research assistant. Given the topic below, generate %d specific search queries that would help thoroughly research this topic. Each query should cover a different aspect.

Topic: %s

%s. Return ONLY the queries, one per line, no numbering.`, count, topic, langNote)

	resp, err := s.chatLLM(ctx, prompt)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(resp), "\n")
	var queries []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimLeft(line, "0123456789.-) ")
		if line != "" {
			queries = append(queries, line)
		}
	}

	if len(queries) == 0 {
		queries = []string{topic}
	}
	return queries, nil
}

func (s *DeepResearchSkill) extractBrowserContent(ctx context.Context, url string) string {
	if s.browser == nil || !s.browser.Connected() {
		return ""
	}

	navAction := map[string]any{"type": "browser_navigate", "url": url}
	_, err := s.browser.SendAction(ctx, navAction)
	if err != nil {
		return ""
	}

	contentAction := map[string]any{"type": "browser_get_content"}
	result, err := s.browser.SendAction(ctx, contentAction)
	if err != nil {
		return ""
	}

	data, _ := json.Marshal(result)
	var parsed struct {
		Content string `json:"content"`
	}
	json.Unmarshal(data, &parsed)
	return truncate(parsed.Content, 5000)
}

func (s *DeepResearchSkill) synthesizeReport(ctx context.Context, topic string, sources []researchSource, lang string) (string, error) {
	langNote := "用中文撰写报告"
	if lang == "en" {
		langNote = "Write the report in English"
	}

	var sb strings.Builder
	for i, src := range sources {
		sb.WriteString(fmt.Sprintf("\n--- Search %d: %s ---\n", i+1, src.Query))
		for _, r := range src.Results {
			sb.WriteString(fmt.Sprintf("- [%s](%s): %s\n", r.Title, r.URL, r.Snippet))
		}
		if src.Content != "" {
			sb.WriteString(fmt.Sprintf("\nPage content:\n%s\n", truncate(src.Content, 3000)))
		}
	}

	prompt := fmt.Sprintf(
		`You are a research analyst. Based on the search results below, write a comprehensive research report on the topic.

Topic: %s

Search Results:
%s

Requirements:
- %s
- Use Markdown formatting with proper headings (##, ###)
- Include an executive summary at the top
- Organize findings into logical sections
- Include citations as [Source Title](URL) links
- Add a "Sources" section at the end listing all referenced URLs
- Be thorough, analytical, and objective
- Minimum 800 words`, topic, sb.String(), langNote)

	return s.chatLLM(ctx, prompt)
}

func (s *DeepResearchSkill) saveReport(topic, content string) (string, error) {
	dir := s.outputDir
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	safeName := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r >= 0x4e00 && r <= 0x9fff {
			return r
		}
		return '_'
	}, topic)
	if len(safeName) > 40 {
		safeName = safeName[:40]
	}

	filename := fmt.Sprintf("research_%s_%s.md", safeName, time.Now().Format("20060102_1504"))
	path := filepath.Join(dir, filename)

	header := fmt.Sprintf("---\ntitle: \"%s\"\ndate: %s\ntype: deep_research\n---\n\n", topic, time.Now().Format(time.RFC3339))
	if err := os.WriteFile(path, []byte(header+content), 0644); err != nil {
		return "", err
	}

	slog.Info("research report saved", "path", path)
	return filename, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n... (truncated)"
}
