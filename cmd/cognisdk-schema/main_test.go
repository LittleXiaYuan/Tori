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
}

func TestRunExportSchemaToFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "review.schema.json")
	if err := run([]string{"pack-bundle-apply-plan", path}); err != nil {
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
	if schema["title"] != "Cognition SDK Pack Bundle Apply Plan" {
		t.Fatalf("unexpected schema title: %#v", schema["title"])
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
