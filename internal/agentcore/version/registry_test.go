package version

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegistryRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	r.Register(Component{ID: "plugin-a", Name: "Plugin A", CurrentVersion: "1.0.0", Source: "plugin"})

	c, ok := r.Get("plugin-a")
	if !ok {
		t.Fatal("expected to find plugin-a")
	}
	if c.CurrentVersion != "1.0.0" {
		t.Fatalf("expected 1.0.0, got %s", c.CurrentVersion)
	}
}

func TestRegistryAutoID(t *testing.T) {
	r := NewRegistry()
	r.Register(Component{Name: "auto-id-test", CurrentVersion: "1.0.0"})

	_, ok := r.Get("auto-id-test")
	if !ok {
		t.Fatal("expected auto-generated ID from Name")
	}
}

func TestRegistryList(t *testing.T) {
	r := NewRegistry()
	r.Register(Component{ID: "a", Name: "A", CurrentVersion: "1.0.0"})
	r.Register(Component{ID: "b", Name: "B", CurrentVersion: "2.0.0"})

	list := r.List()
	if len(list) != 2 {
		t.Fatalf("expected 2, got %d", len(list))
	}
}

func TestRegistryRemove(t *testing.T) {
	r := NewRegistry()
	r.Register(Component{ID: "a", Name: "A", CurrentVersion: "1.0.0"})
	if !r.Remove("a") {
		t.Fatal("expected successful remove")
	}
	if r.Count() != 0 {
		t.Fatal("expected 0 after remove")
	}
	if r.Remove("nonexistent") {
		t.Fatal("expected false for nonexistent")
	}
}

func TestRegistryCheckUpdate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"version": "2.0.0"})
	}))
	defer srv.Close()

	r := NewRegistry()
	r.Register(Component{ID: "p1", Name: "P1", CurrentVersion: "1.0.0", UpdateURL: srv.URL})

	c, err := r.CheckUpdate(context.Background(), "p1")
	if err != nil {
		t.Fatalf("check update: %v", err)
	}
	if !c.UpdateAvail {
		t.Fatal("expected update available")
	}
	if c.LatestVersion != "2.0.0" {
		t.Fatalf("expected 2.0.0, got %s", c.LatestVersion)
	}
}

func TestRegistryCheckUpdateNoUpdate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"version": "1.0.0"})
	}))
	defer srv.Close()

	r := NewRegistry()
	r.Register(Component{ID: "p1", Name: "P1", CurrentVersion: "1.0.0", UpdateURL: srv.URL})

	c, err := r.CheckUpdate(context.Background(), "p1")
	if err != nil {
		t.Fatalf("check update: %v", err)
	}
	if c.UpdateAvail {
		t.Fatal("expected no update available")
	}
}

func TestRegistryUpdatesAvailable(t *testing.T) {
	r := NewRegistry()
	r.Register(Component{ID: "a", Name: "A", CurrentVersion: "1.0.0", UpdateAvail: true, LatestVersion: "2.0.0"})
	r.Register(Component{ID: "b", Name: "B", CurrentVersion: "1.0.0"})

	updates := r.UpdatesAvailable()
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}
	if updates[0].ID != "a" {
		t.Fatalf("expected 'a', got %s", updates[0].ID)
	}
}

func TestRegistrySetVersion(t *testing.T) {
	r := NewRegistry()
	r.Register(Component{ID: "a", Name: "A", CurrentVersion: "1.0.0", LatestVersion: "2.0.0", UpdateAvail: true})

	if !r.SetVersion("a", "2.0.0") {
		t.Fatal("expected successful set")
	}
	c, _ := r.Get("a")
	if c.CurrentVersion != "2.0.0" {
		t.Fatalf("expected 2.0.0, got %s", c.CurrentVersion)
	}
	if c.UpdateAvail {
		t.Fatal("update should be cleared after version set")
	}
}

func TestRegistryCheckUpdateNoURL(t *testing.T) {
	r := NewRegistry()
	r.Register(Component{ID: "a", Name: "A", CurrentVersion: "1.0.0"})

	_, err := r.CheckUpdate(context.Background(), "a")
	if err == nil {
		t.Fatal("expected error for no update URL")
	}
}

func TestRegistryCheckUpdateNotFound(t *testing.T) {
	r := NewRegistry()
	_, err := r.CheckUpdate(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent component")
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"2.0.0", "1.0.0", 1},
		{"1.0.0", "2.0.0", -1},
		{"1.2.0", "1.1.0", 1},
		{"1.0.1", "1.0.0", 1},
		{"1.10.0", "1.9.0", 1},
		{"0.1.0", "0.0.9", 1},
	}
	for _, c := range cases {
		got := CompareVersions(c.a, c.b)
		if (c.want > 0 && got <= 0) || (c.want < 0 && got >= 0) || (c.want == 0 && got != 0) {
			t.Errorf("CompareVersions(%q, %q) = %d, want sign %d", c.a, c.b, got, c.want)
		}
	}
}

func TestCheckAllUpdates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"version": "3.0.0"})
	}))
	defer srv.Close()

	r := NewRegistry()
	r.Register(Component{ID: "a", Name: "A", CurrentVersion: "1.0.0", UpdateURL: srv.URL})
	r.Register(Component{ID: "b", Name: "B", CurrentVersion: "1.0.0"}) // no update URL

	results := r.CheckAllUpdates(context.Background())
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}
