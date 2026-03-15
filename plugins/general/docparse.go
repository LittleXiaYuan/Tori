package general

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"yunque-agent/pkg/skills"

	"github.com/ledongthuc/pdf"
)

// DocParseSkill extracts text from PDF, DOCX, XLSX, CSV, TXT, and Markdown files.
type DocParseSkill struct {
	hostReadPaths []string
	maxBytes      int64
}

func NewDocParseSkill(hostReadPaths []string) *DocParseSkill {
	return &DocParseSkill{
		hostReadPaths: hostReadPaths,
		maxBytes:      10 << 20, // 10MB max
	}
}

func (s *DocParseSkill) Name() string        { return "doc_parse" }
func (s *DocParseSkill) Description() string { return "解析文档（PDF/Word/Excel/CSV/TXT），提取文本内容" }
func (s *DocParseSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "文档文件路径",
			},
			"max_chars": map[string]any{
				"type":        "integer",
				"description": "最大提取字符数（默认8000，最大32000）",
			},
			"page": map[string]any{
				"type":        "integer",
				"description": "指定页码（PDF专用，从1开始）",
			},
			"sheet": map[string]any{
				"type":        "string",
				"description": "指定工作表名（Excel专用）",
			},
		},
		"required": []string{"path"},
	}
}

func (s *DocParseSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	// Security: validate path is within allowed read paths
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	if !s.isAllowedPath(absPath) {
		return "", fmt.Errorf("access denied: path not in allowed directories")
	}

	// Check file exists and size
	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("file not found: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("path is a directory, not a file")
	}
	if info.Size() > s.maxBytes {
		return "", fmt.Errorf("file too large: %d bytes (max %d)", info.Size(), s.maxBytes)
	}

	maxChars := 8000
	if mc, ok := args["max_chars"].(float64); ok && mc > 0 {
		maxChars = int(mc)
		if maxChars > 32000 {
			maxChars = 32000
		}
	}

	ext := strings.ToLower(filepath.Ext(absPath))
	var text string

	switch ext {
	case ".pdf":
		page := 0
		if p, ok := args["page"].(float64); ok && p > 0 {
			page = int(p)
		}
		text, err = s.parsePDF(absPath, page)
	case ".docx":
		text, err = s.parseDOCX(absPath)
	case ".xlsx":
		sheet, _ := args["sheet"].(string)
		text, err = s.parseXLSX(absPath, sheet)
	case ".csv":
		text, err = s.parseCSV(absPath)
	case ".txt", ".md", ".markdown", ".log", ".json", ".xml", ".yaml", ".yml", ".toml", ".ini", ".cfg", ".conf":
		text, err = s.parseText(absPath)
	default:
		return "", fmt.Errorf("unsupported format: %s (supported: pdf, docx, xlsx, csv, txt, md)", ext)
	}

	if err != nil {
		return "", fmt.Errorf("parse error: %w", err)
	}

	// Truncate if needed
	if utf8.RuneCountInString(text) > maxChars {
		runes := []rune(text)
		text = string(runes[:maxChars]) + "\n\n... [truncated, total " + fmt.Sprintf("%d", len(runes)) + " chars]"
	}

	return fmt.Sprintf("File: %s\nFormat: %s\nSize: %d bytes\n\n%s", filepath.Base(absPath), ext, info.Size(), text), nil
}

func (s *DocParseSkill) isAllowedPath(absPath string) bool {
	if len(s.hostReadPaths) == 0 {
		return true
	}
	for _, allowed := range s.hostReadPaths {
		allowedAbs, err := filepath.Abs(allowed)
		if err != nil {
			continue
		}
		if strings.HasPrefix(strings.ToLower(absPath), strings.ToLower(allowedAbs)) {
			return true
		}
	}
	return false
}

// --- PDF ---

func (s *DocParseSkill) parsePDF(path string, pageNum int) (string, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	totalPages := r.NumPage()
	if totalPages == 0 {
		return "(empty PDF)", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[PDF: %d pages]\n\n", totalPages))

	if pageNum > 0 {
		// Single page
		if pageNum > totalPages {
			return "", fmt.Errorf("page %d out of range (total %d)", pageNum, totalPages)
		}
		p := r.Page(pageNum)
		text, err := p.GetPlainText(nil)
		if err != nil {
			return "", fmt.Errorf("page %d extraction failed: %w", pageNum, err)
		}
		sb.WriteString(fmt.Sprintf("--- Page %d ---\n", pageNum))
		sb.WriteString(text)
	} else {
		// All pages
		for i := 1; i <= totalPages; i++ {
			p := r.Page(i)
			text, err := p.GetPlainText(nil)
			if err != nil {
				sb.WriteString(fmt.Sprintf("--- Page %d (error: %v) ---\n", i, err))
				continue
			}
			if totalPages > 1 {
				sb.WriteString(fmt.Sprintf("--- Page %d ---\n", i))
			}
			sb.WriteString(text)
			sb.WriteString("\n")
		}
	}

	return sb.String(), nil
}

// --- DOCX ---

func (s *DocParseSkill) parseDOCX(path string) (string, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer zr.Close()

	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			defer rc.Close()
			return extractDocxText(rc)
		}
	}
	return "", fmt.Errorf("word/document.xml not found in docx")
}

func extractDocxText(r io.Reader) (string, error) {
	decoder := xml.NewDecoder(r)
	var sb strings.Builder
	inParagraph := false
	paragraphHasContent := false

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return sb.String(), nil // return what we have
		}

		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "p": // paragraph
				if paragraphHasContent {
					sb.WriteString("\n")
				}
				inParagraph = true
				paragraphHasContent = false
			case "tab":
				if inParagraph {
					sb.WriteString("\t")
				}
			case "br":
				if inParagraph {
					sb.WriteString("\n")
				}
			}
		case xml.EndElement:
			if t.Name.Local == "p" {
				inParagraph = false
			}
		case xml.CharData:
			if inParagraph {
				text := strings.TrimSpace(string(t))
				if text != "" {
					sb.WriteString(text)
					paragraphHasContent = true
				}
			}
		}
	}

	return sb.String(), nil
}

// --- XLSX ---

func (s *DocParseSkill) parseXLSX(path string, sheetName string) (string, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer zr.Close()

	// 1. Load shared strings
	sharedStrings := loadSharedStrings(zr)

	// 2. Find sheet files and their names
	sheetMap := loadSheetNames(zr)

	var sb strings.Builder
	for _, f := range zr.File {
		if !strings.HasPrefix(f.Name, "xl/worksheets/sheet") || !strings.HasSuffix(f.Name, ".xml") {
			continue
		}

		name := sheetMap[f.Name]
		if name == "" {
			name = f.Name
		}

		if sheetName != "" && !strings.EqualFold(name, sheetName) {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			continue
		}
		text := extractSheetText(rc, sharedStrings)
		rc.Close()

		sb.WriteString(fmt.Sprintf("[Sheet: %s]\n", name))
		sb.WriteString(text)
		sb.WriteString("\n\n")

		if sheetName != "" {
			break
		}
	}

	if sb.Len() == 0 {
		if sheetName != "" {
			return "", fmt.Errorf("sheet '%s' not found", sheetName)
		}
		return "(empty workbook)", nil
	}

	return sb.String(), nil
}

func loadSharedStrings(zr *zip.ReadCloser) []string {
	for _, f := range zr.File {
		if f.Name == "xl/sharedStrings.xml" {
			rc, err := f.Open()
			if err != nil {
				return nil
			}
			defer rc.Close()
			return parseSharedStrings(rc)
		}
	}
	return nil
}

func parseSharedStrings(r io.Reader) []string {
	decoder := xml.NewDecoder(r)
	var strings_ []string
	var current strings.Builder
	inT := false

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "t" {
				inT = true
			}
			if t.Name.Local == "si" {
				current.Reset()
			}
		case xml.EndElement:
			if t.Name.Local == "t" {
				inT = false
			}
			if t.Name.Local == "si" {
				strings_ = append(strings_, current.String())
			}
		case xml.CharData:
			if inT {
				current.Write(t)
			}
		}
	}
	return strings_
}

func loadSheetNames(zr *zip.ReadCloser) map[string]string {
	result := make(map[string]string)
	for _, f := range zr.File {
		if f.Name == "xl/workbook.xml" {
			rc, err := f.Open()
			if err != nil {
				return result
			}
			defer rc.Close()

			decoder := xml.NewDecoder(rc)
			idx := 1
			for {
				tok, err := decoder.Token()
				if err != nil {
					break
				}
				if se, ok := tok.(xml.StartElement); ok && se.Name.Local == "sheet" {
					for _, attr := range se.Attr {
						if attr.Name.Local == "name" {
							result[fmt.Sprintf("xl/worksheets/sheet%d.xml", idx)] = attr.Value
							idx++
						}
					}
				}
			}
			break
		}
	}
	return result
}

func extractSheetText(r io.Reader, sharedStrings []string) string {
	decoder := xml.NewDecoder(r)
	var sb strings.Builder
	var cellType string
	var cellValue string
	inV := false
	lastRow := ""

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "row":
				for _, attr := range t.Attr {
					if attr.Name.Local == "r" {
						if lastRow != "" {
							sb.WriteString("\n")
						}
						lastRow = attr.Value
					}
				}
			case "c":
				cellType = ""
				cellValue = ""
				for _, attr := range t.Attr {
					if attr.Name.Local == "t" {
						cellType = attr.Value
					}
				}
			case "v":
				inV = true
				cellValue = ""
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "v":
				inV = false
			case "c":
				val := cellValue
				if cellType == "s" && sharedStrings != nil {
					idx := 0
					fmt.Sscanf(cellValue, "%d", &idx)
					if idx < len(sharedStrings) {
						val = sharedStrings[idx]
					}
				}
				if val != "" {
					if sb.Len() > 0 {
						lastChar := sb.String()[sb.Len()-1]
						if lastChar != '\n' {
							sb.WriteString("\t")
						}
					}
					sb.WriteString(val)
				}
			}
		case xml.CharData:
			if inV {
				cellValue += string(t)
			}
		}
	}

	return sb.String()
}

// --- CSV ---

func (s *DocParseSkill) parseCSV(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	reader := csv.NewReader(bytes.NewReader(data))
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1

	records, err := reader.ReadAll()
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[CSV: %d rows]\n\n", len(records)))
	for _, row := range records {
		sb.WriteString(strings.Join(row, "\t"))
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// --- Text / Markdown ---

func (s *DocParseSkill) parseText(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(data) {
		// Try to handle as latin1
		runes := make([]rune, len(data))
		for i, b := range data {
			runes[i] = rune(b)
		}
		return string(runes), nil
	}
	return string(data), nil
}
