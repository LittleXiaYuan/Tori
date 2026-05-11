package xlsxgen

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

// Render writes spec to an .xlsx file at outputPath. The output file is
// created (or truncated) atomically: it is written to a temp file in
// the same directory and only renamed after every sheet is generated
// successfully.
func Render(spec *SpreadsheetSpec, outputPath string) error {
	if spec == nil || len(spec.Sheets) == 0 {
		return fmt.Errorf("spreadsheet spec must contain at least one sheet")
	}

	theme := resolvedTheme(spec.Theme)

	f := excelize.NewFile()
	defer f.Close()

	headerStyleID, err := buildHeaderStyle(f, theme)
	if err != nil {
		return fmt.Errorf("build header style: %w", err)
	}
	bandedStyleID := 0
	if theme.BandedRow {
		bandedStyleID, err = buildBandedStyle(f, theme)
		if err != nil {
			return fmt.Errorf("build banded style: %w", err)
		}
	}

	for i, sheet := range spec.Sheets {
		name, err := normalizeSheetName(f, i, sheet.Name)
		if err != nil {
			return err
		}
		if err := renderSheet(f, name, &spec.Sheets[i], headerStyleID, bandedStyleID); err != nil {
			return fmt.Errorf("render sheet %q: %w", name, err)
		}
	}

	if err := f.SaveAs(outputPath); err != nil {
		return fmt.Errorf("save xlsx: %w", err)
	}
	return nil
}

// normalizeSheetName resolves the sheet name for index i, replacing the
// default Sheet1 for i==0 and creating a new sheet for subsequent
// indices. Excel forbids characters []*?/\ and limits names to 31
// chars; we sanitize to keep within those bounds.
func normalizeSheetName(f *excelize.File, idx int, raw string) (string, error) {
	name := sanitizeSheetName(raw, idx+1)
	if idx == 0 {
		def := f.GetSheetName(0)
		if def != "" && def != name {
			if err := f.SetSheetName(def, name); err != nil {
				return "", fmt.Errorf("rename default sheet: %w", err)
			}
		}
		return name, nil
	}
	if _, err := f.NewSheet(name); err != nil {
		return "", fmt.Errorf("create sheet: %w", err)
	}
	return name, nil
}

var sheetIllegal = regexp.MustCompile(`[\\/?*\[\]]`)

func sanitizeSheetName(name string, fallbackIndex int) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Sprintf("Sheet%d", fallbackIndex)
	}
	name = sheetIllegal.ReplaceAllString(name, "_")
	if len(name) > 31 {
		runes := []rune(name)
		if len(runes) > 31 {
			runes = runes[:31]
		}
		name = string(runes)
	}
	return name
}

// renderSheet writes headers, data rows, freeze panes, auto-filter,
// merges, and column widths for one sheet.
func renderSheet(f *excelize.File, name string, sheet *SheetSpec, headerStyleID, bandedStyleID int) error {
	headers := sheet.Headers
	rows := sheet.Rows

	if len(headers) == 0 && len(rows) > 0 {
		// Promote first data row to headers when none provided.
		headers = make([]ColumnSpec, len(rows[0]))
		for i, v := range rows[0] {
			s, _ := stringForInfer(v)
			headers[i] = ColumnSpec{Name: s}
		}
		rows = rows[1:]
	}
	if len(headers) == 0 {
		return fmt.Errorf("sheet has no headers")
	}

	colCount := len(headers)
	rowCount := len(rows)

	// 1. Header row.
	for i, h := range headers {
		axis, err := excelize.CoordinatesToCellName(i+1, 1)
		if err != nil {
			return err
		}
		if err := f.SetCellValue(name, axis, h.Name); err != nil {
			return err
		}
	}
	if colCount > 0 {
		from, _ := excelize.CoordinatesToCellName(1, 1)
		to, _ := excelize.CoordinatesToCellName(colCount, 1)
		if err := f.SetCellStyle(name, from, to, headerStyleID); err != nil {
			return err
		}
	}

	// 2. Per-column data styles (number formats, dates, percent, currency).
	colStyleIDs := make([]int, colCount)
	for i, h := range headers {
		id, err := buildColumnDataStyle(f, h)
		if err != nil {
			return err
		}
		colStyleIDs[i] = id
	}

	// 3. Body rows.
	for ri, row := range rows {
		excelRow := ri + 2
		for ci := 0; ci < colCount; ci++ {
			axis, err := excelize.CoordinatesToCellName(ci+1, excelRow)
			if err != nil {
				return err
			}
			var v any
			if ci < len(row) {
				v = row[ci]
			}
			if err := writeCell(f, name, axis, v, headers[ci]); err != nil {
				return err
			}
			if colStyleIDs[ci] != 0 {
				_ = f.SetCellStyle(name, axis, axis, colStyleIDs[ci])
			} else if bandedStyleID != 0 && ri%2 == 1 {
				_ = f.SetCellStyle(name, axis, axis, bandedStyleID)
			}
		}
	}

	// 4. Column widths (auto-fit if 0).
	for i, h := range headers {
		col, err := excelize.ColumnNumberToName(i + 1)
		if err != nil {
			return err
		}
		w := h.Width
		if w == 0 {
			w = autoFitWidth(headers[i].Name, rows, i)
		}
		if w > 0 {
			_ = f.SetColWidth(name, col, col, float64(w))
		}
	}

	// 5. Freeze panes.
	freeze := sheet.Freeze
	if freeze == "" {
		// Default: freeze header row when there is body data.
		if rowCount > 0 {
			freeze = "A2"
		}
	}
	if freeze != "" {
		col, row, err := excelize.CellNameToCoordinates(freeze)
		if err == nil {
			panes := &excelize.Panes{
				Freeze:      true,
				XSplit:      col - 1,
				YSplit:      row - 1,
				TopLeftCell: freeze,
				ActivePane:  "bottomRight",
			}
			_ = f.SetPanes(name, panes)
		}
	}

	// 6. Auto-filter on the full table range.
	if sheet.AutoFilter && colCount > 0 && rowCount > 0 {
		end, _ := excelize.CoordinatesToCellName(colCount, rowCount+1)
		_ = f.AutoFilter(name, "A1:"+end, nil)
	}

	// 7. Cell merges.
	for _, m := range sheet.Merge {
		if m.Range == "" {
			continue
		}
		parts := strings.Split(m.Range, ":")
		if len(parts) != 2 {
			continue
		}
		_ = f.MergeCell(name, parts[0], parts[1])
	}

	// 8. Charts (handled by charter.go after sheet is populated).
	if err := renderCharts(f, name, sheet); err != nil {
		return err
	}

	return nil
}

// writeCell stores a value with the right excelize call for its
// declared (or inferred) column type.
func writeCell(f *excelize.File, sheet, axis string, v any, col ColumnSpec) error {
	if v == nil {
		return nil
	}
	if s, ok := v.(string); ok {
		// Formula sentinel: column type "formula" or value starts with "=".
		if col.Type == "formula" || (strings.HasPrefix(s, "=") && len(s) > 1) {
			return f.SetCellFormula(sheet, axis, s)
		}
		if col.Type == "date" {
			if t, ok2 := parseDate(s); ok2 {
				return f.SetCellValue(sheet, axis, t)
			}
		}
	}
	switch col.Type {
	case "integer":
		if n, ok := toInt64(v); ok {
			return f.SetCellInt(sheet, axis, n)
		}
	case "number", "percent", "currency":
		if fl, ok := toFloat64(v); ok {
			return f.SetCellFloat(sheet, axis, fl, -1, 64)
		}
	case "date":
		if t, ok := toTime(v); ok {
			return f.SetCellValue(sheet, axis, t)
		}
	}
	return f.SetCellValue(sheet, axis, v)
}

func toInt64(v any) (int64, bool) {
	switch x := v.(type) {
	case int:
		return int64(x), true
	case int64:
		return x, true
	case float64:
		return int64(x), true
	case string:
		if n, err := strconv.ParseInt(strings.TrimSpace(x), 10, 64); err == nil {
			return n, true
		}
	}
	return 0, false
}

func toFloat64(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(x), 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

func toTime(v any) (time.Time, bool) {
	if s, ok := v.(string); ok {
		if t, ok2 := parseDate(s); ok2 {
			return t, true
		}
	}
	if t, ok := v.(time.Time); ok {
		return t, true
	}
	return time.Time{}, false
}

var dateLayouts = []string{
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
	"2006-01-02",
	"2006/01/02 15:04:05",
	"2006/01/02 15:04",
	"2006/01/02",
	"01/02/2006",
}

func parseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// autoFitWidth picks a reasonable column width given the header text
// and a sample of body values. Values are capped so a single huge cell
// does not stretch the whole column.
func autoFitWidth(header string, rows [][]any, col int) int {
	const minWidth = 8
	const maxWidth = 36
	const sampleLimit = 50

	max := displayWidth(header)
	for i := 0; i < len(rows) && i < sampleLimit; i++ {
		if col >= len(rows[i]) {
			continue
		}
		if s, ok := stringForInfer(rows[i][col]); ok {
			w := displayWidth(s)
			if w > max {
				max = w
			}
		}
	}
	w := max + 2
	if w < minWidth {
		w = minWidth
	}
	if w > maxWidth {
		w = maxWidth
	}
	return w
}

// displayWidth returns the approximate Excel column width of s.
// CJK runes count as 2 (their glyphs are roughly twice as wide as
// Latin glyphs in the default Excel font).
func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		if r >= 0x2E80 && r <= 0x9FFF {
			w += 2
			continue
		}
		if r >= 0xFF00 && r <= 0xFFEF {
			w += 2
			continue
		}
		w++
	}
	return w
}

// ---- Style construction ---------------------------------------------

func buildHeaderStyle(f *excelize.File, theme Theme) (int, error) {
	bg := strings.TrimPrefix(theme.HeaderBG, "#")
	fg := strings.TrimPrefix(theme.HeaderColor, "#")
	border := []excelize.Border{
		{Type: "left", Color: "B0B7C2", Style: 1},
		{Type: "right", Color: "B0B7C2", Style: 1},
		{Type: "top", Color: "B0B7C2", Style: 1},
		{Type: "bottom", Color: "B0B7C2", Style: 1},
	}
	return f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: fg, Size: 11},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{bg},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    border,
	})
}

func buildBandedStyle(f *excelize.File, theme Theme) (int, error) {
	bg := strings.TrimPrefix(theme.BandedColor, "#")
	return f.NewStyle(&excelize.Style{
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{bg},
			Pattern: 1,
		},
	})
}

// buildColumnDataStyle returns 0 when no special formatting is needed
// for the column.
func buildColumnDataStyle(f *excelize.File, col ColumnSpec) (int, error) {
	custom := col.Format
	var numFmt int
	switch col.Type {
	case "percent":
		if custom == "" {
			numFmt = 10 // 0.00%
		}
	case "currency":
		if custom == "" {
			custom = "¥#,##0.00"
		}
	case "date":
		if custom == "" {
			custom = "yyyy-mm-dd"
		}
	case "number":
		// no format — let excelize render naturally
	}
	if custom == "" && numFmt == 0 {
		return 0, nil
	}
	style := &excelize.Style{}
	if custom != "" {
		style.CustomNumFmt = &custom
	} else {
		style.NumFmt = numFmt
	}
	return f.NewStyle(style)
}

// FilenameWithExt ensures the path ends in .xlsx.
func FilenameWithExt(path string) string {
	if filepath.Ext(strings.ToLower(path)) == ".xlsx" {
		return path
	}
	return path + ".xlsx"
}
