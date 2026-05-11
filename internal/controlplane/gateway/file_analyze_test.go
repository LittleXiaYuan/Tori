package gateway

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

func TestTryParseFilePPTX(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("ppt/slides/slide1.xml")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <p:cSld><p:spTree><p:sp><p:txBody>
    <a:p><a:r><a:t>云雀 Agent</a:t></a:r></a:p>
    <a:p><a:r><a:t>创业赋能中心汇报</a:t></a:r></a:p>
  </p:txBody></p:sp></p:spTree></p:cSld>
</p:sld>`))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	got := TryParseFile("demo.pptx", buf.Bytes())
	if !strings.Contains(got, "[Slide 1]") || !strings.Contains(got, "云雀 Agent") || !strings.Contains(got, "创业赋能中心汇报") {
		t.Fatalf("unexpected pptx parse result: %q", got)
	}
}

func TestTryParseFileDOCXTableAndParagraphs(t *testing.T) {
	got := TryParseFile("申请表.docx", testDOCXBytes(t))
	for _, want := range []string{"青岛市创业赋能中心", "公司名称\t云鸢科技", "联系电话\t已填写"} {
		if !strings.Contains(got, want) {
			t.Fatalf("unexpected docx parse result, missing %q in %q", want, got)
		}
	}
}

func TestTryParseFileXLSXInlineStrings(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("xl/workbook.xml")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheets><sheet name="预算" sheetId="1" r:id="rId1"/></sheets>
</workbook>`))
	w, err = zw.Create("xl/worksheets/sheet1.xml")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData>
    <row r="1"><c r="A1" t="inlineStr"><is><t>项目</t></is></c><c r="B1" t="inlineStr"><is><t>金额</t></is></c></row>
    <row r="2"><c r="A2" t="inlineStr"><is><t>云雀 Agent</t></is></c><c r="B2"><v>128</v></c></row>
  </sheetData>
</worksheet>`))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	got := TryParseFile("budget.xlsx", buf.Bytes())
	if !strings.Contains(got, "[Sheet: 预算]") || !strings.Contains(got, "项目\t金额") || !strings.Contains(got, "云雀 Agent\t128") {
		t.Fatalf("unexpected xlsx parse result: %q", got)
	}
}

func TestTryParseFileResultDocumentParserNeededDoesNotFakePreview(t *testing.T) {
	for _, name := range []string{"申请表.pdf", "旧版合同.doc", "路演材料.ppt", "预算.xls"} {
		got := TryParseFileResult(name, []byte{0x00, 0x01, 0x02, 0x03})
		if got.Preview != "" {
			t.Fatalf("%s should not expose placeholder as preview: %q", name, got.Preview)
		}
		if got.Status != "needs_document_parser" {
			t.Fatalf("%s status = %q", name, got.Status)
		}
		if got.Note == "" || !strings.Contains(got.Note, "附件已添加") || !strings.Contains(got.Note, "文档解析") {
			t.Fatalf("%s note should be user-facing and actionable: %q", name, got.Note)
		}
	}
}

func TestFileParseMetadataTruncatesPreviewAndKeepsStatus(t *testing.T) {
	preview := strings.Repeat("云雀", 10)
	meta := fileParseMetadata(FileParseResult{
		Preview:       preview,
		Parser:        "local",
		Backend:       "go",
		MarkdownChars: len([]rune(preview)),
		Status:        "parsed",
	}, 6)
	if meta == nil {
		t.Fatal("expected metadata")
	}
	gotPreview, _ := meta["preview"].(string)
	if !strings.HasSuffix(gotPreview, "...已截断") || strings.ContainsRune(gotPreview, '\uFFFD') {
		t.Fatalf("preview should be rune-safe truncated, got %q", gotPreview)
	}
	if got := meta["status"]; got != "parsed" {
		t.Fatalf("expected status parsed, got %#v", got)
	}
	if got := meta["markdown_chars"]; got != 20 {
		t.Fatalf("expected rune count 20, got %#v", got)
	}
}
