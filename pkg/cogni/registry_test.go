package cogni

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestRegistry_AddListGet(t *testing.T) {
	r := NewRegistry()
	d := &Declaration{ID: "a", Activation: ActivationRules{Keywords: []string{"x"}, MinScore: 0.2}}
	if err := r.Add(d, "test"); err != nil {
		t.Fatalf("add: %v", err)
	}

	if got, ok := r.Get("a"); !ok || got != d {
		t.Fatalf("Get returned wrong value: ok=%v got=%v", ok, got)
	}
	statuses := r.List()
	if len(statuses) != 1 || statuses[0].ID != "a" || !statuses[0].Enabled {
		t.Fatalf("List unexpected: %+v", statuses)
	}
	if active := r.Active(); len(active) != 1 || active[0] != d {
		t.Fatalf("Active unexpected: %+v", active)
	}
}

func TestRegistry_Add_RejectsInvalid(t *testing.T) {
	r := NewRegistry()
	if err := r.Add(nil, "test"); err == nil {
		t.Fatalf("nil should be rejected")
	}
	if err := r.Add(&Declaration{}, "test"); err == nil {
		t.Fatalf("empty ID should be rejected")
	}
}

func TestRegistry_Add_ReplacesExisting(t *testing.T) {
	r := NewRegistry()
	first := &Declaration{ID: "x", Description: "old"}
	second := &Declaration{ID: "x", Description: "new"}
	if err := r.Add(first, "test"); err != nil {
		t.Fatalf("add 1: %v", err)
	}
	if err := r.Add(second, "test"); err != nil {
		t.Fatalf("add 2: %v", err)
	}
	got, _ := r.Get("x")
	if got.Description != "new" {
		t.Fatalf("expected replacement, got %+v", got)
	}
}

func TestRegistry_Remove(t *testing.T) {
	r := NewRegistry()
	d := &Declaration{ID: "a"}
	_ = r.Add(d, "test")
	if !r.Remove("a") {
		t.Fatalf("expected remove=true")
	}
	if r.Remove("a") {
		t.Fatalf("second remove should be false")
	}
	if _, ok := r.Get("a"); ok {
		t.Fatalf("entry should be gone")
	}
}

func TestRegistry_SetEnabledFiltersActive(t *testing.T) {
	r := NewRegistry()
	a := &Declaration{ID: "a"}
	b := &Declaration{ID: "b"}
	_ = r.Add(a, "test")
	_ = r.Add(b, "test")

	if err := r.SetEnabled("b", false); err != nil {
		t.Fatalf("disable b: %v", err)
	}
	if r.IsEnabled("b") {
		t.Fatalf("b should be disabled")
	}
	active := r.Active()
	if len(active) != 1 || active[0].ID != "a" {
		t.Fatalf("Active should only contain a, got %+v", active)
	}

	if err := r.SetEnabled("missing", true); err == nil {
		t.Fatalf("expected error for missing id")
	}
}

func TestRegistry_VersionBumps(t *testing.T) {
	r := NewRegistry()
	v0 := r.Version()
	_ = r.Add(&Declaration{ID: "x"}, "test")
	v1 := r.Version()
	if v1 != v0+1 {
		t.Fatalf("Add should bump version: %d → %d", v0, v1)
	}
	_ = r.SetEnabled("x", false)
	if r.Version() != v1+1 {
		t.Fatalf("SetEnabled should bump version")
	}
	_ = r.SetEnabled("x", false)
	if r.Version() != v1+1 {
		t.Fatalf("redundant SetEnabled must not bump version")
	}
	r.Remove("x")
	if r.Version() != v1+2 {
		t.Fatalf("Remove should bump version")
	}
}

func TestRegistry_OnChangeFiresPerEvent(t *testing.T) {
	r := NewRegistry()
	var calls atomic.Int64
	r.OnChange(func(event, id string) { calls.Add(1) })

	_ = r.Add(&Declaration{ID: "x"}, "test")
	_ = r.SetEnabled("x", false)
	r.Remove("x")

	if got := calls.Load(); got != 3 {
		t.Fatalf("expected 3 hook invocations, got %d", got)
	}
}

func TestRegistry_ReloadFromDir_AddsUpdatesRemoves(t *testing.T) {
	dir := t.TempDir()
	mustWrite := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	mustWrite("a.json", `{"id":"a","description":"v1"}`)
	mustWrite("b.json", `{"id":"b"}`)

	r := NewRegistry()
	// Pre-existing API entry must survive the reload (its source is "api").
	_ = r.Add(&Declaration{ID: "manual"}, "api")

	sum, err := r.ReloadFromDir(dir)
	if err != nil {
		t.Fatalf("reload 1: %v", err)
	}
	if sum.Added != 2 || sum.Updated != 0 || sum.Removed != 0 {
		t.Fatalf("first reload summary unexpected: %+v", sum)
	}

	// modify a.json, drop b.json, add c.json
	mustWrite("a.json", `{"id":"a","description":"v2"}`)
	if err := os.Remove(filepath.Join(dir, "b.json")); err != nil {
		t.Fatalf("rm b: %v", err)
	}
	mustWrite("c.json", `{"id":"c"}`)

	sum, err = r.ReloadFromDir(dir)
	if err != nil {
		t.Fatalf("reload 2: %v", err)
	}
	if sum.Added != 1 || sum.Updated != 1 || sum.Removed != 1 {
		t.Fatalf("second reload summary unexpected: %+v", sum)
	}

	a, _ := r.Get("a")
	if a.Description != "v2" {
		t.Fatalf("a should be updated, got %+v", a)
	}
	if _, ok := r.Get("b"); ok {
		t.Fatalf("b should be removed")
	}
	if _, ok := r.Get("c"); !ok {
		t.Fatalf("c should be added")
	}
	// API-sourced entry survives.
	if _, ok := r.Get("manual"); !ok {
		t.Fatalf("manual entry must not be removed by file reload")
	}
}

func TestRegistry_ReloadFromDir_ReportsErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ok.json"), []byte(`{"id":"ok"}`), 0o644); err != nil {
		t.Fatalf("write ok: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte(`not json`), 0o644); err != nil {
		t.Fatalf("write bad: %v", err)
	}

	r := NewRegistry()
	sum, err := r.ReloadFromDir(dir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if sum.Added != 1 || len(sum.Errors) != 1 {
		t.Fatalf("expected 1 added + 1 error, got %+v", sum)
	}
}

func TestRegistry_ReloadFromDir_MissingDir(t *testing.T) {
	r := NewRegistry()
	sum, err := r.ReloadFromDir(filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatalf("missing dir should not error, got %v", err)
	}
	if sum.Added != 0 || sum.Updated != 0 || sum.Removed != 0 {
		t.Fatalf("expected empty summary, got %+v", sum)
	}
}
