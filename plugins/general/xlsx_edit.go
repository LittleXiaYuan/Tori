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

// ---- XLSX editing ----
//
// Edit cells, formulas, and styles in existing .xlsx files.
// Uses openpyxl to preserve charts, conditional formatting, etc.

type XlsxEditSkill struct {
	readDirs  []string
	writeDirs []string
}

func NewXlsxEditSkill(readDirs, writeDirs []string) *XlsxEditSkill {
	return &XlsxEditSkill{readDirs: readDirs, writeDirs: writeDirs}
}

func (s *XlsxEditSkill) Name() string { return "xlsx_edit" }

func (s *XlsxEditSkill) Description() string {
	return "编辑已有的 Excel 文件。支持：set_cell（写值）、set_formula（写公式）、add_row（追加行）、delete_row（删行）、set_style（设格式：加粗/颜色/边框）。修改后保存"
}

func (s *XlsxEditSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "要编辑的 .xlsx 文件路径",
			},
			"output_path": map[string]any{
				"type":        "string",
				"description": "另存为路径（可选，不填覆盖原文件）",
			},
			"sheet": map[string]any{
				"type":        "string",
				"description": "工作表名（可选，默认活动表）",
			},
			"operations": map[string]any{
				"type":        "array",
				"description": "操作列表",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"type":    map[string]any{"type": "string", "description": "set_cell, set_formula, add_row, delete_row, set_style"},
						"cell":    map[string]any{"type": "string", "description": "单元格地址 如 A1, B3"},
						"value":   map[string]any{"description": "set_cell: 要写入的值"},
						"formula": map[string]any{"type": "string", "description": "set_formula: 公式 如 =SUM(A1:A10)"},
						"row":     map[string]any{"type": "integer", "description": "delete_row: 要删的行号(1起)"},
						"values":  map[string]any{"type": "array", "description": "add_row: 值列表 [\"a\", 1, 2]"},
						"bold":    map[string]any{"type": "boolean", "description": "set_style: 是否加粗"},
						"color":   map[string]any{"type": "string", "description": "set_style: 字体颜色 hex 如 FF0000"},
					},
				},
			},
		},
		"required": []string{"path", "operations"},
	}
}

func (s *XlsxEditSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	path, _ := args["path"].(string)
	outPath, _ := args["output_path"].(string)
	sheet, _ := args["sheet"].(string)
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

	payload := map[string]any{"operations": ops, "sheet": sheet}
	dataBytes, _ := json.Marshal(payload)
	tmpData, _ := os.CreateTemp("", "xlsx_edit_*.json")
	tmpData.Write(dataBytes)
	tmpData.Close()
	defer os.Remove(tmpData.Name())

	tmpPy, _ := os.CreateTemp("", "xlsx_edit_*.py")
	tmpPy.WriteString(xlsxEditPython)
	tmpPy.Close()
	defer os.Remove(tmpPy.Name())

	cmd := exec.CommandContext(ctx, "python", tmpPy.Name(), absPath, absOut, tmpData.Name())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("xlsx_edit failed:\n%s\nError: %w", string(output), err)
	}

	return fmt.Sprintf("已编辑 Excel %s，执行了 %d 项操作", outPath, len(ops)), nil
}

const xlsxEditPython = `import sys, json

try:
    from openpyxl import load_workbook
    from openpyxl.styles import Font
except ImportError:
    sys.exit("openpyxl is not installed. Run: pip install openpyxl")

def main():
    src = sys.argv[1]
    out = sys.argv[2]
    cfg_path = sys.argv[3]

    with open(cfg_path, 'r', encoding='utf-8') as f:
        cfg = json.load(f)

    wb = load_workbook(src)
    sheet_name = cfg.get('sheet', '')
    ws = wb[sheet_name] if sheet_name and sheet_name in wb.sheetnames else wb.active

    for op in cfg['operations']:
        t = op.get('type', '')

        if t == 'set_cell':
            cell = op.get('cell', 'A1')
            ws[cell] = op.get('value', '')

        elif t == 'set_formula':
            cell = op.get('cell', 'A1')
            ws[cell] = op.get('formula', '')

        elif t == 'add_row':
            values = op.get('values', [])
            ws.append(values)

        elif t == 'delete_row':
            row = int(op.get('row', 1))
            ws.delete_rows(row)

        elif t == 'set_style':
            cell = op.get('cell', 'A1')
            c = ws[cell]
            bold = op.get('bold', None)
            color = op.get('color', None)
            font_kwargs = {}
            if c.font:
                font_kwargs = {
                    'name': c.font.name,
                    'size': c.font.size,
                    'bold': c.font.bold,
                    'italic': c.font.italic,
                    'color': c.font.color,
                }
            if bold is not None:
                font_kwargs['bold'] = bold
            if color:
                font_kwargs['color'] = color
            c.font = Font(**font_kwargs)

    wb.save(out)

if __name__ == '__main__':
    main()
`
