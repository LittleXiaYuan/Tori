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

// ---- XLSX template filling ----
//
// Two modes:
//   - "placeholder": replaces {{key}} markers in cells (like docx_fill)
//   - "cell": writes values directly to cell addresses (e.g. {"A1": "foo", "B2": 42})
// Uses openpyxl to preserve formulas, charts, and conditional formatting.

type XlsxFillSkill struct {
	readDirs  []string
	writeDirs []string
}

func NewXlsxFillSkill(readDirs, writeDirs []string) *XlsxFillSkill {
	return &XlsxFillSkill{readDirs: readDirs, writeDirs: writeDirs}
}

func (s *XlsxFillSkill) Name() string { return "xlsx_fill" }

func (s *XlsxFillSkill) Description() string {
	return "填充 Excel 模板。两种模式：1) placeholder 模式替换 {{key}} 占位符；2) cell 模式按单元格坐标写值（如 A1, B3）。保留原有公式、图表、样式"
}

func (s *XlsxFillSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"template_path": map[string]any{
				"type":        "string",
				"description": "模板 .xlsx 路径",
			},
			"output_path": map[string]any{
				"type":        "string",
				"description": "输出文件路径",
			},
			"data": map[string]any{
				"type":        "object",
				"description": "填充数据。placeholder 模式: {\"company\": \"云雀\"}, cell 模式: {\"A1\": \"营收\", \"B1\": 12345}",
			},
			"mode": map[string]any{
				"type":        "string",
				"description": "填充模式：placeholder（默认）或 cell",
			},
			"sheet": map[string]any{
				"type":        "string",
				"description": "目标工作表名（可选，默认活动表）",
			},
		},
		"required": []string{"template_path", "output_path", "data"},
	}
}

func (s *XlsxFillSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	tplPath, _ := args["template_path"].(string)
	outPath, _ := args["output_path"].(string)
	data, _ := args["data"].(map[string]any)
	mode, _ := args["mode"].(string)
	sheet, _ := args["sheet"].(string)

	if tplPath == "" || outPath == "" || len(data) == 0 {
		return "", fmt.Errorf("template_path, output_path, and data are required")
	}
	if mode == "" {
		mode = "placeholder"
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

	payload := map[string]any{"data": data, "mode": mode, "sheet": sheet}
	dataBytes, _ := json.Marshal(payload)
	tmpData, err := os.CreateTemp("", "xlsx_fill_*.json")
	if err != nil {
		return "", err
	}
	tmpData.Write(dataBytes)
	tmpData.Close()
	defer os.Remove(tmpData.Name())

	tmpPy, err := os.CreateTemp("", "xlsx_fill_*.py")
	if err != nil {
		return "", err
	}
	tmpPy.WriteString(xlsxFillPython)
	tmpPy.Close()
	defer os.Remove(tmpPy.Name())

	cmd := exec.CommandContext(ctx, "python", tmpPy.Name(), absTpl, absOut, tmpData.Name())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("xlsx_fill failed (is openpyxl installed?):\n%s\nError: %w", string(output), err)
	}

	info, _ := os.Stat(absOut)
	size := int64(0)
	if info != nil {
		size = info.Size()
	}
	return fmt.Sprintf("已根据模板生成 Excel: %s (%d bytes, %s 模式, %d 项数据)", outPath, size, mode, len(data)), nil
}

const xlsxFillPython = `import sys, json

try:
    from openpyxl import load_workbook
except ImportError:
    sys.exit("openpyxl is not installed. Run: pip install openpyxl")

def main():
    tpl_path = sys.argv[1]
    out_path = sys.argv[2]
    config_path = sys.argv[3]

    with open(config_path, 'r', encoding='utf-8') as f:
        config = json.load(f)

    data = config['data']
    mode = config.get('mode', 'placeholder')
    sheet_name = config.get('sheet', '')

    wb = load_workbook(tpl_path)
    ws = wb[sheet_name] if sheet_name and sheet_name in wb.sheetnames else wb.active

    if mode == 'cell':
        for cell_ref, value in data.items():
            ws[cell_ref] = value
    else:
        # placeholder mode: scan all cells for {{key}}
        for row in ws.iter_rows():
            for cell in row:
                if cell.value and isinstance(cell.value, str):
                    original = cell.value
                    for key, val in data.items():
                        placeholder = "{{" + key + "}}"
                        if placeholder in original:
                            original = original.replace(placeholder, str(val))
                    if original != cell.value:
                        cell.value = original

    wb.save(out_path)

if __name__ == '__main__':
    main()
`
