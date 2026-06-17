package knowledgepack

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"yunque-agent/internal/agentcore/knowledge"
	"yunque-agent/internal/integrations/mineru"
)

type uploadParser struct {
	enabled bool
	result  *mineru.ParseResult
	err     error
}

func (p uploadParser) Enabled() bool { return p.enabled }

func (p uploadParser) ParseFile(ctx context.Context, filePath string) (*mineru.ParseResult, error) {
	return p.result, p.err
}

type uploadGateway struct {
	*fakeKnowledgeGateway
	parser knowledge.DocumentParser
}

func (g uploadGateway) DocumentParser() knowledge.DocumentParser { return g.parser }

func multipartKnowledgeUpload(t *testing.T, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
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

func TestUploadNativeTextSuccess(t *testing.T) {
	h := NewHandlerWithStore(uploadGateway{fakeKnowledgeGateway: &fakeKnowledgeGateway{}}, knowledge.NewStore(500))
	body, contentType := multipartKnowledgeUpload(t, "notes.md", []byte("# Notes\n\nHello"))
	req := httptest.NewRequest(http.MethodPost, "/v1/knowledge/upload", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()

	h.handleUpload(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Source *knowledge.Source `json:"source"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode body: %v: %s", err, rec.Body.String())
	}
	if out.Source == nil || out.Source.Name != "notes.md" {
		t.Fatalf("source = %+v, want notes.md", out.Source)
	}
}

func TestUploadNativeMinerUSuccess(t *testing.T) {
	parser := uploadParser{
		enabled: true,
		result:  &mineru.ParseResult{Backend: "cli", Markdown: "# Parsed\n\nHello MinerU", JSON: `{}`},
	}
	h := NewHandlerWithStore(uploadGateway{fakeKnowledgeGateway: &fakeKnowledgeGateway{}, parser: parser}, knowledge.NewStore(500))
	body, contentType := multipartKnowledgeUpload(t, "paper.pdf", []byte("%PDF"))
	req := httptest.NewRequest(http.MethodPost, "/v1/knowledge/upload", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()

	h.handleUpload(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Source *knowledge.Source `json:"source"`
		Parse  map[string]any    `json:"parse"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode body: %v: %s", err, rec.Body.String())
	}
	if out.Source == nil || out.Source.Name != "paper.md" || out.Source.Path != "paper.pdf" {
		t.Fatalf("source = %+v, want converted paper.md with original path", out.Source)
	}
	if out.Parse["parser"] != "mineru" {
		t.Fatalf("parse = %#v, want mineru metadata", out.Parse)
	}
}

func TestUploadNativeDisabledPackGate(t *testing.T) {
	// The gateway route-level disabled gate is covered in migration tests; this
	// unit keeps the native handler's method gate explicit.
	h := NewHandlerWithStore(uploadGateway{fakeKnowledgeGateway: &fakeKnowledgeGateway{}}, knowledge.NewStore(500))
	rec := httptest.NewRecorder()
	h.handleUpload(rec, httptest.NewRequest(http.MethodGet, "/v1/knowledge/upload", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}
