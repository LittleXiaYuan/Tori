package general

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"yunque-agent/pkg/skills"
)

// ---- DOCX template filling ----
//
// Opens a .docx template, replaces {{placeholder}} markers with real data,
// and saves as a new file. Formatting/styles from the template are preserved.
// Uses python-docx under the hood.

type DocxFillSkill struct {
	readDirs  []string
	writeDirs []string
}

func NewDocxFillSkill(readDirs, writeDirs []string) *DocxFillSkill {
	return &DocxFillSkill{readDirs: readDirs, writeDirs: writeDirs}
}

func (s *DocxFillSkill) Name() string { return "docx_fill" }

func (s *DocxFillSkill) Description() string {
	return "用数据填充 Word 模板。打开含 {{key}} 占位符的 .docx 模板文件，替换为实际内容并保存为新文件。保留原始排版样式。适用于合同、报告、通知书等模板化文档批量生成"
}

func (s *DocxFillSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"template_path": map[string]any{
				"type":        "string",
				"description": "模板文件路径（含 {{key}} 占位符的 .docx 文件）",
			},
			"output_path": map[string]any{
				"type":        "string",
				"description": "输出文件路径（如 data/output/contract_filled.docx）",
			},
			"data": map[string]any{
				"type":        "object",
				"description": "填充数据，键为占位符名（不含 {{ }}），值为替换文本。例：{\"company\": \"云雀科技\", \"date\": \"2026-03-26\"}",
			},
		},
		"required": []string{"template_path", "output_path", "data"},
	}
}

func (s *DocxFillSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	tplPath, _ := args["template_path"].(string)
	outPath, _ := args["output_path"].(string)
	data, _ := args["data"].(map[string]any)

	if tplPath == "" || outPath == "" || len(data) == 0 {
		return "", fmt.Errorf("template_path, output_path, and data are required")
	}

	absTpl, err := filepath.Abs(tplPath)
	if err != nil {
		return "", fmt.Errorf("invalid template path: %w", err)
	}
	if len(s.readDirs) > 0 && !isUnderAllowed(absTpl, s.readDirs) {
		return "", fmt.Errorf("access denied: template not under allowed read paths")
	}

	absOut, err := filepath.Abs(outPath)
	if err != nil {
		return "", fmt.Errorf("invalid output path: %w", err)
	}
	if !isUnderAllowed(absOut, s.writeDirs) {
		return "", fmt.Errorf("access denied: output not under allowed write paths")
	}
	if err := os.MkdirAll(filepath.Dir(absOut), 0755); err != nil {
		return "", fmt.Errorf("cannot create output directory: %w", err)
	}

	// Write data JSON to temp file
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal data: %w", err)
	}
	tmpData, err := os.CreateTemp("", "docx_fill_*.json")
	if err != nil {
		return "", err
	}
	tmpData.Write(dataBytes)
	tmpData.Close()
	defer os.Remove(tmpData.Name())

	// Write Python script to temp file
	tmpPy, err := os.CreateTemp("", "docx_fill_*.py")
	if err != nil {
		return "", err
	}
	tmpPy.WriteString(docxFillPython)
	tmpPy.Close()
	defer os.Remove(tmpPy.Name())

	cmd := exec.CommandContext(ctx, "python", tmpPy.Name(), absTpl, absOut, tmpData.Name())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docx_fill failed (is python-docx installed?):\n%s\nError: %w", string(output), err)
	}

	info, _ := os.Stat(absOut)
	size := int64(0)
	if info != nil {
		size = info.Size()
	}
	return fmt.Sprintf("已根据模板生成文档: %s (%d bytes, 替换了 %d 个字段)", outPath, size, len(data)), nil
}

const docxFillPython = `import sys, json

try:
    from docx import Document
except ImportError:
    sys.exit("python-docx is not installed. Run: pip install python-docx")

def replace_in_paragraph(paragraph, replacements):
    """Replace {{key}} in a paragraph while preserving run-level formatting."""
    full_text = paragraph.text
    changed = False
    for key, val in replacements.items():
        placeholder = "{{" + key + "}}"
        if placeholder in full_text:
            full_text = full_text.replace(placeholder, str(val))
            changed = True
    if changed:
        # Rebuild runs: clear all, write into first run to keep formatting
        if paragraph.runs:
            first_run = paragraph.runs[0]
            for run in paragraph.runs[1:]:
                run.text = ""
            first_run.text = full_text
        else:
            paragraph.text = full_text

def main():
    tpl_path = sys.argv[1]
    out_path = sys.argv[2]
    data_path = sys.argv[3]

    with open(data_path, 'r', encoding='utf-8') as f:
        data = json.load(f)

    doc = Document(tpl_path)

    # Replace in body paragraphs
    for para in doc.paragraphs:
        replace_in_paragraph(para, data)

    # Replace in tables
    for table in doc.tables:
        for row in table.rows:
            for cell in row.cells:
                for para in cell.paragraphs:
                    replace_in_paragraph(para, data)

    # Replace in headers/footers
    for section in doc.sections:
        for header in [section.header, section.first_page_header, section.even_page_header]:
            if header and header.paragraphs:
                for para in header.paragraphs:
                    replace_in_paragraph(para, data)
        for footer in [section.footer, section.first_page_footer, section.even_page_footer]:
            if footer and footer.paragraphs:
                for para in footer.paragraphs:
                    replace_in_paragraph(para, data)

    doc.save(out_path)

if __name__ == '__main__':
    main()
`
