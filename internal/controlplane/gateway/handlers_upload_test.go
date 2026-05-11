package gateway

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleFileUploadReturnsParsedPreviewMetadataForXLSX(t *testing.T) {
	gw := &Gateway{}
	body, contentType := multipartUploadBody(t, "file", "预算.xlsx", testXLSXBytes(t))

	req := httptest.NewRequest(http.MethodPost, "/v1/upload", body)
	req.Header.Set("Content-Type", contentType)
	req = req.WithContext(ctxWithTenant(req.Context(), "t1"))
	w := httptest.NewRecorder()
	gw.handleFileUpload(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	parse, ok := resp["parse"].(map[string]any)
	if !ok {
		t.Fatalf("expected parse metadata, got %#v", resp["parse"])
	}
	if parse["status"] != "parsed" || parse["parser"] != "local" {
		t.Fatalf("unexpected parse metadata: %#v", parse)
	}
	preview, _ := parse["preview"].(string)
	for _, want := range []string{"[Sheet: 预算]", "项目\t金额", "云雀 Agent\t128"} {
		if !strings.Contains(preview, want) {
			t.Fatalf("expected upload parse preview to contain %q, got %q", want, preview)
		}
	}
}

func TestHandleFileUploadReturnsParsedPreviewMetadataForDOCX(t *testing.T) {
	gw := &Gateway{}
	body, contentType := multipartUploadBody(t, "file", "申请表.docx", testDOCXBytes(t))

	req := httptest.NewRequest(http.MethodPost, "/v1/upload", body)
	req.Header.Set("Content-Type", contentType)
	req = req.WithContext(ctxWithTenant(req.Context(), "t1"))
	w := httptest.NewRecorder()
	gw.handleFileUpload(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	parse := uploadParseMetadata(t, w.Body.Bytes())
	if parse["status"] != "parsed" || parse["parser"] != "local" {
		t.Fatalf("unexpected parse metadata: %#v", parse)
	}
	preview, _ := parse["preview"].(string)
	for _, want := range []string{"青岛市创业赋能中心", "公司名称\t云鸢科技", "联系电话\t已填写"} {
		if !strings.Contains(preview, want) {
			t.Fatalf("expected upload docx parse preview to contain %q, got %q", want, preview)
		}
	}
}

func TestHandleFileUploadReturnsParsedPreviewMetadataForPPTX(t *testing.T) {
	gw := &Gateway{}
	body, contentType := multipartUploadBody(t, "file", "路演材料.pptx", testPPTXBytes(t))

	req := httptest.NewRequest(http.MethodPost, "/v1/upload", body)
	req.Header.Set("Content-Type", contentType)
	req = req.WithContext(ctxWithTenant(req.Context(), "t1"))
	w := httptest.NewRecorder()
	gw.handleFileUpload(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	parse := uploadParseMetadata(t, w.Body.Bytes())
	if parse["status"] != "parsed" || parse["parser"] != "local" {
		t.Fatalf("unexpected parse metadata: %#v", parse)
	}
	preview, _ := parse["preview"].(string)
	for _, want := range []string{"[Slide 1]", "云雀 Agent", "创业赋能中心汇报"} {
		if !strings.Contains(preview, want) {
			t.Fatalf("expected upload pptx parse preview to contain %q, got %q", want, preview)
		}
	}
}

func TestHandleFileUploadReturnsParserNeededMetadataWithoutFakePreview(t *testing.T) {
	gw := &Gateway{}
	body, contentType := multipartUploadBody(t, "file", "申请表.pdf", []byte{0x25, 0x50, 0x44, 0x46})

	req := httptest.NewRequest(http.MethodPost, "/v1/upload", body)
	req.Header.Set("Content-Type", contentType)
	req = req.WithContext(ctxWithTenant(req.Context(), "t1"))
	w := httptest.NewRecorder()
	gw.handleFileUpload(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	parse, ok := resp["parse"].(map[string]any)
	if !ok {
		t.Fatalf("expected parse metadata, got %#v", resp["parse"])
	}
	if parse["status"] != "needs_document_parser" || parse["parser"] != "document" {
		t.Fatalf("unexpected parse metadata: %#v", parse)
	}
	if preview, _ := parse["preview"].(string); preview != "" {
		t.Fatalf("pdf upload should not fake parsed preview, got %q", preview)
	}
	note, _ := parse["note"].(string)
	if !strings.Contains(note, "附件已添加") || !strings.Contains(note, "文档解析") {
		t.Fatalf("expected actionable parse note, got %q", note)
	}
}

func uploadParseMetadata(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var resp map[string]any
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatal(err)
	}
	parse, ok := resp["parse"].(map[string]any)
	if !ok {
		t.Fatalf("expected parse metadata, got %#v", resp["parse"])
	}
	return parse
}

func multipartUploadBody(t *testing.T, fieldName, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return body, writer.FormDataContentType()
}

func testDOCXBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>青岛市创业赋能中心</w:t></w:r></w:p>
    <w:tbl>
      <w:tr>
        <w:tc><w:p><w:r><w:t>公司名称</w:t></w:r></w:p></w:tc>
        <w:tc><w:p><w:r><w:t>云鸢科技</w:t></w:r></w:p></w:tc>
      </w:tr>
      <w:tr>
        <w:tc><w:p><w:r><w:t>联系电话</w:t></w:r></w:p></w:tc>
        <w:tc><w:p><w:r><w:t>已填写</w:t></w:r></w:p></w:tc>
      </w:tr>
    </w:tbl>
  </w:body>
</w:document>`))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func testPPTXBytes(t *testing.T) []byte {
	t.Helper()
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
	return buf.Bytes()
}

func testXLSXBytes(t *testing.T) []byte {
	t.Helper()
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
	return buf.Bytes()
}
