package websearch

import (
	"context"
	"testing"
)

type mockProvider struct {
	name    string
	results []Result
	err     error
}

func (m *mockProvider) Name() string { return m.name }
func (m *mockProvider) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	if m.err != nil {
		return nil, m.err
	}
	if limit < len(m.results) {
		return m.results[:limit], nil
	}
	return m.results, nil
}

func TestRegistryRegisterAndSearch(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&mockProvider{
		name: "test",
		results: []Result{
			{Title: "Result 1", URL: "https://example.com", Snippet: "First result"},
		},
	})

	results, err := reg.Search(context.Background(), "golang", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("results = %d, want 1", len(results))
	}
	if results[0].Title != "Result 1" {
		t.Errorf("title = %s, want Result 1", results[0].Title)
	}
}

func TestRegistryNoProvider(t *testing.T) {
	reg := NewRegistry()
	_, err := reg.Search(context.Background(), "test", 10)
	if err == nil {
		t.Error("expected error with no provider")
	}
}

func TestRegistrySetPrimary(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&mockProvider{name: "p1", results: []Result{{Title: "P1"}}})
	reg.Register(&mockProvider{name: "p2", results: []Result{{Title: "P2"}}})

	// Default primary should be first registered
	results, _ := reg.Search(context.Background(), "test", 10)
	if results[0].Title != "P1" {
		t.Errorf("default primary title = %s, want P1", results[0].Title)
	}

	// Switch primary
	ok := reg.SetPrimary("p2")
	if !ok {
		t.Error("SetPrimary should succeed")
	}

	results, _ = reg.Search(context.Background(), "test", 10)
	if results[0].Title != "P2" {
		t.Errorf("after SetPrimary title = %s, want P2", results[0].Title)
	}
}

func TestRegistrySetPrimaryNotFound(t *testing.T) {
	reg := NewRegistry()
	ok := reg.SetPrimary("nonexistent")
	if ok {
		t.Error("SetPrimary should fail for unknown provider")
	}
}

func TestRegistrySearchWith(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&mockProvider{name: "google", results: []Result{{Title: "G"}}})
	reg.Register(&mockProvider{name: "bing", results: []Result{{Title: "B"}}})

	results, err := reg.SearchWith(context.Background(), "bing", "test", 10)
	if err != nil {
		t.Fatalf("SearchWith: %v", err)
	}
	if results[0].Title != "B" {
		t.Errorf("title = %s, want B", results[0].Title)
	}
}

func TestRegistrySearchWithNotFound(t *testing.T) {
	reg := NewRegistry()
	_, err := reg.SearchWith(context.Background(), "nonexistent", "test", 10)
	if err == nil {
		t.Error("expected error for unknown provider")
	}
}

func TestRegistryList(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&mockProvider{name: "alpha"})
	reg.Register(&mockProvider{name: "beta"})

	names := reg.List()
	if len(names) != 2 {
		t.Errorf("names = %d, want 2", len(names))
	}
}
