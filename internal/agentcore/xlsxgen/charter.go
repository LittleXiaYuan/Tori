package xlsxgen

import (
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

// AutoRecommendCharts inspects every sheet that has no charts and
// appends a single chart recommendation chosen by [recommendChart].
// Sheets with explicit charts are left untouched.
func AutoRecommendCharts(spec *SpreadsheetSpec) {
	if spec == nil {
		return
	}
	for i := range spec.Sheets {
		sheet := &spec.Sheets[i]
		if len(sheet.Charts) > 0 {
			continue
		}
		if c, ok := recommendChart(sheet); ok {
			sheet.Charts = append(sheet.Charts, c)
		}
	}
}

// recommendChart picks one chart type based on the shape of the data.
// Returns ok=false when no useful visualization can be inferred.
func recommendChart(sheet *SheetSpec) (ChartSpec, bool) {
	cols := len(sheet.Headers)
	rowN := len(sheet.Rows)
	if cols < 2 || rowN < 2 {
		return ChartSpec{}, false
	}

	timeCol := -1
	numericCols := []int{}
	for i, h := range sheet.Headers {
		switch h.Type {
		case "date":
			timeCol = i
		case "integer", "number", "percent", "currency":
			numericCols = append(numericCols, i)
		}
	}
	if len(numericCols) == 0 {
		return ChartSpec{}, false
	}

	// Treat the first non-numeric / non-date column as the category axis
	// when no date column is present.
	catCol := timeCol
	if catCol < 0 {
		for i, h := range sheet.Headers {
			if h.Type == "string" || h.Type == "" {
				if !containsInt(numericCols, i) {
					catCol = i
					break
				}
			}
		}
	}
	if catCol < 0 {
		return ChartSpec{}, false
	}

	// Drop the category column from the value series list.
	valueCols := make([]int, 0, len(numericCols))
	for _, c := range numericCols {
		if c != catCol {
			valueCols = append(valueCols, c)
		}
	}
	if len(valueCols) == 0 {
		return ChartSpec{}, false
	}

	chartType := "bar"
	switch {
	case timeCol == catCol:
		chartType = "line"
	case len(valueCols) == 1 && rowN <= 8:
		chartType = "pie"
	case len(valueCols) >= 2:
		chartType = "stacked_bar"
	}

	// Anchor: two columns to the right of the data block.
	anchorCol := cols + 2
	anchor, _ := excelize.ColumnNumberToName(anchorCol)
	dataRange, categoriesRange := buildRanges(sheet.Name, catCol, valueCols, rowN)

	title := sheet.Name + " - "
	switch chartType {
	case "line":
		title += "趋势"
	case "pie":
		title += "占比"
	case "stacked_bar":
		title += "对比 (堆叠)"
	default:
		title += "对比"
	}

	return ChartSpec{
		Type:       chartType,
		Title:      title,
		DataRange:  dataRange,
		Categories: categoriesRange,
		Position:   anchor + "2",
		Width:      560,
		Height:     320,
	}, true
}

// buildRanges returns the data range (covering the value columns) and
// the categories range (the category column body) in A1 notation.
//
// Both ranges include the sheet name so the caller can hand them
// straight to excelize.
func buildRanges(sheetName string, catCol int, valueCols []int, rowN int) (string, string) {
	q := quoteSheetName(sheetName)
	catLetter, _ := excelize.ColumnNumberToName(catCol + 1)
	categories := fmt.Sprintf("%s!$%s$2:$%s$%d", q, catLetter, catLetter, rowN+1)

	if len(valueCols) == 1 {
		v, _ := excelize.ColumnNumberToName(valueCols[0] + 1)
		return fmt.Sprintf("%s!$%s$1:$%s$%d", q, v, v, rowN+1), categories
	}

	// Multi-series: take the contiguous span [minCol .. maxCol]
	min, max := valueCols[0], valueCols[0]
	for _, c := range valueCols {
		if c < min {
			min = c
		}
		if c > max {
			max = c
		}
	}
	startLetter, _ := excelize.ColumnNumberToName(min + 1)
	endLetter, _ := excelize.ColumnNumberToName(max + 1)
	return fmt.Sprintf("%s!$%s$1:$%s$%d", q, startLetter, endLetter, rowN+1), categories
}

// quoteSheetName wraps sheet names that contain spaces or punctuation
// in single quotes for Excel formulas.
func quoteSheetName(name string) string {
	needsQuote := strings.ContainsAny(name, " '!\"#$%&()+,./:;<=>?@[]^_`{|}~")
	if !needsQuote {
		return name
	}
	escaped := strings.ReplaceAll(name, "'", "''")
	return "'" + escaped + "'"
}

func containsInt(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// ---- Rendering ------------------------------------------------------

// renderCharts iterates the chart specs of one sheet and writes each
// chart into the workbook via excelize.AddChart.
func renderCharts(f *excelize.File, sheetName string, sheet *SheetSpec) error {
	for _, c := range sheet.Charts {
		if err := renderChart(f, sheetName, sheet, c); err != nil {
			return fmt.Errorf("render chart %q: %w", c.Type, err)
		}
	}
	return nil
}

func renderChart(f *excelize.File, sheetName string, sheet *SheetSpec, c ChartSpec) error {
	excType, ok := mapChartType(c.Type)
	if !ok {
		return fmt.Errorf("unsupported chart type %q", c.Type)
	}

	pos := c.Position
	if pos == "" {
		anchorCol := len(sheet.Headers) + 2
		anchor, _ := excelize.ColumnNumberToName(anchorCol)
		pos = anchor + "2"
	}
	w, h := c.Width, c.Height
	if w == 0 {
		w = 560
	}
	if h == 0 {
		h = 320
	}

	series, err := buildSeries(sheetName, sheet, c)
	if err != nil {
		return err
	}

	chart := &excelize.Chart{
		Type:   excType,
		Series: series,
		Format: excelize.GraphicOptions{
			ScaleX: 1.0,
			ScaleY: 1.0,
		},
		Dimension: excelize.ChartDimension{
			Width:  uint(w),
			Height: uint(h),
		},
		Title: []excelize.RichTextRun{{Text: c.Title}},
		Legend: excelize.ChartLegend{
			Position: "bottom",
		},
		PlotArea: excelize.ChartPlotArea{
			ShowVal: c.Type == "pie",
		},
	}

	return f.AddChart(sheetName, pos, chart)
}

// mapChartType translates the user-friendly type strings used in
// SpreadsheetSpec into excelize.ChartType constants.
func mapChartType(s string) (excelize.ChartType, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "bar", "column":
		return excelize.Col, true
	case "stacked_bar", "stacked_column":
		return excelize.ColStacked, true
	case "bar_horizontal", "bar_h":
		return excelize.Bar, true
	case "line":
		return excelize.Line, true
	case "pie":
		return excelize.Pie, true
	case "doughnut":
		return excelize.Doughnut, true
	case "scatter":
		return excelize.Scatter, true
	case "area":
		return excelize.Area, true
	case "radar":
		return excelize.Radar, true
	}
	return 0, false
}

// buildSeries converts the chart's data range into one [excelize.ChartSeries]
// per value column. The series name comes from the corresponding
// header cell.
func buildSeries(sheetName string, sheet *SheetSpec, c ChartSpec) ([]excelize.ChartSeries, error) {
	if c.DataRange == "" {
		return nil, fmt.Errorf("chart data_range is required")
	}

	dataSheet, dataRef := splitSheetRef(c.DataRange, sheetName)
	startCol, _, endCol, endRow, err := decodeRange(dataRef)
	if err != nil {
		return nil, err
	}

	categoriesRef := c.Categories
	if categoriesRef == "" {
		// First column inside the range becomes the category axis.
		startLetter, _ := excelize.ColumnNumberToName(startCol)
		categoriesRef = fmt.Sprintf("%s!$%s$2:$%s$%d", quoteSheetName(dataSheet), startLetter, startLetter, endRow)
		startCol++
	}

	if startCol > endCol {
		return nil, fmt.Errorf("chart data_range has no value columns")
	}

	series := make([]excelize.ChartSeries, 0, endCol-startCol+1)
	for col := startCol; col <= endCol; col++ {
		colLetter, _ := excelize.ColumnNumberToName(col)
		nameRef := fmt.Sprintf("%s!$%s$1", quoteSheetName(dataSheet), colLetter)
		valuesRef := fmt.Sprintf("%s!$%s$2:$%s$%d", quoteSheetName(dataSheet), colLetter, colLetter, endRow)
		series = append(series, excelize.ChartSeries{
			Name:       nameRef,
			Categories: categoriesRef,
			Values:     valuesRef,
		})
	}
	return series, nil
}

// splitSheetRef splits "Sheet1!$A$1:$D$10" into ("Sheet1", "$A$1:$D$10").
// When ref already lacks a sheet prefix, the fallback name is used.
func splitSheetRef(ref, fallback string) (string, string) {
	ref = strings.TrimSpace(ref)
	if i := strings.LastIndex(ref, "!"); i > 0 {
		name := strings.TrimSpace(ref[:i])
		name = strings.Trim(name, "'")
		return name, ref[i+1:]
	}
	return fallback, ref
}

// decodeRange parses "$A$1:$D$10" (or "A1:D10") into 1-based column
// indices and 1-based row numbers.
func decodeRange(ref string) (startCol, startRow, endCol, endRow int, err error) {
	clean := strings.ReplaceAll(ref, "$", "")
	parts := strings.Split(clean, ":")
	if len(parts) != 2 {
		return 0, 0, 0, 0, fmt.Errorf("invalid range %q", ref)
	}
	startCol, startRow, err = excelize.CellNameToCoordinates(parts[0])
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("invalid start cell %q: %w", parts[0], err)
	}
	endCol, endRow, err = excelize.CellNameToCoordinates(parts[1])
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("invalid end cell %q: %w", parts[1], err)
	}
	return startCol, startRow, endCol, endRow, nil
}
