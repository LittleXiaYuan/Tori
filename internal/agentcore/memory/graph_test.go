package memory

import (
	"testing"
)

func TestGraphPutAndGetEntity(t *testing.T) {
	g := NewGraph()
	e := g.PutEntity(Entity{ID: "e1", Name: "Alice", Type: "person"})
	if e.Name != "Alice" {
		t.Fatalf("expected Alice, got %s", e.Name)
	}
	if e.Mentions != 1 {
		t.Fatalf("expected 1 mention, got %d", e.Mentions)
	}

	// Update same entity
	e2 := g.PutEntity(Entity{ID: "e1", Name: "Alice", Type: "person", Properties: map[string]string{"role": "engineer"}})
	if e2.Mentions != 2 {
		t.Fatalf("expected 2 mentions, got %d", e2.Mentions)
	}
	if e2.Properties["role"] != "engineer" {
		t.Fatal("expected role property")
	}
}

func TestGraphFindByName(t *testing.T) {
	g := NewGraph()
	g.PutEntity(Entity{ID: "e1", Name: "Golang", Type: "concept"})

	e, ok := g.FindByName("golang")
	if !ok {
		t.Fatal("should find by case-insensitive name")
	}
	if e.ID != "e1" {
		t.Fatalf("expected e1, got %s", e.ID)
	}
}

func TestGraphRelations(t *testing.T) {
	g := NewGraph()
	g.PutEntity(Entity{ID: "alice", Name: "Alice", Type: "person"})
	g.PutEntity(Entity{ID: "proj", Name: "Yunque", Type: "project"})

	r := g.PutRelation(Relation{ID: "r1", FromID: "alice", ToID: "proj", Type: "works_on"})
	if r.Weight != 0.5 {
		t.Fatalf("expected default weight 0.5, got %f", r.Weight)
	}

	// Strengthen same relation
	r2 := g.PutRelation(Relation{ID: "r1_dup", FromID: "alice", ToID: "proj", Type: "works_on"})
	if r2.Weight != 0.6 {
		t.Fatalf("expected strengthened weight 0.6, got %f", r2.Weight)
	}

	rels := g.GetRelations("alice")
	if len(rels) != 1 {
		t.Fatalf("expected 1 relation, got %d", len(rels))
	}
}

func TestGraphNeighbors(t *testing.T) {
	g := NewGraph()
	g.PutEntity(Entity{ID: "a", Name: "A", Type: "person"})
	g.PutEntity(Entity{ID: "b", Name: "B", Type: "person"})
	g.PutEntity(Entity{ID: "c", Name: "C", Type: "concept"})
	g.PutRelation(Relation{ID: "r1", FromID: "a", ToID: "b", Type: "knows"})
	g.PutRelation(Relation{ID: "r2", FromID: "b", ToID: "c", Type: "uses"})

	// Depth 1: only B
	n1 := g.Neighbors("a", 1)
	if len(n1) != 1 {
		t.Fatalf("expected 1 neighbor at depth 1, got %d", len(n1))
	}
	if n1[0].ID != "b" {
		t.Fatalf("expected B, got %s", n1[0].ID)
	}

	// Depth 2: B and C
	n2 := g.Neighbors("a", 2)
	if len(n2) != 2 {
		t.Fatalf("expected 2 neighbors at depth 2, got %d", len(n2))
	}
}

func TestGraphContextFor(t *testing.T) {
	g := NewGraph()
	g.PutEntity(Entity{ID: "alice", Name: "Alice", Type: "person", Properties: map[string]string{"lang": "Go"}})
	g.PutEntity(Entity{ID: "proj", Name: "Yunque", Type: "project"})
	g.PutRelation(Relation{ID: "r1", FromID: "alice", ToID: "proj", Type: "works_on"})

	ctx := g.ContextFor("alice")
	if ctx == "" {
		t.Fatal("expected non-empty context")
	}
	if !graphContains(ctx, "Alice") || !graphContains(ctx, "works_on") || !graphContains(ctx, "Yunque") {
		t.Fatalf("context should contain entity and relation info, got:\n%s", ctx)
	}
}

func TestGraphSearchEntities(t *testing.T) {
	g := NewGraph()
	g.PutEntity(Entity{ID: "e1", Name: "Golang Programming", Type: "concept"})
	g.PutEntity(Entity{ID: "e2", Name: "Python", Type: "concept"})
	g.PutEntity(Entity{ID: "e3", Name: "Alice", Type: "person", Properties: map[string]string{"skill": "golang"}})

	results := g.SearchEntities("golang", 10)
	if len(results) != 2 {
		t.Fatalf("expected 2 results for 'golang', got %d", len(results))
	}
}

func TestGraphRemoveEntity(t *testing.T) {
	g := NewGraph()
	g.PutEntity(Entity{ID: "a", Name: "A", Type: "x"})
	g.PutEntity(Entity{ID: "b", Name: "B", Type: "x"})
	g.PutRelation(Relation{ID: "r1", FromID: "a", ToID: "b", Type: "knows"})

	g.RemoveEntity("a")
	stats := g.Stats()
	if stats["entities"] != 1 {
		t.Fatalf("expected 1 entity after remove, got %d", stats["entities"])
	}
	if stats["relations"] != 0 {
		t.Fatalf("expected 0 relations after remove, got %d", stats["relations"])
	}
}

func graphContains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
