package document

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
	"yunque-agent/pkg/skills"
)

// MedicalExcelSplitSkill is a native Go pipeline for processing large hospital
// billing spreadsheets. It filters rows by doctor names and splits them into
// separate sheets with configurable percentage allocations.
type MedicalExcelSplitSkill struct{}

func (s *MedicalExcelSplitSkill) Name() string {
	return "split_medical_excel"
}

func (s *MedicalExcelSplitSkill) Description() string {
	return "原生高性能 Excel 处理引擎：按医生名单过滤大型门诊流水表，并按百分比比例拆分到独立 Sheet。支持万行级数据，低于 2 秒完成。参数: input_path(源文件), output_path(输出文件), source_sheet(源Sheet名,默认Sheet1), doctor_column(医生姓名所在列,如A/B/C), splits(拆分规则JSON数组,每项含name和percent)。"
}

func (s *MedicalExcelSplitSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"input_path": map[string]any{
				"type":        "string",
				"description": "源 Excel 文件的路径",
			},
			"output_path": map[string]any{
				"type":        "string",
				"description": "输出 Excel 文件的路径",
			},
			"source_sheet": map[string]any{
				"type":        "string",
				"description": "源 Sheet 名称，默认 Sheet1",
			},
			"doctor_column": map[string]any{
				"type":        "string",
				"description": "医生姓名所在的列字母，如 A、B、C",
			},
			"splits": map[string]any{
				"type":        "array",
				"description": "拆分规则数组，每项包含 name(医生/Sheet名) 和 percent(百分比,0-100)，percent 为 0 表示提取该医生全部数据不做比例截取",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name":    map[string]any{"type": "string"},
						"percent": map[string]any{"type": "number"},
					},
				},
			},
		},
		"required": []string{"input_path", "output_path", "doctor_column", "splits"},
	}
}

type splitRule struct {
	Name    string  `json:"name"`
	Percent float64 `json:"percent"`
}

func (s *MedicalExcelSplitSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	inputPath, _ := args["input_path"].(string)
	if inputPath == "" {
		return "", fmt.Errorf("input_path is required")
	}
	outputPath, _ := args["output_path"].(string)
	if outputPath == "" {
		return "", fmt.Errorf("output_path is required")
	}
	doctorCol, _ := args["doctor_column"].(string)
	if doctorCol == "" {
		return "", fmt.Errorf("doctor_column is required")
	}
	doctorCol = strings.ToUpper(strings.TrimSpace(doctorCol))

	sourceSheet, _ := args["source_sheet"].(string)
	if sourceSheet == "" {
		sourceSheet = "Sheet1"
	}

	var splits []splitRule
	switch v := args["splits"].(type) {
	case string:
		if err := json.Unmarshal([]byte(v), &splits); err != nil {
			return "", fmt.Errorf("splits JSON 解析失败: %w", err)
		}
	case []any:
		raw, _ := json.Marshal(v)
		if err := json.Unmarshal(raw, &splits); err != nil {
			return "", fmt.Errorf("splits 解析失败: %w", err)
		}
	default:
		return "", fmt.Errorf("splits 参数类型无效")
	}
	if len(splits) == 0 {
		return "", fmt.Errorf("splits 不能为空")
	}

	colIdx, err := excelize.ColumnNameToNumber(doctorCol)
	if err != nil {
		return "", fmt.Errorf("无效的列名 %q: %w", doctorCol, err)
	}
	colIdx-- // 0-based

	f, err := excelize.OpenFile(inputPath)
	if err != nil {
		return "", fmt.Errorf("无法打开 Excel 文件: %w", err)
	}
	defer f.Close()

	rows, err := f.GetRows(sourceSheet)
	if err != nil {
		return "", fmt.Errorf("读取 %s 失败: %w", sourceSheet, err)
	}
	if len(rows) == 0 {
		return "", fmt.Errorf("Sheet %s 为空", sourceSheet)
	}

	header := rows[0]
	dataRows := rows[1:]

	nameIndex := make(map[string][]int) // doctor name -> row indices in dataRows
	for i, row := range dataRows {
		if colIdx >= len(row) {
			continue
		}
		name := strings.TrimSpace(row[colIdx])
		if name != "" {
			nameIndex[name] = append(nameIndex[name], i)
		}
	}

	outF := excelize.NewFile()
	defer outF.Close()

	var report strings.Builder
	report.WriteString(fmt.Sprintf("源文件: %s\n总数据行: %d\n\n", filepath.Base(inputPath), len(dataRows)))

	for _, sp := range splits {
		name := strings.TrimSpace(sp.Name)
		if name == "" {
			continue
		}

		matchedIdxs := nameIndex[name]
		if len(matchedIdxs) == 0 {
			report.WriteString(fmt.Sprintf("⚠ %s: 未找到匹配的行\n", name))
			continue
		}

		sheetName := name
		if sp.Percent > 0 && sp.Percent < 100 {
			sheetName = fmt.Sprintf("%s%.0f%%", name, sp.Percent)
		}
		if len(sheetName) > 31 {
			sheetName = sheetName[:31]
		}

		outF.NewSheet(sheetName)

		for ci, cell := range header {
			colLetter, _ := excelize.ColumnNumberToName(ci + 1)
			outF.SetCellValue(sheetName, colLetter+"1", cell)
		}

		targetRows := matchedIdxs
		if sp.Percent > 0 && sp.Percent < 100 {
			take := int(math.Ceil(float64(len(matchedIdxs)) * sp.Percent / 100.0))
			if take > len(matchedIdxs) {
				take = len(matchedIdxs)
			}
			targetRows = matchedIdxs[:take]
		}

		for ri, dataIdx := range targetRows {
			row := dataRows[dataIdx]
			for ci, cell := range row {
				colLetter, _ := excelize.ColumnNumberToName(ci + 1)
				outF.SetCellValue(sheetName, fmt.Sprintf("%s%d", colLetter, ri+2), cell)
			}
		}

		report.WriteString(fmt.Sprintf("✅ %s: 匹配 %d 行", sheetName, len(matchedIdxs)))
		if sp.Percent > 0 && sp.Percent < 100 {
			report.WriteString(fmt.Sprintf("，取 %.0f%% = %d 行", sp.Percent, len(targetRows)))
		}
		report.WriteString("\n")
	}

	outF.DeleteSheet("Sheet1")

	if err := outF.SaveAs(outputPath); err != nil {
		return "", fmt.Errorf("保存输出文件失败: %w", err)
	}

	return fmt.Sprintf("【处理完成】\n%s输出路径: %s", report.String(), outputPath), nil
}
