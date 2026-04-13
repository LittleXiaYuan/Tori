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
	return `生成专业 PowerPoint 演示文稿(.pptx)。每张幻灯片用 --- 分隔。
支持：[layout:title|content|image|two_column|section|blank]、[subtitle:文字]、![alt](url) 图片插入、[notes] 演讲备注、template 参数选择模板。
第一张自动识别为标题页，有图片的自动左文右图布局`
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
				"description": "幻灯片内容。每张用 --- 分隔。支持指令：[layout:title|content|image|two_column|section|blank] [subtitle:文字] ![alt](url) [notes]。示例：\n# 标题\n[subtitle:副标题]\n正文内容\n![logo](https://example.com/img.png)\n[notes]\n演讲备注",
			},
			"template": map[string]any{
				"type":        "string",
				"description": "模板名称（如 business, education, creative, minimal）。留空使用默认模板",
			},
		},
		"required": []string{"path", "content"},
	}
}

func (s *PptxCreateSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	pathStr, _ := args["path"].(string)
	content, _ := args["content"].(string)
	templateName, _ := args["template"].(string)

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

	templatePath := resolveTemplate(templateName)

	pyBin := s.pythonBin
	if pyBin == "" {
		pyBin = findPython()
	}

	var engine string
	if pyBin != "" {
		if err := tryPythonPptx(ctx, pyBin, absPath, slides, templatePath); err != nil {
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
	tmplInfo := ""
	if templatePath != "" {
		tmplInfo = ", template=" + filepath.Base(templatePath)
	}
	return fmt.Sprintf("已生成演示文稿: %s (%d bytes, %d 张幻灯片, engine=%s%s)", pathStr, size, len(slides), engine, tmplInfo), nil
}

func tryPythonPptx(ctx context.Context, pyBin, absPath string, slides []slideData, templatePath string) error {
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

	absTemplate := ""
	if templatePath != "" {
		absTemplate, _ = filepath.Abs(templatePath)
	}

	cmd := exec.CommandContext(ctx, pyBin, pyPath, jsonPath, absPath, absTemplate)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %w", string(out), err)
	}
	return nil
}

type slideImage struct {
	URL   string  `json:"url"`
	Left  float64 `json:"left"`
	Top   float64 `json:"top"`
	Width float64 `json:"width"`
}

type slideData struct {
	Title    string       `json:"Title"`
	Subtitle string       `json:"Subtitle,omitempty"`
	Body     string       `json:"Body"`
	Layout   string       `json:"Layout,omitempty"` // title, content, image, two_column, section, blank
	Images   []slideImage `json:"Images,omitempty"`
	Notes    string       `json:"Notes,omitempty"`
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
		title = strings.TrimLeft(title, "# ")
		body := ""
		if len(lines) > 1 {
			body = strings.TrimSpace(lines[1])
		}

		var images []slideImage
		var cleanLines []string
		layout := ""
		subtitle := ""
		notes := ""
		inNotes := false

		for _, line := range strings.Split(body, "\n") {
			trimmed := strings.TrimSpace(line)

			if strings.HasPrefix(trimmed, "[layout:") && strings.HasSuffix(trimmed, "]") {
				layout = strings.TrimSuffix(strings.TrimPrefix(trimmed, "[layout:"), "]")
				layout = strings.TrimSpace(layout)
				continue
			}

			if strings.HasPrefix(trimmed, "[subtitle:") && strings.HasSuffix(trimmed, "]") {
				subtitle = strings.TrimSuffix(strings.TrimPrefix(trimmed, "[subtitle:"), "]")
				subtitle = strings.TrimSpace(subtitle)
				continue
			}

			if strings.EqualFold(trimmed, "[notes]") {
				inNotes = true
				continue
			}
			if inNotes {
				if notes != "" {
					notes += "\n"
				}
				notes += line
				continue
			}

			if strings.HasPrefix(trimmed, "![") {
				if start := strings.Index(trimmed, "]("); start >= 0 {
					if end := strings.Index(trimmed[start+2:], ")"); end >= 0 {
						url := trimmed[start+2 : start+2+end]
						if url != "" {
							images = append(images, slideImage{URL: url, Width: 4.0})
						}
						continue
					}
				}
			}
			cleanLines = append(cleanLines, line)
		}
		body = strings.TrimSpace(strings.Join(cleanLines, "\n"))

		if layout == "" {
			layout = inferLayout(title, body, images, len(slides))
		}

		slides = append(slides, slideData{
			Title:    title,
			Subtitle: subtitle,
			Body:     body,
			Layout:   layout,
			Images:   images,
			Notes:    strings.TrimSpace(notes),
		})
	}
	return slides
}

func inferLayout(title, body string, images []slideImage, idx int) string {
	if idx == 0 && (body == "" || len(body) < 50) {
		return "title"
	}
	if len(images) > 0 && body != "" {
		return "image"
	}
	if len(images) > 0 && body == "" {
		return "image"
	}
	if body == "" {
		return "section"
	}
	return "content"
}

const pptxPythonScript = `import sys, json, os, tempfile
try:
    from pptx import Presentation
    from pptx.util import Inches, Pt, Emu
    from pptx.dml.color import RGBColor
    from pptx.enum.text import PP_ALIGN, MSO_ANCHOR
    from pptx.enum.shapes import MSO_SHAPE
except ImportError:
    sys.exit("python-pptx is not installed. Please run: pip install python-pptx")

SLIDE_W = Inches(13.333)
SLIDE_H = Inches(7.5)
MARGIN = Inches(0.6)

COLOR_PRIMARY   = RGBColor(0x1A, 0x1A, 0x2E)
COLOR_ACCENT    = RGBColor(0x00, 0x6D, 0x77)
COLOR_LIGHT_BG  = RGBColor(0xF0, 0xF4, 0xF8)
COLOR_TEXT       = RGBColor(0x2D, 0x3A, 0x4A)
COLOR_SUBTLE     = RGBColor(0x6B, 0x7B, 0x8D)
COLOR_WHITE     = RGBColor(0xFF, 0xFF, 0xFF)

def set_font(run, size=18, bold=False, color=None, name=None):
    run.font.size = Pt(size)
    run.font.bold = bold
    if color:
        run.font.color.rgb = color
    if name:
        run.font.name = name

def add_rect(slide, left, top, width, height, fill_color):
    shape = slide.shapes.add_shape(MSO_SHAPE.RECTANGLE, left, top, width, height)
    shape.fill.solid()
    shape.fill.fore_color.rgb = fill_color
    shape.line.fill.background()
    return shape

def add_text_box(slide, left, top, width, height, text, size=18, bold=False, color=None, align=PP_ALIGN.LEFT, name=None):
    txBox = slide.shapes.add_textbox(left, top, width, height)
    tf = txBox.text_frame
    tf.word_wrap = True
    p = tf.paragraphs[0]
    p.alignment = align
    run = p.add_run()
    run.text = text
    set_font(run, size, bold, color, name)
    return txBox

def fetch_image(url):
    if os.path.isfile(url):
        return url, False
    import urllib.request
    ext = os.path.splitext(url)[-1].split('?')[0] or '.png'
    tmp = tempfile.NamedTemporaryFile(suffix=ext, delete=False)
    try:
        req = urllib.request.Request(url, headers={'User-Agent': 'Mozilla/5.0'})
        with urllib.request.urlopen(req, timeout=30) as resp:
            tmp.write(resp.read())
        tmp.close()
        return tmp.name, True
    except Exception as e:
        tmp.close()
        os.unlink(tmp.name)
        raise RuntimeError(f"Cannot download image {url}: {e}")

def add_images_to_slide(slide, images, region_left, region_top, region_w, region_h):
    if not images:
        return
    n = len(images)
    for i, img in enumerate(images):
        url = img.get('url', '')
        if not url:
            continue
        img_left = region_left if img.get('left', 0) <= 0 else Inches(img['left'])
        img_top  = region_top + int(region_h * i / max(n, 1)) if img.get('top', 0) <= 0 else Inches(img['top'])
        img_w    = Inches(img.get('width', 0)) or region_w
        try:
            path, is_tmp = fetch_image(url)
            slide.shapes.add_picture(path, img_left, img_top, width=img_w)
            if is_tmp:
                os.unlink(path)
        except Exception as e:
            print(f"Warning: skipping image {url}: {e}", file=sys.stderr)

def build_body_paragraphs(tf, body, text_color=None):
    lines = body.split('\n')
    first = True
    for line in lines:
        stripped = line.strip()
        if not stripped:
            continue
        if first:
            p = tf.paragraphs[0]
            first = False
        else:
            p = tf.add_paragraph()

        is_bullet = stripped.startswith(('- ', '* ', '• '))
        if is_bullet:
            stripped = stripped.lstrip('-*• ').strip()
            p.level = 0
            p.space_before = Pt(4)

        if stripped.startswith('**') and stripped.endswith('**'):
            run = p.add_run()
            run.text = stripped[2:-2]
            set_font(run, 16, bold=True, color=text_color or COLOR_TEXT)
        else:
            run = p.add_run()
            run.text = stripped
            set_font(run, 16, color=text_color or COLOR_TEXT)

def layout_title_slide(slide, data):
    add_rect(slide, 0, 0, SLIDE_W, SLIDE_H, COLOR_PRIMARY)
    add_rect(slide, 0, SLIDE_H - Inches(0.15), SLIDE_W, Inches(0.15), COLOR_ACCENT)

    title = data.get('Title', '')
    subtitle = data.get('Subtitle', '')

    title_top = Inches(2.2) if subtitle else Inches(2.8)
    add_text_box(slide, MARGIN, title_top, SLIDE_W - 2*MARGIN, Inches(1.5),
                 title, size=40, bold=True, color=COLOR_WHITE, align=PP_ALIGN.CENTER)

    if subtitle:
        add_text_box(slide, MARGIN, title_top + Inches(1.6), SLIDE_W - 2*MARGIN, Inches(0.8),
                     subtitle, size=20, color=COLOR_SUBTLE, align=PP_ALIGN.CENTER)

    images = data.get('Images', [])
    if images:
        add_images_to_slide(slide, images, Inches(4.5), Inches(4.5), Inches(4), Inches(2))

def layout_section_slide(slide, data):
    add_rect(slide, 0, 0, SLIDE_W, SLIDE_H, COLOR_ACCENT)
    add_text_box(slide, MARGIN, Inches(2.5), SLIDE_W - 2*MARGIN, Inches(2),
                 data.get('Title', ''), size=36, bold=True, color=COLOR_WHITE, align=PP_ALIGN.CENTER)
    if data.get('Subtitle'):
        add_text_box(slide, MARGIN, Inches(4.2), SLIDE_W - 2*MARGIN, Inches(1),
                     data['Subtitle'], size=18, color=RGBColor(0xCC, 0xE8, 0xEB), align=PP_ALIGN.CENTER)

def layout_content_slide(slide, data):
    add_rect(slide, 0, 0, SLIDE_W, Inches(1.3), COLOR_PRIMARY)
    add_text_box(slide, MARGIN, Inches(0.25), SLIDE_W - 2*MARGIN, Inches(0.8),
                 data.get('Title', ''), size=28, bold=True, color=COLOR_WHITE)
    add_rect(slide, MARGIN, Inches(1.3), Inches(2), Inches(0.06), COLOR_ACCENT)

    body_top = Inches(1.7)
    body_w = SLIDE_W - 2*MARGIN
    body_h = SLIDE_H - body_top - MARGIN

    body = data.get('Body', '')
    if body:
        txBox = slide.shapes.add_textbox(MARGIN, body_top, body_w, body_h)
        tf = txBox.text_frame
        tf.word_wrap = True
        build_body_paragraphs(tf, body)

def layout_image_slide(slide, data):
    add_rect(slide, 0, 0, SLIDE_W, Inches(1.3), COLOR_PRIMARY)
    add_text_box(slide, MARGIN, Inches(0.25), SLIDE_W - 2*MARGIN, Inches(0.8),
                 data.get('Title', ''), size=28, bold=True, color=COLOR_WHITE)
    add_rect(slide, MARGIN, Inches(1.3), Inches(2), Inches(0.06), COLOR_ACCENT)

    images = data.get('Images', [])
    body = data.get('Body', '')
    has_body = bool(body.strip())

    if has_body and images:
        text_w = Inches(6)
        txBox = slide.shapes.add_textbox(MARGIN, Inches(1.7), text_w, Inches(5))
        tf = txBox.text_frame
        tf.word_wrap = True
        build_body_paragraphs(tf, body)

        img_left = MARGIN + text_w + Inches(0.4)
        img_w = SLIDE_W - img_left - MARGIN
        add_images_to_slide(slide, images, img_left, Inches(1.7), img_w, Inches(5))
    elif images:
        n = len(images)
        region_w = SLIDE_W - 2*MARGIN
        add_images_to_slide(slide, images, MARGIN, Inches(1.7), region_w, Inches(5.2))
    elif has_body:
        layout_content_slide.__wrapped__(slide, data) if hasattr(layout_content_slide, '__wrapped__') else None

def layout_two_column(slide, data):
    add_rect(slide, 0, 0, SLIDE_W, Inches(1.3), COLOR_PRIMARY)
    add_text_box(slide, MARGIN, Inches(0.25), SLIDE_W - 2*MARGIN, Inches(0.8),
                 data.get('Title', ''), size=28, bold=True, color=COLOR_WHITE)
    add_rect(slide, MARGIN, Inches(1.3), Inches(2), Inches(0.06), COLOR_ACCENT)

    body = data.get('Body', '')
    parts = body.split('\n\n', 1)
    col_w = (SLIDE_W - 2*MARGIN - Inches(0.5)) / 2
    col_top = Inches(1.7)
    col_h = SLIDE_H - col_top - MARGIN

    for i, part in enumerate(parts[:2]):
        left = MARGIN + i * (col_w + Inches(0.5))
        txBox = slide.shapes.add_textbox(left, col_top, int(col_w), col_h)
        tf = txBox.text_frame
        tf.word_wrap = True
        build_body_paragraphs(tf, part.strip())

    images = data.get('Images', [])
    if images:
        add_images_to_slide(slide, images, MARGIN, Inches(5), SLIDE_W - 2*MARGIN, Inches(2))

def layout_blank_slide(slide, data):
    images = data.get('Images', [])
    if images:
        add_images_to_slide(slide, images, MARGIN, MARGIN, SLIDE_W - 2*MARGIN, SLIDE_H - 2*MARGIN)

LAYOUT_MAP = {
    'title':      layout_title_slide,
    'section':    layout_section_slide,
    'content':    layout_content_slide,
    'image':      layout_image_slide,
    'two_column': layout_two_column,
    'blank':      layout_blank_slide,
}

def render_slide(prs, data):
    blank_layout = prs.slide_layouts[6] if len(prs.slide_layouts) > 6 else prs.slide_layouts[0]
    slide = prs.slides.add_slide(blank_layout)

    layout_name = data.get('Layout', 'content')
    renderer = LAYOUT_MAP.get(layout_name, layout_content_slide)
    renderer(slide, data)

    notes_text = data.get('Notes', '')
    if notes_text:
        notes_slide = slide.notes_slide
        notes_slide.notes_text_frame.text = notes_text

    return slide

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
        prs.slide_width = SLIDE_W
        prs.slide_height = SLIDE_H

    for data in slides_data:
        render_slide(prs, data)

    prs.save(out_path)

if __name__ == '__main__':
    main()
`
