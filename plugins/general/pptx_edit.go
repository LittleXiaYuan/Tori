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

// ---- PPTX editing ----
//
// Edit existing .pptx: modify slide text, add/delete slides, global text replace.
// Uses python-pptx to preserve design, transitions, animations.

type PptxEditSkill struct {
	readDirs  []string
	writeDirs []string
}

func NewPptxEditSkill(readDirs, writeDirs []string) *PptxEditSkill {
	return &PptxEditSkill{readDirs: readDirs, writeDirs: writeDirs}
}

func (s *PptxEditSkill) Name() string { return "pptx_edit" }

func (s *PptxEditSkill) Description() string {
	return "编辑已有的 PPT 演示文稿。支持：edit_slide（修改指定幻灯片标题/正文）、add_slide（新增幻灯片）、delete_slide（删除幻灯片）、replace_text（全文替换）。保留原始设计"
}

func (s *PptxEditSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "要编辑的 .pptx 文件路径",
			},
			"output_path": map[string]any{
				"type":        "string",
				"description": "另存为（可选，不填覆盖原文件）",
			},
			"operations": map[string]any{
				"type":        "array",
				"description": "操作列表",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"type":     map[string]any{"type": "string", "description": "edit_slide, add_slide, delete_slide, replace_text"},
						"slide":    map[string]any{"type": "integer", "description": "幻灯片编号(1起)"},
						"title":    map[string]any{"type": "string", "description": "edit_slide/add_slide: 标题"},
						"body":     map[string]any{"type": "string", "description": "edit_slide/add_slide: 正文"},
						"old_text": map[string]any{"type": "string", "description": "replace_text: 原文"},
						"new_text": map[string]any{"type": "string", "description": "replace_text: 替换为"},
					},
				},
			},
		},
		"required": []string{"path", "operations"},
	}
}

func (s *PptxEditSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	path, _ := args["path"].(string)
	outPath, _ := args["output_path"].(string)
	ops, _ := args["operations"].([]any)

	if path == "" || len(ops) == 0 {
		return "", fmt.Errorf("path and operations are required")
	}
	if outPath == "" {
		outPath = path
	}

	absPath, _ := filepath.Abs(path)
	absOut, _ := filepath.Abs(outPath)
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
	tmpData, _ := os.CreateTemp("", "pptx_edit_*.json")
	tmpData.Write(dataBytes)
	tmpData.Close()
	defer os.Remove(tmpData.Name())

	tmpPy, _ := os.CreateTemp("", "pptx_edit_*.py")
	tmpPy.WriteString(pptxEditPython)
	tmpPy.Close()
	defer os.Remove(tmpPy.Name())

	cmd := exec.CommandContext(ctx, "python", tmpPy.Name(), absPath, absOut, tmpData.Name())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("pptx_edit failed:\n%s\nError: %w", string(output), err)
	}

	return fmt.Sprintf("已编辑演示文稿 %s，执行了 %d 项操作", outPath, len(ops)), nil
}

const pptxEditPython = `import sys, json

try:
    from pptx import Presentation
except ImportError:
    sys.exit("python-pptx is not installed. Run: pip install python-pptx")

def main():
    src = sys.argv[1]
    out = sys.argv[2]
    cfg_path = sys.argv[3]

    with open(cfg_path, 'r', encoding='utf-8') as f:
        cfg = json.load(f)

    prs = Presentation(src)
    ops = cfg['operations']

    for op in ops:
        t = op.get('type', '')

        if t == 'edit_slide':
            idx = int(op.get('slide', 1)) - 1
            if 0 <= idx < len(prs.slides):
                slide = prs.slides[idx]
                title = op.get('title')
                body = op.get('body')
                if title and slide.shapes.title:
                    slide.shapes.title.text = title
                if body:
                    for shape in slide.shapes:
                        if shape.has_text_frame and shape != slide.shapes.title:
                            shape.text_frame.text = body
                            break

        elif t == 'add_slide':
            layout_idx = 1 if len(prs.slide_layouts) > 1 else 0
            slide = prs.slides.add_slide(prs.slide_layouts[layout_idx])
            title = op.get('title', '')
            body = op.get('body', '')
            if slide.shapes.title:
                slide.shapes.title.text = title
            for ph in slide.placeholders:
                if ph.placeholder_format.idx == 1:
                    ph.text = body
                    break

        elif t == 'delete_slide':
            idx = int(op.get('slide', 1)) - 1
            if 0 <= idx < len(prs.slides):
                rId = prs.slides._sldIdLst[idx].get('{http://schemas.openxmlformats.org/officeDocument/2006/relationships}id')
                prs.part.drop_rel(rId)
                del prs.slides._sldIdLst[idx]

        elif t == 'replace_text':
            old = op.get('old_text', '')
            new = op.get('new_text', '')
            if old:
                for slide in prs.slides:
                    for shape in slide.shapes:
                        if shape.has_text_frame:
                            for para in shape.text_frame.paragraphs:
                                for run in para.runs:
                                    if old in run.text:
                                        run.text = run.text.replace(old, new)

    prs.save(out)

if __name__ == '__main__':
    main()
`
