package general

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"yunque-agent/pkg/skills"
)

type pptxTemplate struct {
	Name string
	URL  string
	Tags string
}

var builtinTemplates = []pptxTemplate{
	{Name: "business", URL: "https://github.com/yunque-agent/pptx-templates/releases/download/v1/business.pptx", Tags: "business,report,work,corporate,formal"},
	{Name: "education", URL: "https://github.com/yunque-agent/pptx-templates/releases/download/v1/education.pptx", Tags: "education,school,teaching,academic,lecture"},
	{Name: "creative", URL: "https://github.com/yunque-agent/pptx-templates/releases/download/v1/creative.pptx", Tags: "creative,design,art,portfolio,colorful"},
	{Name: "minimal", URL: "https://github.com/yunque-agent/pptx-templates/releases/download/v1/minimal.pptx", Tags: "minimal,simple,clean,modern,basic"},
}

func templatesDir() string {
	dir, _ := filepath.Abs(filepath.Join("data", "templates"))
	return dir
}

func ListTemplates() []string {
	dir := templatesDir()
	var names []string

	for _, t := range builtinTemplates {
		local := filepath.Join(dir, t.Name+".pptx")
		status := "not-downloaded"
		if _, err := os.Stat(local); err == nil {
			status = "available"
		}
		names = append(names, fmt.Sprintf("%s (%s)", t.Name, status))
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return names
	}
	known := make(map[string]bool)
	for _, t := range builtinTemplates {
		known[t.Name+".pptx"] = true
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".pptx") && !known[e.Name()] {
			names = append(names, strings.TrimSuffix(e.Name(), ".pptx")+" (custom)")
		}
	}
	return names
}

func downloadFile(url, dest string) error {
	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "yunque-agent/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download: HTTP %d", resp.StatusCode)
	}

	dir := filepath.Dir(dest)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, "dl-*.pptx")
	if err != nil {
		return err
	}
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	tmp.Close()
	if err := os.Rename(tmp.Name(), dest); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}

func DownloadTemplate(name string) (string, error) {
	var tpl *pptxTemplate
	for i := range builtinTemplates {
		if builtinTemplates[i].Name == name {
			tpl = &builtinTemplates[i]
			break
		}
	}
	if tpl == nil {
		return "", fmt.Errorf("unknown builtin template: %s", name)
	}
	if tpl.URL == "" {
		return "", fmt.Errorf("template %s has no remote URL", name)
	}

	dir := templatesDir()
	dest := filepath.Join(dir, name+".pptx")
	if info, err := os.Stat(dest); err == nil && info.Size() > 0 {
		return dest, nil
	}

	slog.Info("pptx_templates: downloading builtin", "name", name, "url", tpl.URL)
	if err := downloadFile(tpl.URL, dest); err != nil {
		return "", err
	}
	slog.Info("pptx_templates: downloaded", "name", name, "path", dest)
	return dest, nil
}

func SelectTemplate(hint string) string {
	if hint == "" {
		return ""
	}
	hint = strings.ToLower(hint)

	for _, t := range builtinTemplates {
		if strings.EqualFold(t.Name, hint) {
			return t.Name
		}
	}

	bestName := ""
	bestScore := 0
	for _, t := range builtinTemplates {
		score := 0
		for _, tag := range strings.Split(t.Tags, ",") {
			if strings.Contains(hint, tag) {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			bestName = t.Name
		}
	}
	return bestName
}

func resolveTemplate(templateName string) string {
	if templateName == "" {
		fallback := filepath.Join("data", "templates", "business.pptx")
		abs, _ := filepath.Abs(fallback)
		if _, err := os.Stat(abs); err == nil {
			return abs
		}
		return ""
	}

	selected := SelectTemplate(templateName)
	if selected == "" {
		selected = templateName
	}

	dir := templatesDir()
	local := filepath.Join(dir, selected+".pptx")
	if _, err := os.Stat(local); err == nil {
		return local
	}

	if path, err := DownloadTemplate(selected); err == nil {
		return path
	} else {
		slog.Warn("pptx_templates: download failed, using blank", "template", selected, "err", err)
	}
	return ""
}

// --- GitHub PPTX template search ---

type ghSearchResult struct {
	Items []ghSearchItem `json:"items"`
}

type ghSearchItem struct {
	Name       string `json:"name"`
	HTMLURL    string `json:"html_url"`
	DownloadURL string `json:"download_url"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

func searchGitHubTemplates(ctx context.Context, keyword string, maxResults int) ([]ghSearchItem, error) {
	query := fmt.Sprintf("extension:pptx %s", keyword)
	url := fmt.Sprintf("https://api.github.com/search/code?q=%s&per_page=%d",
		strings.ReplaceAll(query, " ", "+"), maxResults)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "yunque-agent/1.0")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == 429 {
		return nil, fmt.Errorf("github API rate limited (HTTP %d)", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github search: HTTP %d", resp.StatusCode)
	}

	var result ghSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("github search decode: %w", err)
	}

	var pptxItems []ghSearchItem
	for _, item := range result.Items {
		if strings.HasSuffix(strings.ToLower(item.Name), ".pptx") {
			pptxItems = append(pptxItems, item)
		}
	}
	return pptxItems, nil
}

func downloadGitHubTemplate(ctx context.Context, item ghSearchItem) (string, error) {
	if item.DownloadURL == "" {
		return "", fmt.Errorf("no download URL for %s", item.Name)
	}

	safeName := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			return '_'
		}
		return r
	}, item.Name)
	if !strings.HasSuffix(strings.ToLower(safeName), ".pptx") {
		safeName += ".pptx"
	}

	dir := templatesDir()
	dest := filepath.Join(dir, safeName)
	if info, err := os.Stat(dest); err == nil && info.Size() > 0 {
		return dest, nil
	}

	slog.Info("pptx_templates: downloading from GitHub", "name", item.Name, "repo", item.Repository.FullName)
	if err := downloadFile(item.DownloadURL, dest); err != nil {
		return "", err
	}
	return dest, nil
}

// --- PptxTemplateSearchSkill: LLM-callable skill ---

type PptxTemplateSearchSkill struct {
	allowedDirs []string
}

func NewPptxTemplateSearchSkill(allowedDirs []string) *PptxTemplateSearchSkill {
	return &PptxTemplateSearchSkill{allowedDirs: allowedDirs}
}

func (s *PptxTemplateSearchSkill) Name() string { return "pptx_template_search" }
func (s *PptxTemplateSearchSkill) Description() string {
	return "搜索并下载 PowerPoint 模板。支持列出本地模板、按关键词在 GitHub 搜索在线模板并下载。用于在生成 PPT 前选择合适的模板风格"
}

func (s *PptxTemplateSearchSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"list", "search", "download"},
				"description": "操作类型：list=列出本地模板, search=搜索在线模板, download=下载指定模板",
			},
			"keyword": map[string]any{
				"type":        "string",
				"description": "搜索关键词（action=search 时必填），如 business, education, technology",
			},
			"url": map[string]any{
				"type":        "string",
				"description": "模板下载URL（action=download 时可用），直接指定下载地址",
			},
			"name": map[string]any{
				"type":        "string",
				"description": "模板名称（action=download 时可用），内置模板名或保存文件名",
			},
		},
		"required": []string{"action"},
	}
}

func (s *PptxTemplateSearchSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	action, _ := args["action"].(string)

	switch action {
	case "list":
		templates := ListTemplates()
		if len(templates) == 0 {
			return "暂无本地模板。可使用 action=search 搜索在线模板", nil
		}
		return fmt.Sprintf("可用模板 (%d):\n- %s", len(templates), strings.Join(templates, "\n- ")), nil

	case "search":
		keyword, _ := args["keyword"].(string)
		if keyword == "" {
			keyword = "template presentation"
		}
		items, err := searchGitHubTemplates(ctx, keyword, 10)
		if err != nil {
			return "", fmt.Errorf("搜索失败: %w", err)
		}
		if len(items) == 0 {
			return fmt.Sprintf("未找到与 '%s' 相关的 PPTX 模板。建议更换关键词重试", keyword), nil
		}

		var lines []string
		for i, item := range items {
			lines = append(lines, fmt.Sprintf("%d. %s (repo: %s)\n   下载: %s",
				i+1, item.Name, item.Repository.FullName, item.DownloadURL))
		}
		return fmt.Sprintf("找到 %d 个模板:\n%s\n\n使用 action=download, url=<下载链接> 下载模板",
			len(items), strings.Join(lines, "\n")), nil

	case "download":
		name, _ := args["name"].(string)
		url, _ := args["url"].(string)

		if url != "" {
			if name == "" {
				parts := strings.Split(url, "/")
				name = parts[len(parts)-1]
			}
			if !strings.HasSuffix(strings.ToLower(name), ".pptx") {
				name += ".pptx"
			}
			dest := filepath.Join(templatesDir(), name)
			if err := downloadFile(url, dest); err != nil {
				return "", fmt.Errorf("下载失败: %w", err)
			}
			return fmt.Sprintf("模板已下载: %s\n路径: %s", name, dest), nil
		}

		if name != "" {
			path, err := DownloadTemplate(name)
			if err != nil {
				return "", fmt.Errorf("下载失败: %w", err)
			}
			return fmt.Sprintf("模板已下载: %s\n路径: %s", name, path), nil
		}

		return "", fmt.Errorf("download 需要 url 或 name 参数")

	default:
		return "", fmt.Errorf("未知操作: %s (支持 list, search, download)", action)
	}
}
