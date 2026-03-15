package general

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yunque-agent/pkg/skills"
)

func TestDocParseSkill_Name(t *testing.T) {
	s := NewDocParseSkill(nil)
	if s.Name() != "doc_parse" {
		t.Fatalf("expected doc_parse, got %s", s.Name())
	}
}

func TestDocParseSkill_UnsupportedFormat(t *testing.T) {
	s := NewDocParseSkill(nil)
	tmp := filepath.Join(t.TempDir(), "test.bin")
	os.WriteFile(tmp, []byte("binary"), 0644)

	_, err := s.Execute(context.Background(), map[string]any{"path": tmp}, &skills.Environment{})
	if err == nil || !strings.Contains(err.Error(), "unsupported format") {
		t.Fatalf("expected unsupported format error, got %v", err)
	}
}

func TestDocParseSkill_TextFile(t *testing.T) {
	s := NewDocParseSkill(nil)
	tmp := filepath.Join(t.TempDir(), "hello.txt")
	os.WriteFile(tmp, []byte("Hello World\nLine 2"), 0644)

	result, err := s.Execute(context.Background(), map[string]any{"path": tmp}, &skills.Environment{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Hello World") {
		t.Fatalf("expected Hello World in result, got %s", result)
	}
	if !strings.Contains(result, "Line 2") {
		t.Fatalf("expected Line 2 in result")
	}
	if !strings.Contains(result, ".txt") {
		t.Fatalf("expected format info")
	}
}

func TestDocParseSkill_CSV(t *testing.T) {
	s := NewDocParseSkill(nil)
	tmp := filepath.Join(t.TempDir(), "data.csv")
	os.WriteFile(tmp, []byte("name,age\nAlice,30\nBob,25"), 0644)

	result, err := s.Execute(context.Background(), map[string]any{"path": tmp}, &skills.Environment{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Alice") || !strings.Contains(result, "Bob") {
		t.Fatalf("expected CSV data in result, got %s", result)
	}
	if !strings.Contains(result, "3 rows") {
		t.Fatalf("expected row count")
	}
}

func TestDocParseSkill_Markdown(t *testing.T) {
	s := NewDocParseSkill(nil)
	tmp := filepath.Join(t.TempDir(), "readme.md")
	os.WriteFile(tmp, []byte("# Title\n\nSome **bold** text."), 0644)

	result, err := s.Execute(context.Background(), map[string]any{"path": tmp}, &skills.Environment{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "# Title") {
		t.Fatalf("expected markdown content")
	}
}

func TestDocParseSkill_DOCX(t *testing.T) {
	s := NewDocParseSkill(nil)
	tmp := filepath.Join(t.TempDir(), "test.docx")
	createTestDOCX(t, tmp, "Hello from DOCX")

	result, err := s.Execute(context.Background(), map[string]any{"path": tmp}, &skills.Environment{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Hello from DOCX") {
		t.Fatalf("expected DOCX content, got %s", result)
	}
}

func TestDocParseSkill_XLSX(t *testing.T) {
	s := NewDocParseSkill(nil)
	tmp := filepath.Join(t.TempDir(), "test.xlsx")
	createTestXLSX(t, tmp)

	result, err := s.Execute(context.Background(), map[string]any{"path": tmp}, &skills.Environment{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Alice") || !strings.Contains(result, "Bob") {
		t.Fatalf("expected XLSX data, got %s", result)
	}
}

func TestDocParseSkill_MaxChars(t *testing.T) {
	s := NewDocParseSkill(nil)
	tmp := filepath.Join(t.TempDir(), "long.txt")
	os.WriteFile(tmp, []byte(strings.Repeat("x", 1000)), 0644)

	result, err := s.Execute(context.Background(), map[string]any{
		"path":      tmp,
		"max_chars": float64(50),
	}, &skills.Environment{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "truncated") {
		t.Fatalf("expected truncation notice")
	}
}

func TestDocParseSkill_PathSecurity(t *testing.T) {
	s := NewDocParseSkill([]string{"/allowed/only"})
	tmp := filepath.Join(t.TempDir(), "secret.txt")
	os.WriteFile(tmp, []byte("secret"), 0644)

	_, err := s.Execute(context.Background(), map[string]any{"path": tmp}, &skills.Environment{})
	if err == nil || !strings.Contains(err.Error(), "access denied") {
		t.Fatalf("expected access denied, got %v", err)
	}
}

func TestDocParseSkill_FileNotFound(t *testing.T) {
	s := NewDocParseSkill(nil)
	_, err := s.Execute(context.Background(), map[string]any{"path": "/nonexistent/file.txt"}, &skills.Environment{})
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

// --- helpers ---

func createTestDOCX(t *testing.T, path, text string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)

	// [Content_Types].xml
	ct, _ := zw.Create("[Content_Types].xml")
	ct.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="xml" ContentType="application/xml"/>
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
</Types>`))

	// word/document.xml
	doc, _ := zw.Create("word/document.xml")
	escaped := xmlEscape(text)
	doc.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>` + escaped + `</w:t></w:r></w:p>
  </w:body>
</w:document>`))

	zw.Close()
}

func createTestXLSX(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)

	// [Content_Types].xml
	ct, _ := zw.Create("[Content_Types].xml")
	ct.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="xml" ContentType="application/xml"/>
</Types>`))

	// xl/workbook.xml
	wb, _ := zw.Create("xl/workbook.xml")
	wb.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheets><sheet name="Sheet1" sheetId="1"/></sheets>
</workbook>`))

	// xl/sharedStrings.xml
	ss, _ := zw.Create("xl/sharedStrings.xml")
	ss.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <si><t>Name</t></si>
  <si><t>Age</t></si>
  <si><t>Alice</t></si>
  <si><t>Bob</t></si>
</sst>`))

	// xl/worksheets/sheet1.xml
	sh, _ := zw.Create("xl/worksheets/sheet1.xml")
	sh.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData>
    <row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="s"><v>1</v></c></row>
    <row r="2"><c r="A2" t="s"><v>2</v></c><c r="B2"><v>30</v></c></row>
    <row r="3"><c r="A3" t="s"><v>3</v></c><c r="B3"><v>25</v></c></row>
  </sheetData>
</worksheet>`))

	zw.Close()
}

func xmlEscape(s string) string {
	var b strings.Builder
	xml.EscapeText(&b, []byte(s))
	return b.String()
}
