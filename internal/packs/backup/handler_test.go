package backuppack

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBackupExportImportRestoresLedgerDirectory(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataDir, "ledger"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "ledger", "ledger.db"), []byte("healthy ledger snapshot"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "memory.json"), []byte(`{"memory":"before"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	h := New(Config{DataDir: dataDir, Version: "0.1.0", Now: func() time.Time { return time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC) }})
	exportReq := httptest.NewRequest(http.MethodGet, "/v1/backup/export", nil)
	exportW := httptest.NewRecorder()
	h.Export(exportW, exportReq)
	if exportW.Code != http.StatusOK {
		t.Fatalf("export status=%d body=%s", exportW.Code, exportW.Body.String())
	}
	if !zipContains(t, exportW.Body.Bytes(), "ledger/ledger.db") {
		t.Fatal("backup archive should include ledger/ledger.db")
	}

	if err := os.WriteFile(filepath.Join(dataDir, "ledger", "ledger.db"), []byte("corrupted ledger"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "memory.json"), []byte(`{"memory":"after"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("backup", "backup.zip")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(exportW.Body.Bytes()); err != nil {
		t.Fatal(err)
	}
	mw.Close()

	importReq := httptest.NewRequest(http.MethodPost, "/v1/backup/import", &body)
	importReq.Header.Set("Content-Type", mw.FormDataContentType())
	importW := httptest.NewRecorder()
	h.Import(importW, importReq)
	if importW.Code != http.StatusOK {
		t.Fatalf("import status=%d body=%s", importW.Code, importW.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(importW.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["status"] != "restored" {
		t.Fatalf("unexpected import response: %#v", resp)
	}
	got, err := os.ReadFile(filepath.Join(dataDir, "ledger", "ledger.db"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "healthy ledger snapshot" {
		t.Fatalf("ledger was not restored: %q", string(got))
	}
}

func zipContains(t *testing.T, data []byte, name string) bool {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, _ := io.ReadAll(rc)
		rc.Close()
		return strings.Contains(string(content), "healthy ledger snapshot")
	}
	return false
}
