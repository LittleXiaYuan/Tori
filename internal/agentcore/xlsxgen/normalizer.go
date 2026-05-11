package xlsxgen

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Normalize converts arbitrary user input into a [SpreadsheetSpec].
//
// The format argument may be "csv", "tsv", "json", "markdown" (or "md"),
// or "auto" / "" (the default) for content sniffing.
//
// defaultSheetName is used when the input does not specify a sheet name
// (i.e. flat CSV/TSV/Markdown). For full SpreadsheetSpec JSON the sheet
// names come from the input.
func Normalize(format, data, defaultSheetName string) (*SpreadsheetSpec, error) {
	if defaultSheetName == "" {
		defaultSheetName = "Sheet1"
	}
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" || format == "auto" {
		format = sniffFormat(data)
	}

	switch format {
	case "csv":
		return parseDelimited(data, ',', defaultSheetName)
	case "tsv":
		return parseDelimited(data, '\t', defaultSheetName)
	case "json":
		return parseJSON(data, defaultSheetName)
	case "md", "markdown":
		return parseMarkdown(data, defaultSheetName)
	default:
		return nil, fmt.Errorf("unsupported format %q (want csv|tsv|json|markdown|auto)", format)
	}
}

// sniffFormat inspects data and returns a best-guess format string.
// Defaults to "csv" when nothing definitive is found.
func sniffFormat(data string) string {
	s := strings.TrimLeftFunc(data, isSpaceRune)
	if s == "" {
		return "csv"
	}
	switch s[0] {
	case '{', '[':
		return "json"
	case '|':
		return "markdown"
	}
	first := firstLine(data)
	if strings.Contains(first, "\t") {
		return "tsv"
	}
	if strings.HasPrefix(strings.TrimSpace(first), "|") && strings.HasSuffix(strings.TrimSpace(first), "|") {
		return "markdown"
	}
	return "csv"
}

func isSpaceRune(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

func firstLine(s string) string {
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		return s[:i]
	}
	return s
}

// parseDelimited parses CSV/TSV input. The first row is treated as the
// header; remaining rows become data.
func parseDelimited(data string, sep rune, sheetName string) (*SpreadsheetSpec, error) {
	r := csv.NewReader(strings.NewReader(data))
	r.Comma = sep
	r.LazyQuotes = true
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true

	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("delimited parse: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("input is empty")
	}

	headers := make([]ColumnSpec, len(records[0]))
	for i, h := range records[0] {
		headers[i] = ColumnSpec{Name: strings.TrimSpace(h)}
	}

	rows := make([][]any, 0, len(records)-1)
	for _, rec := range records[1:] {
		row := make([]any, len(headers))
		for i := range headers {
			if i < len(rec) {
				row[i] = strings.TrimSpace(rec[i])
			}
		}
		rows = append(rows, row)
	}

	spec := &SpreadsheetSpec{
		Sheets: []SheetSpec{{
			Name:    sheetName,
			Headers: headers,
			Rows:    rows,
		}},
	}
	InferTypes(spec)
	CoerceValues(spec)
	return spec, nil
}

// parseJSON accepts three shapes:
//
//  1. A complete SpreadsheetSpec JSON: {"sheets":[{...}]}.
//  2. A flat array of records: [{"a":1,"b":2}, {"a":3,"b":4}].
//  3. A wrapper object with a "rows" array: {"rows":[{...}]}.
func parseJSON(data, sheetName string) (*SpreadsheetSpec, error) {
	data = strings.TrimSpace(data)
	if data == "" {
		return nil, fmt.Errorf("input is empty")
	}

	dec := json.NewDecoder(bytes.NewReader([]byte(data)))
	dec.UseNumber()

	var raw any
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("json parse: %w", err)
	}

	switch v := raw.(type) {
	case map[string]any:
		if _, ok := v["sheets"]; ok {
			spec := &SpreadsheetSpec{}
			if err := json.Unmarshal([]byte(data), spec); err != nil {
				return nil, fmt.Errorf("json parse spreadsheet: %w", err)
			}
			if len(spec.Sheets) == 0 {
				return nil, fmt.Errorf("sheets array is empty")
			}
			InferTypes(spec)
			CoerceValues(spec)
			return spec, nil
		}
		if rowsRaw, ok := v["rows"]; ok {
			arr, ok2 := rowsRaw.([]any)
			if !ok2 {
				return nil, fmt.Errorf("'rows' must be an array")
			}
			return specFromRecords(arr, sheetName)
		}
		// Single-record object becomes a one-row sheet.
		return specFromRecords([]any{v}, sheetName)
	case []any:
		return specFromRecords(v, sheetName)
	default:
		return nil, fmt.Errorf("unsupported JSON top-level type")
	}
}

// specFromRecords builds a SpreadsheetSpec from an array of records.
// The header order follows the keys of the first object that has them
// (subsequent objects may add new keys, which are appended).
func specFromRecords(records []any, sheetName string) (*SpreadsheetSpec, error) {
	if len(records) == 0 {
		return nil, fmt.Errorf("records array is empty")
	}

	headerOrder := []string{}
	headerSeen := map[string]bool{}
	for _, rec := range records {
		obj, ok := rec.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("each record must be an object, got %T", rec)
		}
		// Preserve insertion order of first occurrence.
		// Go maps don't preserve order, so we rely on the JSON decoder's
		// behavior: unmarshalling into map[string]any does NOT preserve
		// order. To get a stable header order for typical inputs, we
		// sort alphabetically on the first pass and keep that order.
		// Users who care about column order should pass a
		// SpreadsheetSpec directly.
		for k := range obj {
			if !headerSeen[k] {
				headerSeen[k] = true
				headerOrder = append(headerOrder, k)
			}
		}
	}
	sortStringsStable(headerOrder)

	headers := make([]ColumnSpec, len(headerOrder))
	for i, name := range headerOrder {
		headers[i] = ColumnSpec{Name: name}
	}

	rows := make([][]any, 0, len(records))
	for _, rec := range records {
		obj := rec.(map[string]any)
		row := make([]any, len(headerOrder))
		for i, key := range headerOrder {
			if val, ok := obj[key]; ok {
				row[i] = val
			}
		}
		rows = append(rows, row)
	}

	spec := &SpreadsheetSpec{
		Sheets: []SheetSpec{{
			Name:    sheetName,
			Headers: headers,
			Rows:    rows,
		}},
	}
	InferTypes(spec)
	CoerceValues(spec)
	return spec, nil
}

// sortStringsStable performs an insertion sort to keep the order
// stable for inputs that already happen to be sorted.
func sortStringsStable(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// parseMarkdown extracts the first GitHub-Flavored Markdown table found
// in data. Surrounding prose is ignored.
func parseMarkdown(data, sheetName string) (*SpreadsheetSpec, error) {
	lines := strings.Split(data, "\n")

	var headerLine, sepLine string
	bodyStart := -1
	for i := 0; i < len(lines)-1; i++ {
		l := strings.TrimSpace(lines[i])
		next := strings.TrimSpace(lines[i+1])
		if isMDTableRow(l) && isMDTableSep(next) {
			headerLine = l
			sepLine = next
			bodyStart = i + 2
			break
		}
	}
	if bodyStart < 0 {
		return nil, fmt.Errorf("no markdown table found")
	}
	_ = sepLine

	headers := splitMDRow(headerLine)
	if len(headers) == 0 {
		return nil, fmt.Errorf("markdown table has empty header")
	}

	headerSpecs := make([]ColumnSpec, len(headers))
	for i, h := range headers {
		headerSpecs[i] = ColumnSpec{Name: h}
	}

	rows := make([][]any, 0)
	for i := bodyStart; i < len(lines); i++ {
		l := strings.TrimSpace(lines[i])
		if !isMDTableRow(l) {
			break
		}
		cells := splitMDRow(l)
		row := make([]any, len(headers))
		for j := range headers {
			if j < len(cells) {
				row[j] = cells[j]
			}
		}
		rows = append(rows, row)
	}

	spec := &SpreadsheetSpec{
		Sheets: []SheetSpec{{
			Name:    sheetName,
			Headers: headerSpecs,
			Rows:    rows,
		}},
	}
	InferTypes(spec)
	CoerceValues(spec)
	return spec, nil
}

func isMDTableRow(s string) bool {
	return strings.HasPrefix(s, "|") && strings.HasSuffix(s, "|") && len(s) > 1
}

var mdSepRowRe = regexp.MustCompile(`^\s*\|?\s*:?-{3,}:?\s*(\|\s*:?-{3,}:?\s*)+\|?\s*$`)

func isMDTableSep(s string) bool {
	return mdSepRowRe.MatchString(s)
}

func splitMDRow(s string) []string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "|")
	s = strings.TrimSuffix(s, "|")
	cells := strings.Split(s, "|")
	for i, c := range cells {
		cells[i] = strings.TrimSpace(c)
	}
	return cells
}

// ---- Type inference & coercion --------------------------------------

// InferTypes walks every sheet whose ColumnSpec.Type is empty and
// assigns a best-guess type based on the cell values in that column.
// Types already set by the caller are never overwritten.
func InferTypes(spec *SpreadsheetSpec) {
	if spec == nil {
		return
	}
	for si := range spec.Sheets {
		sheet := &spec.Sheets[si]
		if len(sheet.Headers) == 0 {
			continue
		}
		for ci := range sheet.Headers {
			col := &sheet.Headers[ci]
			if col.Type != "" {
				continue
			}
			col.Type = inferColumnType(sheet.Rows, ci)
		}
	}
}

// inferColumnType scans up to 100 sample rows and votes for a column
// type. Empty cells are skipped. The most-common positive guess wins;
// "string" is the fallback when there is no consensus.
func inferColumnType(rows [][]any, col int) string {
	const sampleLimit = 100
	votes := map[string]int{}
	seen := 0
	for i := 0; i < len(rows) && seen < sampleLimit; i++ {
		if col >= len(rows[i]) {
			continue
		}
		v := rows[i][col]
		s, ok := stringForInfer(v)
		if !ok || s == "" {
			continue
		}
		seen++
		votes[guessValueType(s)]++
	}
	if seen == 0 {
		return "string"
	}
	best := "string"
	bestN := 0
	for k, n := range votes {
		if n > bestN || (n == bestN && typeRank(k) > typeRank(best)) {
			best = k
			bestN = n
		}
	}
	if float64(bestN)/float64(seen) < 0.7 {
		return "string"
	}
	return best
}

// stringForInfer returns a printable form of v for type inference.
// Numeric / boolean / json.Number values bypass parsing because their
// types are already known.
func stringForInfer(v any) (string, bool) {
	switch x := v.(type) {
	case nil:
		return "", false
	case string:
		return strings.TrimSpace(x), true
	case json.Number:
		return x.String(), true
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64), true
	case int:
		return strconv.Itoa(x), true
	case int64:
		return strconv.FormatInt(x, 10), true
	case bool:
		return strconv.FormatBool(x), true
	default:
		return fmt.Sprintf("%v", v), true
	}
}

func typeRank(t string) int {
	switch t {
	case "integer":
		return 5
	case "number":
		return 4
	case "percent":
		return 3
	case "currency":
		return 2
	case "date":
		return 1
	}
	return 0
}

var (
	intRe      = regexp.MustCompile(`^[+-]?\d+$`)
	floatRe    = regexp.MustCompile(`^[+-]?(\d+\.\d+|\.\d+|\d+\.)$`)
	percentRe  = regexp.MustCompile(`^[+-]?\d+(\.\d+)?\s*%$`)
	currencyRe = regexp.MustCompile(`^[¥$€£￥]\s*[+-]?\d{1,3}(,\d{3})*(\.\d+)?$|^[+-]?\d+(\.\d+)?\s*(元|RMB|CNY|USD|EUR)$`)
	dateRe     = regexp.MustCompile(`^\d{4}[/-]\d{1,2}[/-]\d{1,2}( \d{1,2}:\d{2}(:\d{2})?)?$|^\d{1,2}/\d{1,2}/\d{4}$`)
)

func guessValueType(s string) string {
	switch {
	case percentRe.MatchString(s):
		return "percent"
	case currencyRe.MatchString(s):
		return "currency"
	case dateRe.MatchString(s):
		return "date"
	case intRe.MatchString(s):
		return "integer"
	case floatRe.MatchString(s):
		return "number"
	}
	return "string"
}

// CoerceValues normalizes string cells to their declared column type
// (parsed numbers, percentages, dates) so the renderer can pass them
// straight to excelize without further conversion.
//
// Unrecognized values are left unchanged so the renderer can fall back
// to writing them as plain strings.
func CoerceValues(spec *SpreadsheetSpec) {
	if spec == nil {
		return
	}
	for si := range spec.Sheets {
		sheet := &spec.Sheets[si]
		for ri := range sheet.Rows {
			for ci := range sheet.Rows[ri] {
				if ci >= len(sheet.Headers) {
					continue
				}
				typ := sheet.Headers[ci].Type
				if typ == "" || typ == "string" {
					continue
				}
				sheet.Rows[ri][ci] = coerceCell(sheet.Rows[ri][ci], typ)
			}
		}
	}
}

func coerceCell(v any, typ string) any {
	s, ok := stringForInfer(v)
	if !ok {
		return v
	}
	switch typ {
	case "integer":
		if n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil {
			return n
		}
	case "number":
		if f, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
			return f
		}
	case "percent":
		t := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "%"))
		if f, err := strconv.ParseFloat(t, 64); err == nil {
			return f / 100.0
		}
	case "currency":
		t := stripCurrency(s)
		if f, err := strconv.ParseFloat(t, 64); err == nil {
			return f
		}
	case "formula":
		return s
	case "date":
		// Leave as string; renderer applies date format and cell style.
		return s
	}
	return v
}

func stripCurrency(s string) string {
	s = strings.TrimSpace(s)
	for _, p := range []string{"¥", "$", "€", "£", "￥"} {
		s = strings.TrimPrefix(s, p)
	}
	for _, suf := range []string{"元", "RMB", "CNY", "USD", "EUR"} {
		s = strings.TrimSuffix(s, suf)
	}
	s = strings.ReplaceAll(s, ",", "")
	return strings.TrimSpace(s)
}
