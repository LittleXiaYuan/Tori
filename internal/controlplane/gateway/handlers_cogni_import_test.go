package gateway

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"yunque-agent/pkg/cogni"
)

func TestCogniImportBundle_PersistsToDisc(t *testing.T) {
	// Create a temporary directory for cogni files
	tmpDir := t.TempDir()

	// Setup gateway with registry and directory
	gw := &Gateway{}
	reg := cogni.NewRegistry()
	gw.SetCogniRegistry(reg, tmpDir)

	// Pre-populate with one existing cogni
	existingDecl := &cogni.Declaration{
		ID:          "existing-cogni",
		Description: "original version",
		Activation:  cogni.ActivationRules{AlwaysOn: true},
	}
	if err := reg.Add(existingDecl, "test"); err != nil {
		t.Fatalf("failed to add existing cogni: %v", err)
	}

	// Create a bundle with new and updated cognis
	bundle := cogni.Bundle{
		Schema: cogni.BundleSchema,
		Cognis: []*cogni.Declaration{
			{
				ID:          "new-cogni-1",
				DisplayName: "New Cogni 1",
				Description: "First new cogni",
				Activation:  cogni.ActivationRules{Keywords: []string{"test"}},
			},
			{
				ID:          "new-cogni-2",
				DisplayName: "New Cogni 2",
				Description: "Second new cogni",
				Activation:  cogni.ActivationRules{AlwaysOn: true},
			},
			{
				ID:          "existing-cogni",
				Description: "updated version",
				Activation:  cogni.ActivationRules{AlwaysOn: true},
			},
		},
	}

	// Marshal bundle to JSON
	bundleJSON, err := json.Marshal(bundle)
	if err != nil {
		t.Fatalf("failed to marshal bundle: %v", err)
	}

	// Create HTTP request
	req := httptest.NewRequest(http.MethodPost, "/v1/cognis/import?overwrite=true", bytes.NewReader(bundleJSON))
	w := httptest.NewRecorder()

	// Call the handler
	gw.cogniImportBundle(w, req)

	// Check response status
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Parse response
	var summary cogni.ImportSummary
	if err := json.NewDecoder(w.Body).Decode(&summary); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify summary
	if len(summary.Added) != 2 {
		t.Errorf("expected 2 added cognis, got %d: %v", len(summary.Added), summary.Added)
	}
	if len(summary.Updated) != 1 {
		t.Errorf("expected 1 updated cogni, got %d: %v", len(summary.Updated), summary.Updated)
	}
	if len(summary.Failed) != 0 {
		t.Errorf("expected 0 failed cognis, got %d: %v", len(summary.Failed), summary.Failed)
	}

	// Verify files were created on disk
	expectedFiles := []string{"new-cogni-1.json", "new-cogni-2.json", "existing-cogni.json"}
	for _, filename := range expectedFiles {
		filePath := filepath.Join(tmpDir, filename)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist, but it doesn't", filename)
		} else {
			// Verify file content is valid JSON and matches the declaration
			data, err := os.ReadFile(filePath)
			if err != nil {
				t.Errorf("failed to read file %s: %v", filename, err)
				continue
			}
			var decl cogni.Declaration
			if err := json.Unmarshal(data, &decl); err != nil {
				t.Errorf("file %s contains invalid JSON: %v", filename, err)
			}
		}
	}
}

func TestCogniImportBundle_SkipsFailedCognis(t *testing.T) {
	tmpDir := t.TempDir()

	gw := &Gateway{}
	reg := cogni.NewRegistry()
	gw.SetCogniRegistry(reg, tmpDir)

	// Create a bundle with valid and invalid cognis
	bundle := cogni.Bundle{
		Schema: cogni.BundleSchema,
		Cognis: []*cogni.Declaration{
			{
				ID:          "valid-cogni",
				Description: "This one is valid",
				Activation:  cogni.ActivationRules{AlwaysOn: true},
			},
			{
				// Invalid: missing ID
				Description: "This one is invalid",
				Activation:  cogni.ActivationRules{AlwaysOn: true},
			},
			{
				ID: "invalid-score",
				// Invalid: MinScore out of range
				Activation: cogni.ActivationRules{MinScore: 2.0},
			},
		},
	}

	bundleJSON, _ := json.Marshal(bundle)
	req := httptest.NewRequest(http.MethodPost, "/v1/cognis/import", bytes.NewReader(bundleJSON))
	w := httptest.NewRecorder()

	gw.cogniImportBundle(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var summary cogni.ImportSummary
	json.NewDecoder(w.Body).Decode(&summary)

	// Only valid cogni should be added
	if len(summary.Added) != 1 {
		t.Errorf("expected 1 added cogni, got %d", len(summary.Added))
	}
	if len(summary.Failed) != 2 {
		t.Errorf("expected 2 failed cognis, got %d", len(summary.Failed))
	}

	// Only valid cogni should have a file
	validPath := filepath.Join(tmpDir, "valid-cogni.json")
	if _, err := os.Stat(validPath); os.IsNotExist(err) {
		t.Errorf("expected valid-cogni.json to exist")
	}

	// Invalid cognis should not have files
	invalidPath := filepath.Join(tmpDir, "invalid-score.json")
	if _, err := os.Stat(invalidPath); !os.IsNotExist(err) {
		t.Errorf("invalid-score.json should not exist")
	}
}

func TestCogniImportBundle_HandlesEmptyDirectory(t *testing.T) {
	// Test with empty cogniDir (should not crash)
	gw := &Gateway{}
	reg := cogni.NewRegistry()
	gw.SetCogniRegistry(reg, "") // Empty directory

	bundle := cogni.Bundle{
		Schema: cogni.BundleSchema,
		Cognis: []*cogni.Declaration{
			{
				ID:         "test-cogni",
				Activation: cogni.ActivationRules{AlwaysOn: true},
			},
		},
	}

	bundleJSON, _ := json.Marshal(bundle)
	req := httptest.NewRequest(http.MethodPost, "/v1/cognis/import", bytes.NewReader(bundleJSON))
	w := httptest.NewRecorder()

	gw.cogniImportBundle(w, req)

	// Should succeed even without directory
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	// Cogni should still be in registry
	if _, ok := reg.Get("test-cogni"); !ok {
		t.Errorf("cogni should be in registry even without persistence")
	}
}
