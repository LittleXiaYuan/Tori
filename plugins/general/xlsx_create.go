package general

import (
	"archive/zip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"yunque-agent/pkg/skills"
)

type XlsxCreateSkill struct {
	allowedDirs []string
}

func NewXlsxCreateSkill(allowedDirs []string) *XlsxCreateSkill {
	return &XlsxCreateSkill{allowedDirs: allowedDirs}
}

func (s *XlsxCreateSkill) Name() string { return "xlsx_create" }
func (s *XlsxCreateSkill) Description() string {
	return "生成 Excel 表格(.xlsx)。用于创建数据报表、统计表格、清单等任务产出物。支持 CSV 格式输入"
}

func (s *XlsxCreateSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "输出文件路径（如 data/output/report.xlsx）",
			},
			"sheet_name": map[string]any{
				"type":        "string",
				"description": "工作表名称（默认 Sheet1）",
			},
			"data": map[string]any{
				"type":        "string",
				"description": "表格数据，CSV 格式（逗号分隔，换行分行）。第一行为表头",
			},
		},
		"required": []string{"path", "data"},
	}
}

func (s *XlsxCreateSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	path, _ := args["path"].(string)
	sheetName, _ := args["sheet_name"].(string)
	data, _ := args["data"].(string)

	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	if data == "" {
		return "", fmt.Errorf("data is required")
	}
	if sheetName == "" {
		sheetName = "Sheet1"
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

	rows := parseCSVData(data)
	if len(rows) == 0 {
		return "", fmt.Errorf("no data rows provided")
	}

	if err := writeXlsx(absPath, sheetName, rows); err != nil {
		return "", fmt.Errorf("xlsx generation failed: %w", err)
	}

	info, _ := os.Stat(absPath)
	size := int64(0)
	if info != nil {
		size = info.Size()
	}
	rowCount := len(rows)
	colCount := 0
	if rowCount > 0 {
		colCount = len(rows[0])
	}
	return fmt.Sprintf("已生成 Excel 表格: %s (%d bytes, %d行 × %d列)", path, size, rowCount, colCount), nil
}

func parseCSVData(data string) [][]string {
	lines := strings.Split(data, "\n")
	var rows [][]string
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		cells := strings.Split(line, ",")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		rows = append(rows, cells)
	}
	return rows
}

func writeXlsx(path, sheetName string, rows [][]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	writeZipFile(w, "[Content_Types].xml", xlsxContentTypes)
	writeZipFile(w, "_rels/.rels", xlsxRels)
	writeZipFile(w, "xl/_rels/workbook.xml.rels", xlsxWorkbookRels)
	writeZipFile(w, "xl/styles.xml", xlsxStyles)

	var sharedStrings []string
	ssIndex := make(map[string]int)
	for _, row := range rows {
		for _, cell := range row {
			if _, ok := ssIndex[cell]; !ok {
				ssIndex[cell] = len(sharedStrings)
				sharedStrings = append(sharedStrings, cell)
			}
		}
	}

	writeZipFile(w, "xl/sharedStrings.xml", buildSharedStringsXML(sharedStrings))
	writeZipFile(w, "xl/worksheets/sheet1.xml", buildSheetXML(rows, ssIndex))
	writeZipFile(w, "xl/workbook.xml", buildWorkbookXML(sheetName))

	return nil
}

func colRef(col int) string {
	result := ""
	for {
		result = string(rune('A'+col%26)) + result
		col = col/26 - 1
		if col < 0 {
			break
		}
	}
	return result
}

func buildSheetXML(rows [][]string, ssIndex map[string]int) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"
           xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
<sheetData>
`)
	for rowIdx, row := range rows {
		sb.WriteString(fmt.Sprintf(`<row r="%d">`, rowIdx+1))
		for colIdx, cell := range row {
			ref := colRef(colIdx) + fmt.Sprintf("%d", rowIdx+1)
			si := ssIndex[cell]
			if rowIdx == 0 {
				sb.WriteString(fmt.Sprintf(`<c r="%s" t="s" s="1"><v>%d</v></c>`, ref, si))
			} else {
				sb.WriteString(fmt.Sprintf(`<c r="%s" t="s"><v>%d</v></c>`, ref, si))
			}
		}
		sb.WriteString("</row>\n")
	}
	sb.WriteString(`</sheetData>
</worksheet>`)
	return sb.String()
}

func buildSharedStringsXML(strings_ []string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="%d" uniqueCount="%d">
`, len(strings_), len(strings_)))
	for _, s := range strings_ {
		sb.WriteString("<si><t>")
		sb.WriteString(docXMLEscape(s))
		sb.WriteString("</t></si>\n")
	}
	sb.WriteString("</sst>")
	return sb.String()
}

func buildWorkbookXML(sheetName string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"
          xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets>
    <sheet name="%s" sheetId="1" r:id="rId1"/>
  </sheets>
</workbook>`, docXMLEscape(sheetName))
}

const xlsxContentTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
  <Override PartName="/xl/sharedStrings.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sharedStrings+xml"/>
  <Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>
</Types>`

const xlsxRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`

const xlsxWorkbookRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/sharedStrings" Target="sharedStrings.xml"/>
  <Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`

const xlsxStyles = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <fonts count="2">
    <font><sz val="11"/><name val="Calibri"/></font>
    <font><b/><sz val="11"/><name val="Calibri"/></font>
  </fonts>
  <fills count="2">
    <fill><patternFill patternType="none"/></fill>
    <fill><patternFill patternType="gray125"/></fill>
  </fills>
  <borders count="1">
    <border><left/><right/><top/><bottom/><diagonal/></border>
  </borders>
  <cellStyleXfs count="1">
    <xf numFmtId="0" fontId="0" fillId="0" borderId="0"/>
  </cellStyleXfs>
  <cellXfs count="2">
    <xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/>
    <xf numFmtId="0" fontId="1" fillId="0" borderId="0" xfId="0" applyFont="1"/>
  </cellXfs>
</styleSheet>`
