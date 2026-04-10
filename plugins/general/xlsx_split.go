package general

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"yunque-agent/pkg/skills"
)

// XlsxSplitSkill splits an xlsx by distinct values in one column.
// Each output file keeps the header row + matching data rows.
type XlsxSplitSkill struct {
	readDirs  []string
	writeDirs []string
}

func NewXlsxSplitSkill(readDirs, writeDirs []string) *XlsxSplitSkill {
	return &XlsxSplitSkill{readDirs: readDirs, writeDirs: writeDirs}
}

func (s *XlsxSplitSkill) Name() string { return "xlsx_split" }

func (s *XlsxSplitSkill) Description() string {
	return "将 Excel(.xlsx) 按某一列的去重值拆成多个 .xlsx（每份保留表头）。列号为 1-based，默认处理第一个工作表"
}

func (s *XlsxSplitSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "源 xlsx 路径",
			},
			"column": map[string]any{
				"type":        "integer",
				"description": "用于分组的列号（从 1 开始，例如 1 表示 A 列）",
			},
			"output_dir": map[string]any{
				"type":        "string",
				"description": "输出目录（须位于允许写入路径下，默认 data/output）",
			},
			"sheet": map[string]any{
				"type":        "string",
				"description": "工作表名称（可选，默认第一个表）",
			},
			"prefix": map[string]any{
				"type":        "string",
				"description": "输出文件名前缀（可选，默认 split）",
			},
		},
		"required": []string{"path", "column"},
	}
}

func (s *XlsxSplitSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	_ = ctx
	_ = env
	src, _ := args["path"].(string)
	colF, _ := args["column"].(float64)
	colIdx := int(colF)
	if src == "" || colIdx < 1 {
		return "", fmt.Errorf("path and column (>=1) are required")
	}
	outDir, _ := args["output_dir"].(string)
	if outDir == "" {
		outDir = "data/output"
	}
	sheetName, _ := args["sheet"].(string)
	prefix, _ := args["prefix"].(string)
	if prefix == "" {
		prefix = "split"
	}

	absSrc, err := filepath.Abs(src)
	if err != nil {
		return "", err
	}
	if len(s.readDirs) > 0 && !isUnderAllowed(absSrc, s.readDirs) {
		return "", fmt.Errorf("access denied: source not under allowed read paths")
	}
	absOut, err := filepath.Abs(outDir)
	if err != nil {
		return "", err
	}
	if !isUnderAllowed(absOut, s.writeDirs) {
		return "", fmt.Errorf("access denied: output_dir not under allowed write paths")
	}
	if err := os.MkdirAll(absOut, 0755); err != nil {
		return "", err
	}

	matrix, sheet, err := readXLSXMatrix(absSrc, sheetName)
	if err != nil {
		return "", err
	}
	if len(matrix) < 2 {
		return "", fmt.Errorf("need at least a header row and one data row")
	}
	header := matrix[0]
	keyCol := colIdx - 1
	if keyCol >= len(header) {
		return "", fmt.Errorf("column %d out of range (width %d)", colIdx, len(header))
	}

	groups := map[string][][]string{}
	for _, row := range matrix[1:] {
		key := ""
		if keyCol < len(row) {
			key = strings.TrimSpace(row[keyCol])
		}
		if key == "" {
			key = "_empty"
		}
		safe := sanitizeFilePart(key)
		groups[safe] = append(groups[safe], row)
	}

	var outFiles []string
	for k, rows := range groups {
		rows = append([][]string{header}, rows...)
		outName := fmt.Sprintf("%s_%s.xlsx", prefix, k)
		outPath := filepath.Join(absOut, outName)
		if err := writeXlsx(outPath, sheet, rows); err != nil {
			return "", fmt.Errorf("write %s: %w", outName, err)
		}
		outFiles = append(outFiles, outPath)
	}
	return fmt.Sprintf("已按第 %d 列拆分为 %d 个文件：\n%s", colIdx, len(outFiles), strings.Join(outFiles, "\n")), nil
}

func sanitizeFilePart(s string) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		case r == ' ':
			return '_'
		default:
			return '_'
		}
	}, s)
	if s == "" {
		return "part"
	}
	if len(s) > 48 {
		s = s[:48]
	}
	return s
}

// readXLSXMatrix opens an xlsx (which is a zip of OOXML parts) and reads
// the first (or named) sheet into a string matrix.
//
// OOXML structure we care about:
//   xl/sharedStrings.xml  — string table (cells reference these by index)
//   xl/workbook.xml       — lists sheet names
//   xl/worksheets/sheet1.xml — actual cell data
func readXLSXMatrix(path, wantSheet string) ([][]string, string, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, "", err
	}
	defer zr.Close()

	shared := xlsxLoadSharedStrings(&zr.Reader)
	sheetMap := xlsxLoadSheetNames(&zr.Reader)
	var targetFile string
	var sheetTitle string
	for fname, title := range sheetMap {
		if wantSheet == "" || strings.EqualFold(title, wantSheet) {
			targetFile = fname
			sheetTitle = title
			break
		}
	}
	if targetFile == "" {
		for _, f := range zr.File {
			if strings.HasPrefix(f.Name, "xl/worksheets/sheet") && strings.HasSuffix(f.Name, ".xml") {
				targetFile = f.Name
				sheetTitle = sheetMap[f.Name]
				if sheetTitle == "" {
					sheetTitle = "Sheet1"
				}
				break
			}
		}
	}
	if targetFile == "" {
		return nil, "", fmt.Errorf("no worksheet found")
	}

	var zf *zip.File
	for _, f := range zr.File {
		if f.Name == targetFile {
			zf = f
			break
		}
	}
	if zf == nil {
		return nil, "", fmt.Errorf("worksheet xml missing")
	}
	rc, err := zf.Open()
	if err != nil {
		return nil, "", err
	}
	defer rc.Close()
	matrix, err := readMatrixFromSheetXML(rc, shared)
	return matrix, sheetTitle, err
}

// readMatrixFromSheetXML streams through <sheetData> XML.
// Each <c> element has r="A1" (cell ref) and optionally t="s" (shared string).
// We build a sparse map[row][col] then flatten to a dense [][]string.
func readMatrixFromSheetXML(r io.Reader, shared []string) ([][]string, error) {
	dec := xml.NewDecoder(r)
	rowMap := map[int]map[int]string{}
	maxR, maxC := -1, -1
	var cellType, cellRef string
	var inV bool
	var valBuilder strings.Builder

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "c" {
				cellRef = ""
				cellType = ""
				for _, a := range t.Attr {
					if a.Name.Local == "r" {
						cellRef = a.Value
					}
					if a.Name.Local == "t" {
						cellType = a.Value
					}
				}
			}
			if t.Name.Local == "v" {
				inV = true
				valBuilder.Reset()
			}
		case xml.EndElement:
			if t.Name.Local == "v" {
				inV = false
				raw := valBuilder.String()
				row, col, ok := parseCellRef(cellRef)
				if !ok {
					continue
				}
				val := raw
				if cellType == "s" {
					idx, _ := strconv.Atoi(raw)
					if idx >= 0 && idx < len(shared) {
						val = shared[idx]
					}
				}
				if rowMap[row] == nil {
					rowMap[row] = map[int]string{}
				}
				rowMap[row][col] = val
				if row > maxR {
					maxR = row
				}
				if col > maxC {
					maxC = col
				}
			}
		case xml.CharData:
			if inV {
				valBuilder.Write(t)
			}
		}
	}
	if maxR < 0 {
		return [][]string{}, nil
	}
	out := make([][]string, maxR+1)
	for r := 0; r <= maxR; r++ {
		row := make([]string, maxC+1)
		if rowMap[r] != nil {
			for c := 0; c <= maxC; c++ {
				row[c] = rowMap[r][c]
			}
		}
		out[r] = row
	}
	return out, nil
}

// parseCellRef turns "B3" into (row=2, col=1). Returns zero-indexed.
func parseCellRef(ref string) (row, col int, ok bool) {
	i := 0
	for i < len(ref) && ref[i] >= 'A' && ref[i] <= 'Z' {
		i++
	}
	if i == 0 || i >= len(ref) {
		return 0, 0, false
	}
	col = excelColToIndex(ref[:i])
	rn, err := strconv.Atoi(ref[i:])
	if err != nil {
		return 0, 0, false
	}
	return rn - 1, col, true
}

// excelColToIndex: "A"->0, "B"->1, "AA"->26, etc.
func excelColToIndex(col string) int {
	idx := 0
	for _, ch := range col {
		idx = idx*26 + int(ch-'A'+1)
	}
	return idx - 1
}

// xlsxLoadSharedStrings reads xl/sharedStrings.xml — the string interning table.
// Cells with t="s" store an index into this table rather than inline text.
func xlsxLoadSharedStrings(zr *zip.Reader) []string {
	for _, f := range zr.File {
		if f.Name == "xl/sharedStrings.xml" {
			rc, err := f.Open()
			if err != nil {
				return nil
			}
			defer rc.Close()
			return xlsxParseSharedStrings(rc)
		}
	}
	return nil
}

func xlsxParseSharedStrings(r io.Reader) []string {
	decoder := xml.NewDecoder(r)
	var out []string
	var cur strings.Builder
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
				cur.Reset()
			}
		case xml.EndElement:
			if t.Name.Local == "t" {
				inT = false
			}
			if t.Name.Local == "si" {
				out = append(out, cur.String())
			}
		case xml.CharData:
			if inT {
				cur.Write(t)
			}
		}
	}
	return out
}

// xlsxLoadSheetNames maps "xl/worksheets/sheet1.xml" -> "Sheet1" from workbook.xml.
func xlsxLoadSheetNames(zr *zip.Reader) map[string]string {
	result := make(map[string]string)
	for _, f := range zr.File {
		if f.Name != "xl/workbook.xml" {
			continue
		}
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
	return result
}
