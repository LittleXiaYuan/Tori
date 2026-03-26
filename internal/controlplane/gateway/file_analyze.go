package gateway

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const fileAnalyzeMaxBytes = 256 << 10 // 256 KiB preview for LLM

// TryParseFile extracts a text preview from upload bytes based on filename extension.
func TryParseFile(filename string, data []byte) string {
	if len(data) > fileAnalyzeMaxBytes {
		data = data[:fileAnalyzeMaxBytes]
	}
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".csv":
		s, err := parseCSVBytes(data)
		if err != nil {
			return fmt.Sprintf("(csv parse error: %v)", err)
		}
		return s
	case ".xlsx":
		s, err := parseXLSXBytes(data, "")
		if err != nil {
			return fmt.Sprintf("(xlsx parse error: %v)", err)
		}
		return s
	case ".docx":
		s, err := parseDOCXBytes(data)
		if err != nil {
			return fmt.Sprintf("(docx parse error: %v)", err)
		}
		return s
	case ".txt", ".md", ".markdown", ".log", ".json", ".xml", ".yaml", ".yml":
		if !utf8.Valid(data) {
			return string(data) // best effort
		}
		return string(data)
	default:
		if utf8.Valid(data) && len(data) < 64<<10 {
			return string(data)
		}
		return fmt.Sprintf("(binary or unsupported extension %s, %d bytes)", ext, len(data))
	}
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
			text, err := extractDocxTextGateway(rc)
			rc.Close()
			return text, err
		}
	}
	return "", fmt.Errorf("word/document.xml not found")
}

func extractDocxTextGateway(r io.Reader) (string, error) {
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
			return sb.String(), nil
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
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
