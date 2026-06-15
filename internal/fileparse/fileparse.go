// Package fileparse extracts text previews and user-facing parse metadata from
// uploaded or generated file bytes. It is a Tier-0 shared utility: it depends
// only on the standard library so both the control-plane gateway (upload
// analysis) and capability packs (e.g. the files pack preview surface) can use
// it without coupling to the gateway monolith.
package fileparse

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

const maxPreviewBytes = 256 << 10 // 256 KiB preview for LLM

// Result is a parsed text preview plus user-facing parse metadata.
type Result struct {
	Preview       string
	Parser        string
	Backend       string
	MarkdownChars int
	HasLayoutJSON bool
	Status        string
	Note          string
}

// Parse extracts a text preview plus user-facing parse metadata from file bytes
// based on the filename extension.
//
// Important: for document-like formats that require the external document
// parser (.pdf/.doc/.ppt/.xls), Preview is intentionally empty when the local
// parser cannot unfold the body. This avoids passing a misleading placeholder
// to the model as if it were the document content.
func Parse(filename string, data []byte) Result {
	ext := strings.ToLower(filepath.Ext(filename))
	result := Result{
		Parser:  "local",
		Backend: "go",
		Status:  "parsed",
	}
	setPreview := func(s string) Result {
		result.Preview = s
		result.MarkdownChars = len([]rune(s))
		return result
	}
	switch ext {
	case ".csv":
		if len(data) > maxPreviewBytes {
			data = data[:maxPreviewBytes]
		}
		s, err := parseCSVBytes(data)
		if err != nil {
			result.Status = "error"
			return setPreview(fmt.Sprintf("(csv parse error: %v)", err))
		}
		return setPreview(s)
	case ".xlsx":
		s, err := parseXLSXBytes(data, "")
		if err != nil {
			result.Status = "error"
			return setPreview(fmt.Sprintf("(xlsx parse error: %v)", err))
		}
		return setPreview(s)
	case ".docx":
		s, err := parseDOCXBytes(data)
		if err != nil {
			result.Status = "error"
			return setPreview(fmt.Sprintf("(docx parse error: %v)", err))
		}
		return setPreview(s)
	case ".pptx":
		s, err := parsePPTXBytes(data)
		if err != nil {
			result.Status = "error"
			return setPreview(fmt.Sprintf("(pptx parse error: %v)", err))
		}
		return setPreview(s)
	case ".txt", ".md", ".markdown", ".log", ".json", ".xml", ".yaml", ".yml":
		if len(data) > maxPreviewBytes {
			data = data[:maxPreviewBytes]
		}
		if !utf8.Valid(data) {
			return setPreview(string(data)) // best effort
		}
		return setPreview(string(data))
	default:
		if requiresDocumentParser(ext) {
			result.Parser = "document"
			result.Backend = "external"
			result.Status = "needs_document_parser"
			result.Note = fmt.Sprintf("附件已添加，但当前本地解析器还不能直接展开 %s 正文；配置文档解析后端后会自动提取正文。", ext)
			return result
		}
		if utf8.Valid(data) && len(data) < 64<<10 {
			return setPreview(string(data))
		}
		result.Status = "unsupported"
		result.Note = fmt.Sprintf("附件已添加，但当前无法直接预览该文件类型（%s，%d bytes）。", ext, len(data))
		return result
	}
}

// Metadata projects a Result into a JSON-serializable map, truncating the
// preview to previewLimit runes when positive. Returns nil for an empty result.
func Metadata(result Result, previewLimit int) map[string]any {
	if result.Parser == "" && result.Backend == "" && result.Status == "" && result.Note == "" && strings.TrimSpace(result.Preview) == "" {
		return nil
	}
	preview := result.Preview
	if previewLimit > 0 {
		if r := []rune(preview); len(r) > previewLimit {
			preview = string(r[:previewLimit]) + "\n\n...已截断"
		}
	}
	meta := map[string]any{
		"parser":          result.Parser,
		"backend":         result.Backend,
		"markdown_chars":  result.MarkdownChars,
		"has_layout_json": result.HasLayoutJSON,
		"status":          result.Status,
		"note":            result.Note,
	}
	if strings.TrimSpace(preview) != "" {
		meta["preview"] = preview
	}
	return meta
}

func requiresDocumentParser(ext string) bool {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case ".pdf", ".doc", ".ppt", ".xls":
		return true
	default:
		return false
	}
}

func parsePPTXBytes(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}
	var slides []*zip.File
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "ppt/slides/slide") && strings.HasSuffix(f.Name, ".xml") {
			slides = append(slides, f)
		}
	}
	sort.Slice(slides, func(i, j int) bool { return slides[i].Name < slides[j].Name })
	var sb strings.Builder
	for i, f := range slides {
		rc, err := f.Open()
		if err != nil {
			continue
		}
		text := strings.TrimSpace(extractPPTXSlideText(rc))
		rc.Close()
		if text == "" {
			continue
		}
		sb.WriteString(fmt.Sprintf("[Slide %d]\n", i+1))
		sb.WriteString(text)
		sb.WriteString("\n\n")
	}
	if sb.Len() == 0 {
		return "", fmt.Errorf("no slide text found")
	}
	return sb.String(), nil
}

func extractPPTXSlideText(r io.Reader) string {
	decoder := xml.NewDecoder(r)
	var sb strings.Builder
	inText := false
	paragraphHasContent := false
	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "p":
				if paragraphHasContent && !strings.HasSuffix(sb.String(), "\n") {
					sb.WriteString("\n")
				}
				paragraphHasContent = false
			case "t":
				inText = true
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "t":
				inText = false
			case "p":
				if paragraphHasContent && !strings.HasSuffix(sb.String(), "\n") {
					sb.WriteString("\n")
				}
			}
		case xml.CharData:
			if inText {
				text := strings.TrimSpace(string(t))
				if text != "" {
					if paragraphHasContent {
						sb.WriteString(" ")
					}
					sb.WriteString(text)
					paragraphHasContent = true
				}
			}
		}
	}
	return sb.String()
}

func parseCSVBytes(data []byte) (string, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.LazyQuotes = true
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[CSV preview: %d rows]\n", len(records)))
	max := 80
	if len(records) < max {
		max = len(records)
	}
	for i := 0; i < max; i++ {
		sb.WriteString(strings.Join(records[i], "\t"))
		sb.WriteString("\n")
	}
	if len(records) > max {
		sb.WriteString("...\n")
	}
	return sb.String(), nil
}

func parseDOCXBytes(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			text, err := extractDocxText(rc)
			rc.Close()
			return text, err
		}
	}
	return "", fmt.Errorf("word/document.xml not found")
}

func extractDocxText(r io.Reader) (string, error) {
	decoder := xml.NewDecoder(r)
	var sb strings.Builder
	inParagraph := false
	paragraphHasContent := false
	inTableRow := false
	rowHasCell := false

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return sb.String(), nil
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "tr":
				if sb.Len() > 0 && !strings.HasSuffix(sb.String(), "\n") {
					sb.WriteString("\n")
				}
				inTableRow = true
				rowHasCell = false
			case "tc":
				if inTableRow {
					if rowHasCell {
						sb.WriteString("\t")
					}
					rowHasCell = true
					// A table cell boundary already carries the separator. Reset
					// paragraph state so the first paragraph in the next cell does
					// not inherit paragraphHasContent from the previous cell and
					// accidentally insert a newline after the tab.
					paragraphHasContent = false
				}
			case "p":
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
			switch t.Name.Local {
			case "p":
				inParagraph = false
			case "tr":
				if sb.Len() > 0 && !strings.HasSuffix(sb.String(), "\n") {
					sb.WriteString("\n")
				}
				inTableRow = false
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

func parseXLSXBytes(data []byte, sheetName string) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}
	shared := loadSharedStringsZip(zr)
	sheetMap := loadSheetNamesZip(zr)
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
		text := extractSheetTextZip(rc, shared)
		rc.Close()
		sb.WriteString(fmt.Sprintf("[Sheet: %s]\n", name))
		sb.WriteString(text)
		sb.WriteString("\n\n")
		if sheetName != "" {
			break
		}
	}
	if sb.Len() == 0 {
		return "(empty workbook)", nil
	}
	return sb.String(), nil
}

func loadSharedStringsZip(zr *zip.Reader) []string {
	for _, f := range zr.File {
		if f.Name == "xl/sharedStrings.xml" {
			rc, err := f.Open()
			if err != nil {
				return nil
			}
			defer rc.Close()
			return parseSharedStringsZip(rc)
		}
	}
	return nil
}

func parseSharedStringsZip(r io.Reader) []string {
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

func loadSheetNamesZip(zr *zip.Reader) map[string]string {
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

func extractSheetTextZip(r io.Reader, sharedStrings []string) string {
	decoder := xml.NewDecoder(r)
	var sb strings.Builder
	var cellType string
	var cellValue string
	inV := false
	inInlineText := false
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
			case "t":
				if cellType == "inlineStr" {
					inInlineText = true
				}
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "v":
				inV = false
			case "t":
				inInlineText = false
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
			} else if inInlineText {
				cellValue += string(t)
			}
		}
	}
	return sb.String()
}
