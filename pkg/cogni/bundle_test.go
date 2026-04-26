package cogni

import (
	"encoding/json"
	"testing"
)

func TestExportBundle_AllByDefault(t *testing.T) {
	r := NewRegistry()
	_ = r.Add(&Declaration{ID: "a"}, "test")
	_ = r.Add(&Declaration{ID: "b"}, "test")

	b := r.ExportBundle(nil, "quick snapshot")
	if b.Schema != BundleSchema {
		t.Fatalf("schema must be set, got %q", b.Schema)
	}
	if len(b.Cognis) != 2 {
		t.Fatalf("expected 2 cognis, got %d", len(b.Cognis))
	}
	// Declarations are deep-copied; mutating the bundle must not affect the registry.
	b.Cognis[0].DisplayName = "MUTATED"
	got, _ := r.Get(b.Cognis[0].ID)
	if got.DisplayName == "MUTATED" {
		t.Fatalf("bundle mutation leaked back into registry")
	}
	if b.Notes != "quick snapshot" {
		t.Fatalf("notes must be preserved, got %q", b.Notes)
	}
}

func TestExportBundle_SubsetSilentlySkipsUnknown(t *testing.T) {
	r := NewRegistry()
	_ = r.Add(&Declaration{ID: "x"}, "test")
	b := r.ExportBundle([]string{"x", "ghost"}, "")
	if len(b.Cognis) != 1 || b.Cognis[0].ID != "x" {
		t.Fatalf("expected only x, got %+v", b.Cognis)
	}
}

func TestImportBundle_AddsNewDeclarations(t *testing.T) {
	r := NewRegistry()
	bundle := &Bundle{
		Schema: BundleSchema,
		Cognis: []*Declaration{
			{ID: "new-1", Activation: ActivationRules{Keywords: []string{"hi"}}},
			{ID: "new-2", Activation: ActivationRules{AlwaysOn: true}},
		},
	}
	sum, err := r.ImportBundle(bundle, false)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if len(sum.Added) != 2 {
		t.Fatalf("expected 2 added, got %+v", sum)
	}
	if len(sum.Skipped) != 0 || len(sum.Failed) != 0 {
		t.Fatalf("no skipped/failed expected, got %+v", sum)
	}
	if _, ok := r.Get("new-1"); !ok {
		t.Fatalf("new-1 must be present after import")
	}
}

func TestImportBundle_SkipsExistingWithoutOverwrite(t *testing.T) {
	r := NewRegistry()
	_ = r.Add(&Declaration{ID: "dup", Description: "original"}, "test")
	bundle := &Bundle{
		Schema: BundleSchema,
		Cognis: []*Declaration{{ID: "dup", Description: "bundled"}},
	}
	sum, err := r.ImportBundle(bundle, false)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if len(sum.Skipped) != 1 || sum.Skipped[0] != "dup" {
		t.Fatalf("expected dup skipped, got %+v", sum)
	}
	got, _ := r.Get("dup")
	if got.Description != "original" {
		t.Fatalf("original must be preserved when overwrite=false")
	}
}

func TestImportBundle_OverwriteReplacesExisting(t *testing.T) {
	r := NewRegistry()
	_ = r.Add(&Declaration{ID: "dup", Description: "original"}, "test")
	bundle := &Bundle{
		Schema: BundleSchema,
		Cognis: []*Declaration{{ID: "dup", Description: "bundled"}},
	}
	sum, err := r.ImportBundle(bundle, true)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if len(sum.Updated) != 1 || sum.Updated[0] != "dup" {
		t.Fatalf("expected dup updated, got %+v", sum)
	}
	got, _ := r.Get("dup")
	if got.Description != "bundled" {
		t.Fatalf("overwrite=true must replace, got %q", got.Description)
	}
}

func TestImportBundle_RejectsInvalidDeclarations(t *testing.T) {
	r := NewRegistry()
	bundle := &Bundle{
		Schema: BundleSchema,
		Cognis: []*Declaration{
			{ID: "good"},
			{},                                   // missing ID
			{ID: "bad", Activation: ActivationRules{MinScore: 2.0}}, // out-of-range
		},
	}
	sum, err := r.ImportBundle(bundle, false)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if len(sum.Added) != 1 || sum.Added[0] != "good" {
		t.Fatalf("expected only 'good' imported, got %+v", sum)
	}
	if len(sum.Failed) != 2 {
		t.Fatalf("expected 2 failures, got %+v", sum.Failed)
	}
}

func TestImportBundle_RejectsUnknownSchema(t *testing.T) {
	r := NewRegistry()
	_, err := r.ImportBundle(&Bundle{Schema: "foreign/v9", Cognis: nil}, false)
	if err == nil {
		t.Fatalf("expected error on unknown schema")
	}
}

func TestImportBundle_EmptySchemaIsTolerated(t *testing.T) {
	r := NewRegistry()
	_, err := r.ImportBundle(&Bundle{
		Cognis: []*Declaration{{ID: "x"}},
	}, false)
	if err != nil {
		t.Fatalf("empty schema must be tolerated for hand-crafted bundles, got %v", err)
	}
	if _, ok := r.Get("x"); !ok {
		t.Fatalf("x must be present")
	}
}

func TestBundle_JSONRoundTrip(t *testing.T) {
	r := NewRegistry()
	_ = r.Add(&Declaration{
		ID:          "round",
		DisplayName: "Round Trip",
		Activation:  ActivationRules{Keywords: []string{"k"}, MinScore: 0.3},
		Context:     ContextInjection{Static: "hi"},
		Surface:     ToolSurface{Only: []string{"a"}, MaxTools: 3},
		Checks:      []ActivationCheck{{Message: "k", ExpectActive: boolPtr(true)}},
	}, "test")

	b := r.ExportBundle(nil, "")
	data, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back Bundle
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(back.Cognis) != 1 || back.Cognis[0].ID != "round" {
		t.Fatalf("round-trip lost cogni: %+v", back)
	}
	if len(back.Cognis[0].Checks) != 1 {
		t.Fatalf("round-trip lost checks: %+v", back.Cognis[0])
	}
}
