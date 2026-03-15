package general

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"yunque-agent/pkg/skills"
)

// ──────────────────────────────────────────────
// DocxCreateSkill — generate .docx files from structured content
// DOCX is Open XML format: a zip file containing XML documents.
// ──────────────────────────────────────────────

type DocxCreateSkill struct {
	allowedDirs []string
}

func NewDocxCreateSkill(allowedDirs []string) *DocxCreateSkill {
	return &DocxCreateSkill{allowedDirs: allowedDirs}
}

func (s *DocxCreateSkill) Name() string { return "docx_create" }
func (s *DocxCreateSkill) Description() string {
	return "生成 Word 文档(.docx)。支持标题、段落、列表。用于创建报告、方案、文档等任务产出物"
}

func (s *DocxCreateSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "输出文件路径（如 data/output/report.docx）",
			},
			"title": map[string]any{
				"type":        "string",
				"description": "文档标题",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "文档内容，支持 Markdown 子集：# 标题、## 二级标题、- 列表项、普通段落。用换行分隔各部分",
			},
		},
		"required": []string{"path", "content"},
	}
}

func (s *DocxCreateSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	path, _ := args["path"].(string)
	title, _ := args["title"].(string)
	content, _ := args["content"].(string)

	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	if content == "" {
		return "", fmt.Errorf("content is required")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// Security: check allowed dirs
	if !isUnderAllowed(absPath, s.allowedDirs) {
		return "", fmt.Errorf("access denied: path %s is not under allowed directories", path)
	}

	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return "", fmt.Errorf("cannot create directory: %w", err)
	}

	// Parse markdown-like content into DOCX paragraphs
	paragraphs := parseDocContent(title, content)

	// Generate DOCX
	if err := writeDocx(absPath, paragraphs); err != nil {
		return "", fmt.Errorf("docx generation failed: %w", err)
	}

	info, _ := os.Stat(absPath)
	size := int64(0)
	if info != nil {
		size = info.Size()
	}
	return fmt.Sprintf("已生成 Word 文档: %s (%d bytes, %d 段落)", path, size, len(paragraphs)), nil
}

// docParagraph represents a paragraph in the document.
type docParagraph struct {
	Style string // "Title", "Heading1", "Heading2", "ListBullet", "Normal"
	Text  string
}

func parseDocContent(title, content string) []docParagraph {
	var paras []docParagraph

	if title != "" {
		paras = append(paras, docParagraph{Style: "Title", Text: title})
	}

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "## ") {
			paras = append(paras, docParagraph{Style: "Heading2", Text: strings.TrimPrefix(line, "## ")})
		} else if strings.HasPrefix(line, "# ") {
			paras = append(paras, docParagraph{Style: "Heading1", Text: strings.TrimPrefix(line, "# ")})
		} else if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			paras = append(paras, docParagraph{Style: "ListBullet", Text: line[2:]})
		} else {
			paras = append(paras, docParagraph{Style: "Normal", Text: line})
		}
	}
	return paras
}

func writeDocx(path string, paragraphs []docParagraph) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	// [Content_Types].xml
	writeZipFile(w, "[Content_Types].xml", contentTypesXML)

	// _rels/.rels
	writeZipFile(w, "_rels/.rels", relsXML)

	// word/_rels/document.xml.rels
	writeZipFile(w, "word/_rels/document.xml.rels", wordRelsXML)

	// word/styles.xml — minimal styles
	writeZipFile(w, "word/styles.xml", stylesXML)

	// word/document.xml — the actual content
	doc := buildDocumentXML(paragraphs)
	writeZipFile(w, "word/document.xml", doc)

	return nil
}

func writeZipFile(w *zip.Writer, name, content string) {
	f, err := w.Create(name)
	if err != nil {
		return
	}
	f.Write([]byte(content))
}

func buildDocumentXML(paragraphs []docParagraph) string {
	var sb strings.Builder
	sb.WriteString(docXMLHeader)

	for _, p := range paragraphs {
		sb.WriteString("    <w:p>")
		if p.Style != "Normal" {
			sb.WriteString("<w:pPr><w:pStyle w:val=\"")
			sb.WriteString(p.Style)
			sb.WriteString("\"/></w:pPr>")
		}
		sb.WriteString("<w:r><w:t xml:space=\"preserve\">")
		sb.WriteString(docXMLEscape(p.Text))
		sb.WriteString("</w:t></w:r></w:p>\n")
	}

	sb.WriteString(docXMLFooter)
	return sb.String()
}

func docXMLEscape(s string) string {
	var b strings.Builder
	xml.EscapeText(&b, []byte(s))
	return b.String()
}

// Minimal DOCX skeleton XML

const contentTypesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
</Types>`

const relsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`

const wordRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`

const stylesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:style w:type="paragraph" w:styleId="Title">
    <w:name w:val="Title"/>
    <w:pPr><w:jc w:val="center"/></w:pPr>
    <w:rPr><w:b/><w:sz w:val="56"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading1">
    <w:name w:val="heading 1"/>
    <w:pPr><w:spacing w:before="360" w:after="120"/></w:pPr>
    <w:rPr><w:b/><w:sz w:val="40"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading2">
    <w:name w:val="heading 2"/>
    <w:pPr><w:spacing w:before="240" w:after="80"/></w:pPr>
    <w:rPr><w:b/><w:sz w:val="32"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="ListBullet">
    <w:name w:val="List Bullet"/>
    <w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr></w:pPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Normal">
    <w:name w:val="Normal"/>
    <w:pPr><w:spacing w:after="120"/></w:pPr>
    <w:rPr><w:sz w:val="24"/></w:rPr>
  </w:style>
</w:styles>`

const docXMLHeader = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
            xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:body>
`

const docXMLFooter = `  </w:body>
</w:document>`

// ──────────────────────────────────────────────
// XlsxCreateSkill — generate .xlsx files from tabular data
// XLSX is also Open XML format: a zip file.
// ──────────────────────────────────────────────

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

	// Parse CSV data
	rows := parseCSVData(data)
	if len(rows) == 0 {
		return "", fmt.Errorf("no data rows provided")
	}

	// Generate XLSX
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

	// [Content_Types].xml
	writeZipFile(w, "[Content_Types].xml", xlsxContentTypes)

	// _rels/.rels
	writeZipFile(w, "_rels/.rels", xlsxRels)

	// xl/_rels/workbook.xml.rels
	writeZipFile(w, "xl/_rels/workbook.xml.rels", xlsxWorkbookRels)

	// xl/styles.xml — minimal styles with header bold
	writeZipFile(w, "xl/styles.xml", xlsxStyles)

	// Collect shared strings
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

	// xl/sharedStrings.xml
	writeZipFile(w, "xl/sharedStrings.xml", buildSharedStringsXML(sharedStrings))

	// xl/worksheets/sheet1.xml
	writeZipFile(w, "xl/worksheets/sheet1.xml", buildSheetXML(rows, ssIndex))

	// xl/workbook.xml
	writeZipFile(w, "xl/workbook.xml", buildWorkbookXML(sheetName))

	return nil
}

// colRef converts 0-based column index to Excel column letter (A, B, ..., Z, AA, AB, ...).
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
			// First row (header) uses style 1 (bold)
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

// Minimal XLSX skeleton XML

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

// isUnderAllowed checks if the target path is under one of the allowed directories.
func isUnderAllowed(absPath string, allowed []string) bool {
	for _, dir := range allowed {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		if strings.HasPrefix(absPath, absDir+string(filepath.Separator)) || absPath == absDir {
			return true
		}
	}
	return false
}
