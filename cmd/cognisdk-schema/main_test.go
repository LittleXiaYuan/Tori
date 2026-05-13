package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunListSchemas(t *testing.T) {
	if err := run([]string{"list"}); err != nil {
		t.Fatalf("list schemas: %v", err)
	}
	if err := run([]string{"list", "--json"}); err != nil {
		t.Fatalf("list schemas json: %v", err)
	}
	if err := run([]string{"list", "--json", "--with-schema"}); err != nil {
		t.Fatalf("list schemas json with schema: %v", err)
	}
}

func TestRunListSchemasOut(t *testing.T) {
	dir := t.TempDir()
	textOut := filepath.Join(dir, "schemas.txt")
	jsonOut := filepath.Join(dir, "schemas.json")
	if err := run([]string{"list", "--out", textOut}); err != nil {
		t.Fatalf("list schemas out: %v", err)
	}
	textData, err := os.ReadFile(textOut)
	if err != nil {
		t.Fatalf("read list output: %v", err)
	}
	if !strings.Contains(string(textData), "pack-bundle") {
		t.Fatalf("list output missing pack-bundle: %s", textData)
	}
	if err := run([]string{"list", "--json", "--out", jsonOut}); err != nil {
		t.Fatalf("list schemas json out: %v", err)
	}
	jsonData, err := os.ReadFile(jsonOut)
	if err != nil {
		t.Fatalf("read list json output: %v", err)
	}
	if !json.Valid(jsonData) {
		t.Fatalf("list json output is not valid json: %s", jsonData)
	}
	var infos []map[string]any
	if err := json.Unmarshal(jsonData, &infos); err != nil {
		t.Fatalf("unmarshal list json output: %v", err)
	}
	if len(infos) == 0 || infos[0]["name"] == "" {
		t.Fatalf("unexpected list json output: %#v", infos)
	}
	withSchemaOut := filepath.Join(dir, "schemas-with-documents.json")
	if err := run([]string{"list", "--json", "--with-schema", "--out", withSchemaOut}); err != nil {
		t.Fatalf("list schemas json with schema out: %v", err)
	}
	withSchemaData, err := os.ReadFile(withSchemaOut)
	if err != nil {
		t.Fatalf("read list json with schema output: %v", err)
	}
	var entries []map[string]any
	if err := json.Unmarshal(withSchemaData, &entries); err != nil {
		t.Fatalf("unmarshal list json with schema output: %v", err)
	}
	if len(entries) == 0 || entries[0]["schema_document"] == nil {
		t.Fatalf("unexpected list json with schema output: %#v", entries)
	}
}

func TestRunListRejectsUnknownOption(t *testing.T) {
	err := run([]string{"list", "--bad"})
	if err == nil || !strings.Contains(err.Error(), "unknown list option") {
		t.Fatalf("expected unknown list option error, got %v", err)
	}
	err = run([]string{"list", "--out"})
	if err == nil || !strings.Contains(err.Error(), "--out requires a path") {
		t.Fatalf("expected list out path error, got %v", err)
	}
	err = run([]string{"list", "--with-schema"})
	if err == nil || !strings.Contains(err.Error(), "--with-schema requires --json") {
		t.Fatalf("expected with-schema requires json error, got %v", err)
	}
}

func TestRunExportSchemaToFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "digest-check.schema.json")
	if err := run([]string{"pack-bundle-digest-check", path}); err != nil {
		t.Fatalf("export schema: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	if !json.Valid(data) {
		t.Fatalf("exported schema is not valid json: %s", data)
	}
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	if schema["title"] != "Cognition SDK Pack Bundle Digest Check" {
		t.Fatalf("unexpected schema title: %#v", schema["title"])
	}
}

func TestRunExportAllSchemas(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "schemas")
	catalogPath := filepath.Join(dir, "schema-artifacts.json")
	if err := run([]string{"export", outDir, "--catalog", catalogPath}); err != nil {
		t.Fatalf("export all schemas: %v", err)
	}
	catalogData, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatalf("read schema artifact catalog: %v", err)
	}
	if !json.Valid(catalogData) {
		t.Fatalf("schema artifact catalog is not valid json: %s", catalogData)
	}
	var artifacts []map[string]any
	if err := json.Unmarshal(catalogData, &artifacts); err != nil {
		t.Fatalf("unmarshal schema artifact catalog: %v", err)
	}
	if len(artifacts) == 0 || artifacts[0]["file"] == "" {
		t.Fatalf("unexpected schema artifact catalog: %#v", artifacts)
	}
	if _, err := os.Stat(filepath.Join(outDir, "pack-bundle.schema.json")); err != nil {
		t.Fatalf("expected exported pack-bundle schema: %v", err)
	}
}

func TestRunExportAllSchemasRejectsBadArgs(t *testing.T) {
	err := run([]string{"export"})
	if err == nil || !strings.Contains(err.Error(), "usage: cognisdk-schema export") {
		t.Fatalf("expected export usage error, got %v", err)
	}
	err = run([]string{"export", t.TempDir(), "--catalog"})
	if err == nil || !strings.Contains(err.Error(), "--catalog requires a path") {
		t.Fatalf("expected catalog path error, got %v", err)
	}
	err = run([]string{"export", t.TempDir(), "--bad"})
	if err == nil || !strings.Contains(err.Error(), "unknown export option") {
		t.Fatalf("expected export unknown option error, got %v", err)
	}
}

func TestRunRejectsUnknownSchema(t *testing.T) {
	err := run([]string{"missing-schema"})
	if err == nil || !strings.Contains(err.Error(), "unknown schema") {
		t.Fatalf("expected unknown schema error, got %v", err)
	}
}

func TestRunRejectsTooManyArgs(t *testing.T) {
	err := run([]string{"pack-bundle", "a.json", "b.json"})
	if err == nil || !strings.Contains(err.Error(), "too many arguments") {
		t.Fatalf("expected too many arguments error, got %v", err)
	}
}
