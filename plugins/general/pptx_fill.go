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

// ---- PPTX template filling ----
//
// Opens a .pptx template, replaces {{placeholder}} in titles, body text,
// table cells, and speaker notes. Uses python-pptx; preserves all design/layout.

type PptxFillSkill struct {
	readDirs  []string
	writeDirs []string
}

func NewPptxFillSkill(readDirs, writeDirs []string) *PptxFillSkill {
	return &PptxFillSkill{readDirs: readDirs, writeDirs: writeDirs}
}

func (s *PptxFillSkill) Name() string { return "pptx_fill" }

func (s *PptxFillSkill) Description() string {
	return "填充 PPT 模板。打开含 {{key}} 占位符的 .pptx 模板，替换标题、正文、表格、备注中的占位符。保留原始设计和排版"
}

func (s *PptxFillSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"template_path": map[string]any{
				"type":        "string",
				"description": "模板 .pptx 路径",
			},
			"output_path": map[string]any{
				"type":        "string",
				"description": "输出文件路径",
			},
			"data": map[string]any{
				"type":        "object",
				"description": "填充数据，键为占位符名。例：{\"title\": \"Q2 业绩汇报\", \"revenue\": \"1.2亿\"}",
			},
		},
		"required": []string{"template_path", "output_path", "data"},
	}
}

func (s *PptxFillSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
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
		return "", err
	}

	dataBytes, _ := json.Marshal(data)
	tmpData, err := os.CreateTemp("", "pptx_fill_*.json")
	if err != nil {
		return "", err
	}
	tmpData.Write(dataBytes)
	tmpData.Close()
	defer os.Remove(tmpData.Name())

	tmpPy, err := os.CreateTemp("", "pptx_fill_*.py")
	if err != nil {
		return "", err
	}
	tmpPy.WriteString(pptxFillPython)
	tmpPy.Close()
	defer os.Remove(tmpPy.Name())

	cmd := exec.CommandContext(ctx, "python", tmpPy.Name(), absTpl, absOut, tmpData.Name())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("pptx_fill failed (is python-pptx installed?):\n%s\nError: %w", string(output), err)
	}

	info, _ := os.Stat(absOut)
	size := int64(0)
	if info != nil {
		size = info.Size()
	}
	return fmt.Sprintf("已根据模板生成演示文稿: %s (%d bytes, 替换了 %d 个字段)", outPath, size, len(data)), nil
}

const pptxFillPython = `import sys, json

try:
    from pptx import Presentation
except ImportError:
    sys.exit("python-pptx is not installed. Run: pip install python-pptx")

def replace_in_text_frame(tf, replacements):
    """Replace {{key}} in a text frame while keeping run formatting."""
    for para in tf.paragraphs:
        full = para.text
        changed = False
        for key, val in replacements.items():
            ph = "{{" + key + "}}"
            if ph in full:
                full = full.replace(ph, str(val))
                changed = True
        if changed and para.runs:
            para.runs[0].text = full
            for run in para.runs[1:]:
                run.text = ""

def main():
    tpl_path = sys.argv[1]
    out_path = sys.argv[2]
    data_path = sys.argv[3]

    with open(data_path, 'r', encoding='utf-8') as f:
        data = json.load(f)

    prs = Presentation(tpl_path)

    for slide in prs.slides:
        for shape in slide.shapes:
            # Text frames (titles, body, text boxes)
            if shape.has_text_frame:
                replace_in_text_frame(shape.text_frame, data)
            # Tables
            if shape.has_table:
                for row in shape.table.rows:
                    for cell in row.cells:
                        replace_in_text_frame(cell.text_frame, data)
        # Speaker notes
        if slide.has_notes_slide and slide.notes_slide.notes_text_frame:
            replace_in_text_frame(slide.notes_slide.notes_text_frame, data)

    prs.save(out_path)

if __name__ == '__main__':
    main()
`
