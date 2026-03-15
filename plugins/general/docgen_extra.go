package general

import (
	"archive/zip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"yunque-agent/pkg/skills"
)

// ──────────────────────────────────────────────
// HtmlExportSkill — convert Markdown content to a standalone HTML file
// Pure Go, zero external dependencies.
// ──────────────────────────────────────────────

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

// renderMarkdownToHTML converts a simple Markdown subset to HTML.
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

// inlineFormat handles **bold** and *italic* in text.
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

// ──────────────────────────────────────────────
// PptxCreateSkill — generate .pptx presentations
// PPTX is Open XML format: a zip file containing XML slides.
// Pure Go, zero external dependencies.
// ──────────────────────────────────────────────

type PptxCreateSkill struct {
	allowedDirs []string
}

func NewPptxCreateSkill(allowedDirs []string) *PptxCreateSkill {
	return &PptxCreateSkill{allowedDirs: allowedDirs}
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
	path, _ := args["path"].(string)
	content, _ := args["content"].(string)

	if path == "" || content == "" {
		return "", fmt.Errorf("path and content are required")
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

	slides := parseSlides(content)
	if len(slides) == 0 {
		return "", fmt.Errorf("no slides found (separate slides with ---)")
	}

	if err := writePptx(absPath, slides); err != nil {
		return "", fmt.Errorf("pptx generation failed: %w", err)
	}

	info, _ := os.Stat(absPath)
	size := int64(0)
	if info != nil {
		size = info.Size()
	}
	return fmt.Sprintf("已生成演示文稿: %s (%d bytes, %d 张幻灯片)", path, size, len(slides)), nil
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

func writePptx(path string, slides []slideData) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	// [Content_Types].xml
	writeZipFile(w, "[Content_Types].xml", pptxContentTypes(len(slides)))

	// _rels/.rels
	writeZipFile(w, "_rels/.rels", pptxRootRels)

	// ppt/presentation.xml
	writeZipFile(w, "ppt/presentation.xml", pptxPresentation(len(slides)))

	// ppt/_rels/presentation.xml.rels
	writeZipFile(w, "ppt/_rels/presentation.xml.rels", pptxPresRels(len(slides)))

	// Each slide
	for i, slide := range slides {
		n := i + 1
		writeZipFile(w, fmt.Sprintf("ppt/slides/slide%d.xml", n), pptxSlideXML(slide))
	}

	return nil
}

func pptxContentTypes(n int) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
`)
	for i := 1; i <= n; i++ {
		sb.WriteString(fmt.Sprintf(`  <Override PartName="/ppt/slides/slide%d.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>
`, i))
	}
	sb.WriteString("</Types>")
	return sb.String()
}

const pptxRootRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>
</Relationships>`

func pptxPresentation(n int) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:presentation xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
                xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
                xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:sldIdLst>
`)
	for i := 1; i <= n; i++ {
		sb.WriteString(fmt.Sprintf(`    <p:sldId id="%d" r:id="rId%d"/>
`, 255+i, i))
	}
	sb.WriteString(`  </p:sldIdLst>
  <p:sldSz cx="9144000" cy="6858000" type="screen4x3"/>
</p:presentation>`)
	return sb.String()
}

func pptxPresRels(n int) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
`)
	for i := 1; i <= n; i++ {
		sb.WriteString(fmt.Sprintf(`  <Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide%d.xml"/>
`, i, i))
	}
	sb.WriteString("</Relationships>")
	return sb.String()
}

func pptxSlideXML(slide slideData) string {
	// EMU: 1 inch = 914400 EMU. Slide is 9144000 x 6858000
	titleEsc := docXMLEscape(slide.Title)
	bodyEsc := docXMLEscape(slide.Body)

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
       xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
       xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld>
    <p:spTree>
      <p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>
      <p:grpSpPr/>
      <p:sp>
        <p:nvSpPr><p:cNvPr id="2" name="Title"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr><p:ph type="title"/></p:nvPr></p:nvSpPr>
        <p:spPr>
          <a:xfrm><a:off x="457200" y="274638"/><a:ext cx="8229600" cy="1143000"/></a:xfrm>
        </p:spPr>
        <p:txBody>
          <a:bodyPr/>
          <a:p><a:r><a:rPr lang="zh-CN" sz="3200" b="1"/><a:t>%s</a:t></a:r></a:p>
        </p:txBody>
      </p:sp>
      <p:sp>
        <p:nvSpPr><p:cNvPr id="3" name="Body"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr><p:ph idx="1"/></p:nvPr></p:nvSpPr>
        <p:spPr>
          <a:xfrm><a:off x="457200" y="1600200"/><a:ext cx="8229600" cy="4525963"/></a:xfrm>
        </p:spPr>
        <p:txBody>
          <a:bodyPr/>
          <a:p><a:r><a:rPr lang="zh-CN" sz="2000"/><a:t>%s</a:t></a:r></a:p>
        </p:txBody>
      </p:sp>
    </p:spTree>
  </p:cSld>
</p:sld>`, titleEsc, bodyEsc)
}
