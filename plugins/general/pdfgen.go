package general

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"yunque-agent/pkg/skills"
)

// PdfCreateSkill generates PDF from text.
// ASCII-only → fast pure-Go path (raw PDF 1.4 objects).
// CJK/Unicode → shells out to Python+reportlab for proper font embedding.
type PdfCreateSkill struct {
	allowedDirs []string
}

func NewPdfCreateSkill(allowedDirs []string) *PdfCreateSkill {
	return &PdfCreateSkill{allowedDirs: allowedDirs}
}

func (s *PdfCreateSkill) Name() string { return "pdf_create" }

func (s *PdfCreateSkill) Description() string {
	return "生成 PDF 文件，支持中英文混排。适合清单、简报、通知书导出"
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

	// Pick rendering path based on character content
	var writeErr error
	if containsNonASCII(content) || containsNonASCII(title) {
		writeErr = writePDFViaReportlab(ctx, absPath, title, lines)
		if writeErr != nil {
			// Fallback to Go path if reportlab isn't available
			writeErr = writeSimplePDF(absPath, lines)
		}
	} else {
		writeErr = writeSimplePDF(absPath, lines)
	}
	if writeErr != nil {
		return "", writeErr
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

func containsNonASCII(s string) bool {
	for _, r := range s {
		if r > 127 {
			return true
		}
	}
	return false
}

// writePDFViaReportlab uses Python + reportlab for proper CJK font embedding.
func writePDFViaReportlab(ctx context.Context, outPath, title string, lines []string) error {
	payload := map[string]any{"title": title, "lines": lines}
	dataBytes, _ := json.Marshal(payload)
	tmpData, err := os.CreateTemp("", "pdf_data_*.json")
	if err != nil {
		return err
	}
	tmpData.Write(dataBytes)
	tmpData.Close()
	defer os.Remove(tmpData.Name())

	tmpPy, err := os.CreateTemp("", "pdf_render_*.py")
	if err != nil {
		return err
	}
	tmpPy.WriteString(pdfReportlabScript)
	tmpPy.Close()
	defer os.Remove(tmpPy.Name())

	cmd := exec.CommandContext(ctx, "python", tmpPy.Name(), tmpData.Name(), outPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("reportlab PDF generation failed:\n%s\nError: %w", string(output), err)
	}
	return nil
}

const pdfReportlabScript = `import sys, json

try:
    from reportlab.lib.pagesizes import A4
    from reportlab.pdfgen import canvas
    from reportlab.pdfbase import pdfmetrics
    from reportlab.pdfbase.cidfonts import UnicodeCIDFont
except ImportError:
    sys.exit("reportlab is not installed. Run: pip install reportlab")

def main():
    data_path = sys.argv[1]
    out_path = sys.argv[2]

    with open(data_path, 'r', encoding='utf-8') as f:
        data = json.load(f)

    title = data.get('title', '')
    lines = data.get('lines', [])

    # Register CJK font
    pdfmetrics.registerFont(UnicodeCIDFont('STSong-Light'))

    c = canvas.Canvas(out_path, pagesize=A4)
    width, height = A4
    y = height - 60
    margin = 50
    line_height = 16
    max_width = width - 2 * margin

    # Title
    if title:
        c.setFont('STSong-Light', 18)
        c.drawString(margin, y, title)
        y -= 30

    c.setFont('STSong-Light', 11)
    for line in lines:
        line = line.rstrip()
        if line.startswith('#'):
            c.setFont('STSong-Light', 14)
            line = line.lstrip('# ')
            y -= 6
        else:
            c.setFont('STSong-Light', 11)

        # Simple line wrapping
        while len(line) > 0:
            # Estimate char count per line (CJK chars are ~2x width)
            fit = 0
            w = 0
            for ch in line:
                cw = 11 if ord(ch) > 127 else 6  # rough estimate
                if w + cw > max_width:
                    break
                w += cw
                fit += 1
            if fit == 0:
                fit = 1
            segment = line[:fit]
            line = line[fit:]
            c.drawString(margin, y, segment)
            y -= line_height
            if y < 60:
                c.showPage()
                c.setFont('STSong-Light', 11)
                y = height - 60

    c.save()

if __name__ == '__main__':
    main()
`

