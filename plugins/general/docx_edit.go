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

// ---- DOCX editing ----
//
// Structured editing of existing .docx files: replace text, insert/delete
// paragraphs, add tables. Uses python-docx to preserve formatting.

type DocxEditSkill struct {
	readDirs  []string
	writeDirs []string
}

func NewDocxEditSkill(readDirs, writeDirs []string) *DocxEditSkill {
	return &DocxEditSkill{readDirs: readDirs, writeDirs: writeDirs}
}

func (s *DocxEditSkill) Name() string { return "docx_edit" }

func (s *DocxEditSkill) Description() string {
	return "编辑已有的 Word 文档。支持：replace_text（全文替换）、edit_paragraph（按索引改段落）、add_paragraph（插入段落）、delete_paragraph（删段落）、add_table（加表格）。修改就地保存或另存新文件"
}

func (s *DocxEditSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "要编辑的 .docx 文件路径",
			},
			"output_path": map[string]any{
				"type":        "string",
				"description": "另存为路径（可选，不填则覆盖原文件）",
			},
			"operations": map[string]any{
				"type":        "array",
				"description": "操作列表，按顺序执行",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"type": map[string]any{
							"type":        "string",
							"description": "操作类型: replace_text, edit_paragraph, add_paragraph, delete_paragraph, add_table",
						},
						"old_text":  map[string]any{"type": "string", "description": "replace_text: 要替换的原文"},
						"new_text":  map[string]any{"type": "string", "description": "replace_text/edit_paragraph/add_paragraph: 新内容"},
						"index":     map[string]any{"type": "integer", "description": "edit_paragraph/delete_paragraph: 段落索引(0起)；add_paragraph: 插入位置"},
						"style":     map[string]any{"type": "string", "description": "add_paragraph: 段落样式(Heading 1, List Bullet等)"},
						"rows":      map[string]any{"type": "array", "description": "add_table: 二维数组 [[\"A\",\"B\"],[\"1\",\"2\"]]"},
					},
				},
			},
		},
		"required": []string{"path", "operations"},
	}
}

func (s *DocxEditSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	path, _ := args["path"].(string)
	outPath, _ := args["output_path"].(string)
	ops, _ := args["operations"].([]any)

	if path == "" || len(ops) == 0 {
		return "", fmt.Errorf("path and operations are required")
	}
	if outPath == "" {
		outPath = path
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	absOut, err := filepath.Abs(outPath)
	if err != nil {
		return "", err
	}
	// Need read access to source and write access to output
	if len(s.readDirs) > 0 && !isUnderAllowed(absPath, s.readDirs) {
		return "", fmt.Errorf("access denied: source not under allowed paths")
	}
	if !isUnderAllowed(absOut, s.writeDirs) {
		return "", fmt.Errorf("access denied: output not under allowed write paths")
	}
	if err := os.MkdirAll(filepath.Dir(absOut), 0755); err != nil {
		return "", err
	}

	payload := map[string]any{"operations": ops}
	dataBytes, _ := json.Marshal(payload)
	tmpData, _ := os.CreateTemp("", "docx_edit_*.json")
	tmpData.Write(dataBytes)
	tmpData.Close()
	defer os.Remove(tmpData.Name())

	tmpPy, _ := os.CreateTemp("", "docx_edit_*.py")
	tmpPy.WriteString(docxEditPython)
	tmpPy.Close()
	defer os.Remove(tmpPy.Name())

	cmd := exec.CommandContext(ctx, "python", tmpPy.Name(), absPath, absOut, tmpData.Name())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docx_edit failed:\n%s\nError: %w", string(output), err)
	}

	return fmt.Sprintf("已编辑文档 %s，执行了 %d 项操作", outPath, len(ops)), nil
}

const docxEditPython = `import sys, json

try:
    from docx import Document
    from docx.oxml.ns import qn
except ImportError:
    sys.exit("python-docx is not installed. Run: pip install python-docx")

def main():
    src = sys.argv[1]
    out = sys.argv[2]
    cfg_path = sys.argv[3]

    with open(cfg_path, 'r', encoding='utf-8') as f:
        cfg = json.load(f)

    doc = Document(src)
    ops = cfg['operations']

    for op in ops:
        t = op.get('type', '')

        if t == 'replace_text':
            old = op.get('old_text', '')
            new = op.get('new_text', '')
            if old:
                for para in doc.paragraphs:
                    if old in para.text:
                        for run in para.runs:
                            if old in run.text:
                                run.text = run.text.replace(old, new)
                for table in doc.tables:
                    for row in table.rows:
                        for cell in row.cells:
                            for para in cell.paragraphs:
                                for run in para.runs:
                                    if old in run.text:
                                        run.text = run.text.replace(old, new)

        elif t == 'edit_paragraph':
            idx = int(op.get('index', 0))
            new_text = op.get('new_text', '')
            if 0 <= idx < len(doc.paragraphs):
                para = doc.paragraphs[idx]
                if para.runs:
                    para.runs[0].text = new_text
                    for run in para.runs[1:]:
                        run.text = ""
                else:
                    para.text = new_text

        elif t == 'add_paragraph':
            text = op.get('new_text', '')
            style = op.get('style', None)
            idx = op.get('index', None)
            if idx is not None and isinstance(idx, (int, float)):
                idx = int(idx)
                if 0 <= idx < len(doc.paragraphs):
                    ref_para = doc.paragraphs[idx]
                    new_p = doc.add_paragraph(text, style=style)
                    ref_para._p.addprevious(new_p._p)
                else:
                    doc.add_paragraph(text, style=style)
            else:
                doc.add_paragraph(text, style=style)

        elif t == 'delete_paragraph':
            idx = int(op.get('index', 0))
            if 0 <= idx < len(doc.paragraphs):
                p = doc.paragraphs[idx]._p
                p.getparent().remove(p)

        elif t == 'add_table':
            rows_data = op.get('rows', [])
            if rows_data:
                n_rows = len(rows_data)
                n_cols = max(len(r) for r in rows_data) if rows_data else 1
                table = doc.add_table(rows=n_rows, cols=n_cols)
                table.style = 'Table Grid'
                for i, row_data in enumerate(rows_data):
                    for j, val in enumerate(row_data):
                        table.cell(i, j).text = str(val)

    doc.save(out)

if __name__ == '__main__':
    main()
`
