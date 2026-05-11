package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestHandleFilePreviewIncludesParseMetadataForDocumentParserNeeded(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/申请表.pdf", []byte{0x25, 0x50, 0x44, 0x46}, 0o600); err != nil {
		t.Fatal(err)
	}
	g := &Gateway{outputDir: dir}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/files/preview?path=%E7%94%B3%E8%AF%B7%E8%A1%A8.pdf", nil)
	g.handleFilePreview(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if preview, _ := resp["preview"].(string); preview != "" {
		t.Fatalf("pdf preview should be empty without document parser, got %q", preview)
	}
	parse, ok := resp["parse"].(map[string]any)
	if !ok {
		t.Fatalf("expected parse metadata, got %#v", resp["parse"])
	}
	if parse["status"] != "needs_document_parser" {
		t.Fatalf("expected needs_document_parser, got %#v", parse["status"])
	}
	note, _ := parse["note"].(string)
	if !strings.Contains(note, "附件已添加") || !strings.Contains(note, "文档解析") {
		t.Fatalf("expected actionable parse note, got %q", note)
	}
}
