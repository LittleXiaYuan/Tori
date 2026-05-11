// Package xlsxgen builds production-grade Excel workbooks from
// heterogeneous user inputs (CSV / JSON / Markdown / TSV / native Spec).
//
// The package is organized as a four-stage pipeline:
//
//	Input  ─►  Normalizer  ─►  Enhancer  ─►  Renderer  ─►  *.xlsx
//	             (parse)        (typing,        (excelize
//	                              charts)         output)
//
// All stages operate on the unified [SpreadsheetSpec] intermediate
// representation. The package is pure-Go and depends only on
// github.com/xuri/excelize/v2.
package xlsxgen

// SpreadsheetSpec is the unified intermediate representation for a
// workbook. It can be produced by the Normalizer (from CSV/JSON/etc.)
// or hand-crafted by callers that already know the exact shape they
// want.
type SpreadsheetSpec struct {
	Title  string      `json:"title,omitempty"`
	Author string      `json:"author,omitempty"`
	Theme  *Theme      `json:"theme,omitempty"`
	Sheets []SheetSpec `json:"sheets"`
}

// Theme controls the visual style applied to every sheet.
// Zero values fall back to a sensible "Yunque Blue" default.
type Theme struct {
	HeaderBG    string `json:"header_bg,omitempty"`    // header fill, default "#2B579A"
	HeaderColor string `json:"header_color,omitempty"` // header text, default "#FFFFFF"
	BandedRow   bool   `json:"banded_row,omitempty"`   // alternate row tint
	BandedColor string `json:"banded_color,omitempty"` // tint color, default "#F2F6FC"
}

// SheetSpec describes a single worksheet.
type SheetSpec struct {
	Name       string       `json:"name"`
	Headers    []ColumnSpec `json:"headers,omitempty"` // optional; if empty, Rows[0] is used as header
	Rows       [][]any      `json:"rows"`              // body rows (excluding header)
	Charts     []ChartSpec  `json:"charts,omitempty"`
	Freeze     string       `json:"freeze,omitempty"`      // e.g. "A2" freezes header row
	AutoFilter bool         `json:"auto_filter,omitempty"` // adds AutoFilter to the table
	Merge      []MergeSpec  `json:"merge,omitempty"`
}

// ColumnSpec describes a single column header and its data type.
//
// Supported Type values:
//
//	""          -> auto detect (default)
//	"string"    -> kept as text
//	"integer"   -> stored as int64
//	"number"    -> stored as float64
//	"percent"   -> stored as float64 with "0.00%" format
//	"currency"  -> stored as float64 with "¥#,##0.00" format
//	"date"      -> stored as Excel date with "yyyy-mm-dd" format
//	"formula"   -> cell value beginning with "=" is set as formula
type ColumnSpec struct {
	Name   string `json:"name"`
	Type   string `json:"type,omitempty"`
	Format string `json:"format,omitempty"` // explicit number format overrides Type default
	Width  int    `json:"width,omitempty"`  // column width in characters; 0 = auto-fit
}

// ChartSpec describes a chart embedded in a sheet.
//
// Supported Type values: "bar", "line", "pie", "scatter",
// "stacked_bar", "area".
//
// Data ranges use Excel A1 notation; sheet names are filled in
// automatically by the renderer when omitted.
type ChartSpec struct {
	Type       string `json:"type"`
	Title      string `json:"title,omitempty"`
	DataRange  string `json:"data_range,omitempty"`  // e.g. "$B$1:$D$10"
	Categories string `json:"categories,omitempty"` // e.g. "$A$2:$A$10"
	Position   string `json:"position,omitempty"`   // e.g. "F2"
	Width      int    `json:"width,omitempty"`      // pixels
	Height     int    `json:"height,omitempty"`     // pixels
}

// MergeSpec describes a merged cell range, e.g. "A1:C1".
type MergeSpec struct {
	Range string `json:"range"`
}

// Theme defaults used when fields are empty.
const (
	DefaultHeaderBG    = "#2B579A"
	DefaultHeaderColor = "#FFFFFF"
	DefaultBandedColor = "#F2F6FC"
)

func resolvedTheme(t *Theme) Theme {
	out := Theme{
		HeaderBG:    DefaultHeaderBG,
		HeaderColor: DefaultHeaderColor,
		BandedColor: DefaultBandedColor,
	}
	if t == nil {
		return out
	}
	if t.HeaderBG != "" {
		out.HeaderBG = t.HeaderBG
	}
	if t.HeaderColor != "" {
		out.HeaderColor = t.HeaderColor
	}
	out.BandedRow = t.BandedRow
	if t.BandedColor != "" {
		out.BandedColor = t.BandedColor
	}
	return out
}
