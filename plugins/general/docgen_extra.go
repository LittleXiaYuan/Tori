package general

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"yunque-agent/pkg/skills"
)

// ---- HTML export (pure Go, no deps) ----

type HtmlExportSkill struct {
	allowedDirs []string
}

func NewHtmlExportSkill(allowedDirs []string) *HtmlExportSkill {
	return &HtmlExportSkill{allowedDirs: allowedDirs}
}

func (s *HtmlExportSkill) Name() string { return "html_export" }
func (s *HtmlExportSkill) Description() string {
	return "将 Markdown 内容导出为独立 HTML 文件。支持标题、段落、列表、代码块、加粗、斜体。适用于生成网页报告"
}

func (s *HtmlExportSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "输出文件路径（如 data/output/report.html）",
			},
			"title": map[string]any{
				"type":        "string",
				"description": "HTML 页面标题",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "Markdown 格式内容",
			},
		},
		"required": []string{"path", "content"},
	}
}

func (s *HtmlExportSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	path, _ := args["path"].(string)
	title, _ := args["title"].(string)
	content, _ := args["content"].(string)

	if path == "" || content == "" {
		return "", fmt.Errorf("path and content are required")
	}
	if title == "" {
		title = "Document"
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	if !isUnderAllowed(absPath, s.allowedDirs) {
		return "", fmt.Errorf("access denied: path %s is not under allowed directories", path)
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return "", fmt.Errorf("cannot create directory: %w", err)
	}

	html := renderMarkdownToHTML(title, content)
	if err := os.WriteFile(absPath, []byte(html), 0644); err != nil {
		return "", fmt.Errorf("write failed: %w", err)
	}

	info, _ := os.Stat(absPath)
	size := int64(0)
	if info != nil {
		size = info.Size()
	}
	return fmt.Sprintf("已生成 HTML 文件: %s (%d bytes)", path, size), nil
}

// renderMarkdownToHTML: simple subset converter (headings, lists, code blocks, bold/italic).
// Not a full MD parser — just enough for report generation.
func renderMarkdownToHTML(title, md string) string {
	var body strings.Builder
	lines := strings.Split(md, "\n")
	inCodeBlock := false
	inList := false

	for _, line := range lines {
		trimmed := strings.TrimRight(line, "\r ")

		// Code block toggle
		if strings.HasPrefix(trimmed, "```") {
			if inCodeBlock {
				body.WriteString("</code></pre>\n")
				inCodeBlock = false
			} else {
				if inList {
					body.WriteString("</ul>\n")
					inList = false
				}
				body.WriteString("<pre><code>")
				inCodeBlock = true
			}
			continue
		}
		if inCodeBlock {
			body.WriteString(htmlEscapeText(trimmed))
			body.WriteString("\n")
			continue
		}

		// Headings
		if strings.HasPrefix(trimmed, "### ") {
			if inList {
				body.WriteString("</ul>\n")
				inList = false
			}
			body.WriteString("<h3>")
			body.WriteString(htmlEscapeText(trimmed[4:]))
			body.WriteString("</h3>\n")
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			if inList {
				body.WriteString("</ul>\n")
				inList = false
			}
			body.WriteString("<h2>")
			body.WriteString(htmlEscapeText(trimmed[3:]))
			body.WriteString("</h2>\n")
			continue
		}
		if strings.HasPrefix(trimmed, "# ") {
			if inList {
				body.WriteString("</ul>\n")
				inList = false
			}
			body.WriteString("<h1>")
			body.WriteString(htmlEscapeText(trimmed[2:]))
			body.WriteString("</h1>\n")
			continue
		}

		// List items
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			if !inList {
				body.WriteString("<ul>\n")
				inList = true
			}
			body.WriteString("<li>")
			body.WriteString(inlineFormat(trimmed[2:]))
			body.WriteString("</li>\n")
			continue
		}

		// Empty line
		if trimmed == "" {
			if inList {
				body.WriteString("</ul>\n")
				inList = false
			}
			continue
		}

		// Paragraph
		if inList {
			body.WriteString("</ul>\n")
			inList = false
		}
		body.WriteString("<p>")
		body.WriteString(inlineFormat(trimmed))
		body.WriteString("</p>\n")
	}
	if inCodeBlock {
		body.WriteString("</code></pre>\n")
	}
	if inList {
		body.WriteString("</ul>\n")
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="zh">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>%s</title>
<style>
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 800px; margin: 40px auto; padding: 0 20px; line-height: 1.6; color: #333; }
h1,h2,h3 { color: #1a1a1a; }
pre { background: #f5f5f5; padding: 16px; border-radius: 8px; overflow-x: auto; }
code { font-family: 'Consolas', 'Monaco', monospace; font-size: 0.9em; }
ul { padding-left: 24px; }
</style>
</head>
<body>
%s</body>
</html>`, htmlEscapeText(title), body.String())
}

// inlineFormat: **bold** and *italic* only.
func inlineFormat(s string) string {
	escaped := htmlEscapeText(s)
	// Bold: **text**
	result := replaceInlinePairs(escaped, "**", "<strong>", "</strong>")
	// Italic: *text*
	result = replaceInlinePairs(result, "*", "<em>", "</em>")
	return result
}

func replaceInlinePairs(s, delim, open, close string) string {
	var out strings.Builder
	parts := strings.Split(s, delim)
	for i, part := range parts {
		if i%2 == 1 {
			out.WriteString(open)
			out.WriteString(part)
			out.WriteString(close)
		} else {
			out.WriteString(part)
		}
	}
	return out.String()
}

func htmlEscapeText(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

// ---- PPTX generation ----
//
// Dual-engine approach: Python (python-pptx) for best quality,
// Go native OOXML as zero-dependency fallback.

type PptxCreateSkill struct {
	allowedDirs []string
	pythonBin   string // optional: injected Python binary path
}

func NewPptxCreateSkill(allowedDirs []string) *PptxCreateSkill {
	return &PptxCreateSkill{allowedDirs: allowedDirs}
}

// SetPythonBin injects the Python binary path from PythonEnv.
func (s *PptxCreateSkill) SetPythonBin(bin string) {
	s.pythonBin = bin
}

func (s *PptxCreateSkill) Name() string { return "pptx_create" }
func (s *PptxCreateSkill) Description() string {
	return "生成 PowerPoint 演示文稿(.pptx)。每个幻灯片用 --- 分隔，第一行为标题，其余为内容。适用于创建汇报、培训材料"
}

func (s *PptxCreateSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "输出文件路径（如 data/output/slides.pptx）",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "幻灯片内容。每张幻灯片用 --- 分隔，每张第一行为标题，其余行为正文内容",
			},
		},
		"required": []string{"path", "content"},
	}
}

func (s *PptxCreateSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	pathStr, _ := args["path"].(string)
	content, _ := args["content"].(string)

	if pathStr == "" || content == "" {
		return "", fmt.Errorf("path and content are required")
	}

	absPath, err := filepath.Abs(pathStr)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	if !isUnderAllowed(absPath, s.allowedDirs) {
		return "", fmt.Errorf("access denied: path %s is not under allowed directories", pathStr)
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return "", fmt.Errorf("cannot create directory: %w", err)
	}

	slides := parseSlides(content)
	if len(slides) == 0 {
		return "", fmt.Errorf("no slides found (separate slides with ---)")
	}

	pyBin := s.pythonBin
	if pyBin == "" {
		pyBin = findPython()
	}

	var engine string
	if pyBin != "" {
		if err := tryPythonPptx(ctx, pyBin, absPath, slides); err != nil {
			slog.Info("pptx: python-pptx failed, falling back to Go engine", "err", err)
			if err2 := writePptxGo(absPath, slides); err2 != nil {
				return "", fmt.Errorf("pptx generation failed: %w", err2)
			}
			engine = "Go-OOXML(fallback)"
		} else {
			engine = "python-pptx"
		}
	} else {
		if err := writePptxGo(absPath, slides); err != nil {
			return "", fmt.Errorf("pptx generation failed: %w", err)
		}
		engine = "Go-OOXML"
	}

	info, _ := os.Stat(absPath)
	size := int64(0)
	if info != nil {
		size = info.Size()
	}
	return fmt.Sprintf("已生成演示文稿: %s (%d bytes, %d 张幻灯片, engine=%s)", pathStr, size, len(slides), engine), nil
}

func tryPythonPptx(ctx context.Context, pyBin, absPath string, slides []slideData) error {
	tmpDir := os.TempDir()
	jsonPath := filepath.Join(tmpDir, "pptx_data.json")

	jsonBytes, err := json.Marshal(slides)
	if err != nil {
		return err
	}
	if err := os.WriteFile(jsonPath, jsonBytes, 0644); err != nil {
		return err
	}
	defer os.Remove(jsonPath)

	pyPath := filepath.Join(tmpDir, "pptx_renderer.py")
	if err := os.WriteFile(pyPath, []byte(pptxPythonScript), 0644); err != nil {
		return err
	}
	defer os.Remove(pyPath)

	templatePath := filepath.Join("data", "templates", "business.pptx")
	absTemplate, _ := filepath.Abs(templatePath)

	cmd := exec.CommandContext(ctx, pyBin, pyPath, jsonPath, absPath, absTemplate)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %w", string(out), err)
	}
	return nil
}

type slideData struct {
	Title string
	Body  string
}

func parseSlides(content string) []slideData {
	sections := strings.Split(content, "---")
	var slides []slideData
	for _, sec := range sections {
		sec = strings.TrimSpace(sec)
		if sec == "" {
			continue
		}
		lines := strings.SplitN(sec, "\n", 2)
		title := strings.TrimSpace(lines[0])
		// Strip leading # from title
		title = strings.TrimLeft(title, "# ")
		body := ""
		if len(lines) > 1 {
			body = strings.TrimSpace(lines[1])
		}
		slides = append(slides, slideData{Title: title, Body: body})
	}
	return slides
}

const pptxPythonScript = `import sys, json, os
try:
    from pptx import Presentation
except ImportError:
    sys.exit("python-pptx is not installed. Please run: pip install python-pptx")

def main():
    json_path = sys.argv[1]
    out_path = sys.argv[2]
    template_path = sys.argv[3] if len(sys.argv) > 3 else None

    with open(json_path, 'r', encoding='utf-8') as f:
        slides_data = json.load(f)

    if template_path and os.path.exists(template_path):
        prs = Presentation(template_path)
    else:
        prs = Presentation()

    for slide in slides_data:
        layout_idx = 1 if len(prs.slide_layouts) > 1 else 0
        new_slide = prs.slides.add_slide(prs.slide_layouts[layout_idx])
        
        title = slide.get('Title', '')
        body = slide.get('Body', '')
        
        if new_slide.shapes.title:
            new_slide.shapes.title.text = title
            
        try:
            for ph in new_slide.placeholders:
                if ph.placeholder_format.idx == 1:
                    ph.text = body
                    break
        except Exception:
            pass

    prs.save(out_path)

if __name__ == '__main__':
    main()
`
