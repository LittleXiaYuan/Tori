package general

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"yunque-agent/pkg/skills"
)

// PdfCreateSkill writes a minimal single-page PDF from plain text / Markdown-like lines.
type PdfCreateSkill struct {
	allowedDirs []string
}

func NewPdfCreateSkill(allowedDirs []string) *PdfCreateSkill {
	return &PdfCreateSkill{allowedDirs: allowedDirs}
}

func (s *PdfCreateSkill) Name() string { return "pdf_create" }

func (s *PdfCreateSkill) Description() string {
	return "从纯文本或简单 Markdown（按行）生成基础 PDF 文件，适合清单、简报导出"
}

func (s *PdfCreateSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "输出 .pdf 路径（如 data/output/report.pdf）",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "正文，多行用换行分隔；以 # 开头的行为标题样式",
			},
			"title": map[string]any{
				"type":        "string",
				"description": "文档标题（可选，显示在第一行上方）",
			},
		},
		"required": []string{"path", "content"},
	}
}

func (s *PdfCreateSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	_ = ctx
	_ = env
	path, _ := args["path"].(string)
	content, _ := args["content"].(string)
	title, _ := args["title"].(string)
	if path == "" || content == "" {
		return "", fmt.Errorf("path and content are required")
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if !isUnderAllowed(absPath, s.allowedDirs) {
		return "", fmt.Errorf("access denied: path not under allowed directories")
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return "", err
	}
	lines := formatPDFLines(title, content)
	if err := writeSimplePDF(absPath, lines); err != nil {
		return "", err
	}
	st, _ := os.Stat(absPath)
	sz := int64(0)
	if st != nil {
		sz = st.Size()
	}
	return fmt.Sprintf("已生成 PDF: %s (%d bytes)", path, sz), nil
}

func formatPDFLines(title, body string) []string {
	var lines []string
	if strings.TrimSpace(title) != "" {
		lines = append(lines, "# "+strings.TrimSpace(title))
	}
	for _, ln := range strings.Split(body, "\n") {
		ln = strings.TrimRight(ln, "\r")
		if ln == "" {
			lines = append(lines, " ")
			continue
		}
		lines = append(lines, ln)
	}
	return lines
}

func writeSimplePDF(path string, lines []string) error {
	var stream bytes.Buffer
	stream.WriteString("BT\n/F1 11 Tf\n72 720 Td\n")
	for i, ln := range lines {
		if len(ln) > 120 {
			ln = ln[:120]
		}
		esc := escapePDFLiteral(sanitizePDFLine(ln))
		stream.WriteString(fmt.Sprintf("(%s) Tj\n", esc))
		if i+1 < len(lines) {
			stream.WriteString("0 -14 Td\n")
		}
	}
	stream.WriteString("ET\n")
	streamBytes := stream.Bytes()

	var buf bytes.Buffer
	w := func(s string) { buf.WriteString(s) }
	fmt.Fprintf(&buf, "%%PDF-1.4\n")
	objects := []string{
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		fmt.Sprintf("<< /Length %d >>stream\n%sendstream", len(streamBytes), string(streamBytes)),
		"<< /Type /Page /Parent 4 0 R /MediaBox [0 0 612 792] /Contents 2 0 R /Resources << /Font << /F1 1 0 R >> >> >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Catalog /Pages 4 0 R >>",
	}
	offsets := make([]int, len(objects)+1)
	offsets[0] = 0
	for i, body := range objects {
		offsets[i+1] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", i+1, body)
	}
	startXref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n", len(objects)+1)
	w("0000000000 65535 f \n")
	for i := 1; i <= len(objects); i++ {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offsets[i])
	}
	fmt.Fprintf(&buf, "trailer<< /Size %d /Root 5 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, startXref)
	return os.WriteFile(path, buf.Bytes(), 0644)
}

func escapePDFLiteral(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "(", "\\(")
	s = strings.ReplaceAll(s, ")", "\\)")
	return s
}

func sanitizePDFLine(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "#") {
		s = strings.TrimSpace(strings.TrimPrefix(s, "#"))
	}
	var b strings.Builder
	for _, r := range s {
		if r >= 32 && r < 127 {
			b.WriteRune(r)
		} else if r == '\t' {
			b.WriteRune(' ')
		} else {
			b.WriteRune('?')
		}
	}
	out := b.String()
	if out == "" {
		return " "
	}
	return out
}
