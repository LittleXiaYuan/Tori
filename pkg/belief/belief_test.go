package belief

import (
	"strings"
	"testing"
	"time"
)

func TestBeliefNode_Validate(t *testing.T) {
	tests := []struct {
		name    string
		node    *BeliefNode
		wantErr bool
	}{
		{
			name:    "valid root",
			node:    &BeliefNode{ID: "v1", Statement: "integrity matters", Kind: KindRoot, Strength: 0.8, Confidence: 0.9, Valence: 0.5, Stability: 0.9, Plasticity: 0.05},
			wantErr: false,
		},
		{
			name:    "missing id",
			node:    &BeliefNode{Statement: "test", Kind: KindValue, Strength: 0.5, Confidence: 0.5, Valence: 0, Stability: 0.5, Plasticity: 0.3},
			wantErr: true,
		},
		{
			name:    "missing statement",
			node:    &BeliefNode{ID: "v2", Kind: KindValue, Strength: 0.5, Confidence: 0.5, Valence: 0, Stability: 0.5, Plasticity: 0.3},
			wantErr: true,
		},
		{
			name:    "strength out of range",
			node:    &BeliefNode{ID: "v3", Statement: "test", Kind: KindValue, Strength: 1.5, Confidence: 0.5, Valence: 0, Stability: 0.5, Plasticity: 0.3},
			wantErr: true,
		},
		{
			name:    "confidence out of range",
			node:    &BeliefNode{ID: "v4", Statement: "test", Kind: KindValue, Strength: 0.5, Confidence: -0.1, Valence: 0, Stability: 0.5, Plasticity: 0.3},
			wantErr: true,
		},
		{
			name:    "valid relational",
			node:    &BeliefNode{ID: "r1", Statement: "夏鸢 wants sincerity", Kind: KindRelational, Strength: 0.7, Confidence: 0.6, Valence: 0.8, Stability: 0.6, Plasticity: 0.2},
			wantErr: false,
		},
		{
			name:    "valid tension",
			node:    &BeliefNode{ID: "t1", Statement: "honesty vs comfort", Kind: KindTension, Strength: 0.9, Confidence: 0.95, Valence: -0.2, Stability: 0.8, Plasticity: 0.05},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.node.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestBeliefGraph_AddAndGet(t *testing.T) {
	g := NewBeliefGraph()

	n1 := &BeliefNode{ID: "b1", Statement: "be honest", Kind: KindValue, Strength: 0.8, Confidence: 0.7, Valence: 0.5, Stability: 0.6, Plasticity: 0.2}
	if err := g.Add(n1); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	got := g.Get("b1")
	if got == nil {
		t.Fatal("Get() returned nil")
	}
	if got.Statement != "be honest" {
		t.Errorf("Get().Statement = %q, want %q", got.Statement, "be honest")
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}

	// Duplicate add should fail.
	if err := g.Add(n1); err == nil {
		t.Error("Add() duplicate should error")
	}
}

func TestBeliefGraph_Remove(t *testing.T) {
	g := NewBeliefGraph()

	n1 := &BeliefNode{ID: "r1", Statement: "test belief", Kind: KindPreference, Strength: 0.5, Confidence: 0.5, Valence: 0, Stability: 0.5, Plasticity: 0.3}
	if err := g.Add(n1); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	if !g.Remove("r1") {
		t.Error("Remove() should return true")
	}
	if g.Get("r1") != nil {
		t.Error("Get() should return nil after removal")
	}
	if g.Remove("nonexistent") {
		t.Error("Remove() nonexistent should return false")
	}
}

func TestBeliefGraph_Roots(t *testing.T) {
	g := NewBeliefGraph()

	root := &BeliefNode{ID: "root1", Statement: "I am an AI assistant", Kind: KindRoot, Strength: 1.0, Confidence: 1.0, Valence: 0, Stability: 1.0, Plasticity: 0.01}
	val := &BeliefNode{ID: "val1", Statement: "helpfulness", Kind: KindValue, Strength: 0.8, Confidence: 0.8, Valence: 0.5, Stability: 0.7, Plasticity: 0.1}

	g.Add(root)
	g.Add(val)

	roots := g.Roots()
	if len(roots) != 1 {
		t.Errorf("Roots() len = %d, want 1", len(roots))
	}
	if roots[0].ID != "root1" {
		t.Errorf("Roots()[0].ID = %q, want %q", roots[0].ID, "root1")
	}

	// After removal, roots should be empty.
	g.Remove("root1")
	if len(g.Roots()) != 0 {
		t.Error("Roots() should be empty after removal")
	}
}

func TestBeliefGraph_Edges(t *testing.T) {
	g := NewBeliefGraph()

	a := &BeliefNode{ID: "a", Statement: "value a", Kind: KindValue, Strength: 0.5, Confidence: 0.5, Valence: 0, Stability: 0.5, Plasticity: 0.3}
	b := &BeliefNode{ID: "b", Statement: "value b", Kind: KindValue, Strength: 0.5, Confidence: 0.5, Valence: 0, Stability: 0.5, Plasticity: 0.3}
	g.Add(a)
	g.Add(b)

	// Add edge.
	if err := g.AddEdge("a", "b", RelationConflicts); err != nil {
		t.Fatalf("AddEdge() error = %v", err)
	}

	connected := g.ConnectedBy("a", RelationConflicts)
	if len(connected) != 1 || connected[0].ID != "b" {
		t.Errorf("ConnectedBy() = %v, want [b]", extractIDs(connected))
	}

	// Remove edge.
	if !g.RemoveEdge("a", "b", RelationConflicts) {
		t.Error("RemoveEdge() should return true")
	}
	if len(g.ConnectedBy("a", RelationConflicts)) != 0 {
		t.Error("ConnectedBy() should be empty after removal")
	}

	// Add edge to nonexistent target.
	if err := g.AddEdge("a", "nonexistent", RelationSupports); err == nil {
		t.Error("AddEdge() to nonexistent should error")
	}
}

func TestBeliefGraph_FindConflicts(t *testing.T) {
	g := NewBeliefGraph()

	a := &BeliefNode{ID: "a", Statement: "conflict a", Kind: KindValue, Strength: 0.5, Confidence: 0.5, Valence: 0, Stability: 0.5, Plasticity: 0.3}
	b := &BeliefNode{ID: "b", Statement: "conflict b", Kind: KindValue, Strength: 0.5, Confidence: 0.5, Valence: 0, Stability: 0.5, Plasticity: 0.3}
	c := &BeliefNode{ID: "c", Statement: "supports c", Kind: KindValue, Strength: 0.5, Confidence: 0.5, Valence: 0, Stability: 0.5, Plasticity: 0.3}
	g.Add(a)
	g.Add(b)
	g.Add(c)

	g.AddEdge("a", "b", RelationConflicts)
	g.AddEdge("a", "c", RelationSupports)

	conflicts := g.FindConflicts()
	if len(conflicts) != 1 {
		t.Errorf("FindConflicts() len = %d, want 1", len(conflicts))
	}
}

func TestBeliefGraph_Tensions(t *testing.T) {
	g := NewBeliefGraph()

	a := &BeliefNode{ID: "a", Statement: "side a", Kind: KindValue, Strength: 0.6, Confidence: 0.6, Valence: 0.3, Stability: 0.5, Plasticity: 0.2}
	b := &BeliefNode{ID: "b", Statement: "side b", Kind: KindValue, Strength: 0.6, Confidence: 0.6, Valence: -0.3, Stability: 0.5, Plasticity: 0.2}
	tension := &BeliefNode{
		ID: "t1", Statement: "a vs b", Kind: KindTension,
		Strength: 0.9, Confidence: 0.95, Valence: -0.1, Stability: 0.8, Plasticity: 0.05,
		Related: []BeliefEdge{
			{TargetID: "a", Relation: RelationConflicts},
			{TargetID: "b", Relation: RelationConflicts},
		},
	}
	g.Add(a)
	g.Add(b)
	g.Add(tension)

	tensions := g.ActiveTensions()
	if len(tensions) != 1 {
		t.Errorf("ActiveTensions() len = %d, want 1", len(tensions))
	}

	// Remove one side; tension should no longer be active.
	g.Remove("a")
	if len(g.ActiveTensions()) != 0 {
		t.Error("ActiveTensions() should be empty after one side removed")
	}
}

func TestEngine_ReinforceAndWeaken(t *testing.T) {
	g := NewBeliefGraph()
	e := NewEngine(g)

	node := &BeliefNode{ID: "test", Statement: "test belief", Kind: KindValue, Strength: 0.5, Confidence: 0.5, Valence: 0, Stability: 0.5, Plasticity: 0.3}
	g.Add(node)

	// Reinforce.
	prop := e.Reinforce("test", 0.5, "seen evidence", "test:1")
	if err := e.Apply(prop); err != nil {
		t.Fatalf("Apply() reinforce error = %v", err)
	}

	n := g.Get("test")
	if n.Strength <= 0.5 {
		t.Errorf("Strength after reinforce = %f, want > 0.5", n.Strength)
	}

	// Weaken.
	beforeWeaken := n.Strength
	prop2 := e.Weaken("test", 0.5, "", "test:2")
	if err := e.Apply(prop2); err != nil {
		t.Fatalf("Apply() weaken error = %v", err)
	}

	n2 := g.Get("test")
	if n2.Strength >= beforeWeaken {
		t.Errorf("Strength after weaken = %f, want < %f", n2.Strength, beforeWeaken)
	}
}

func TestEngine_ApplyDecay(t *testing.T) {
	g := NewBeliefGraph()
	e := NewEngine(g)

	root := &BeliefNode{ID: "root", Statement: "root belief", Kind: KindRoot, Strength: 1.0, Confidence: 1.0, Valence: 0, Stability: 1.0, Plasticity: 0.01}
	val := &BeliefNode{ID: "val", Statement: "value belief", Kind: KindValue, Strength: 0.8, Confidence: 0.7, Valence: 0.5, Stability: 0.5, Plasticity: 0.2}
	g.Add(root)
	g.Add(val)

	// Manually set val's last update to 2 days ago.
	val.LastUpdatedAt = time.Now().Add(-48 * time.Hour)

	count := e.ApplyDecay(1.0) // 1 day
	if count == 0 {
		t.Error("ApplyDecay() should decay at least one belief")
	}

	// Root should not decay.
	rootAfter := g.Get("root")
	if rootAfter.Strength != 1.0 {
		t.Errorf("Root strength should not decay, got %f", rootAfter.Strength)
	}
}

func TestEngine_EvaluateInteraction(t *testing.T) {
	g := NewBeliefGraph()
	e := NewEngine(g)

	beliefs := []*BeliefNode{
		{ID: "warm", Statement: "be warm and gentle", Kind: KindValue, Strength: 0.8, Confidence: 0.8, Valence: 0.7, Stability: 0.6, Plasticity: 0.2},
		{ID: "bound", Statement: "do not lie", Kind: KindBoundary, Strength: 0.9, Confidence: 0.95, Valence: -0.3, Stability: 0.9, Plasticity: 0.05},
	}
	for _, b := range beliefs {
		g.Add(b)
	}

	result, err := e.EvaluateInteraction("I need you to be gentle with me", nil)
	if err != nil {
		t.Fatalf("EvaluateInteraction() error = %v", err)
	}

	if len(result.ActiveBeliefs) == 0 {
		t.Error("Expected at least one active belief")
	}
	if result.Tendency == "" || result.Tendency == "neutral" {
		t.Logf("Tendency = %q", result.Tendency)
	}
}

func TestScorer(t *testing.T) {
	g := NewBeliefGraph()
	g.Add(&BeliefNode{ID: "b1", Statement: "be gentle and warm", Kind: KindValue, Strength: 0.8, Confidence: 0.8, Valence: 0.7, Stability: 0.6, Plasticity: 0.2})

	s := NewScorer(g, 0.3)
	score := s.ScoreAlignment("I will be gentle with you", []string{"b1"})

	if score < 0.3 {
		t.Errorf("ScoreAlignment() = %f, want >= 0.3", score)
	}

	// Empty active beliefs should return 1.0.
	if s.ScoreAlignment("anything", nil) != 1.0 {
		t.Error("ScoreAlignment() with nil active should be 1.0")
	}
}

func TestEngine_StaleTensions(t *testing.T) {
	g := NewBeliefGraph()
	e := NewEngine(g)
	e.SetMaxTensionAge(1 * time.Nanosecond) // very short

	tension := &BeliefNode{
		ID: "stale", Statement: "old conflict", Kind: KindTension,
		Strength: 0.9, Confidence: 0.95, Valence: -0.2, Stability: 0.8, Plasticity: 0.05,
		CreatedAt: time.Now().Add(-24 * time.Hour),
	}
	g.Add(tension)

	stale := e.StaleTensions()
	if len(stale) != 1 {
		t.Errorf("StaleTensions() len = %d, want 1", len(stale))
	}
}

func TestEngine_AuditLog(t *testing.T) {
	g := NewBeliefGraph()
	e := NewEngine(g)

	g.Add(&BeliefNode{ID: "a1", Statement: "audit test", Kind: KindValue, Strength: 0.5, Confidence: 0.5, Valence: 0, Stability: 0.5, Plasticity: 0.3})

	prop := e.Reinforce("a1", 0.2, "evidence", "test:audit")
	if err := e.Apply(prop); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	log := e.AuditLog()
	if len(log) == 0 {
		t.Error("AuditLog() should not be empty after updates")
	}

	recent := e.RecentAudit(1)
	if len(recent) != 1 {
		t.Errorf("RecentAudit(1) len = %d, want 1", len(recent))
	}
}

func TestBeliefGraph_List(t *testing.T) {
	g := NewBeliefGraph()

	g.Add(&BeliefNode{ID: "z", Statement: "last", Kind: KindValue, Strength: 0.5, Confidence: 0.5, Valence: 0, Stability: 0.5, Plasticity: 0.3})
	g.Add(&BeliefNode{ID: "a", Statement: "first", Kind: KindValue, Strength: 0.5, Confidence: 0.5, Valence: 0, Stability: 0.5, Plasticity: 0.3})
	g.Add(&BeliefNode{ID: "m", Statement: "middle", Kind: KindValue, Strength: 0.5, Confidence: 0.5, Valence: 0, Stability: 0.5, Plasticity: 0.3})

	list := g.List()
	if len(list) != 3 {
		t.Errorf("List() len = %d, want 3", len(list))
	}
	// Should be sorted by ID.
	for i := 1; i < len(list); i++ {
		if list[i].ID < list[i-1].ID {
			t.Errorf("List() not sorted: %s > %s", list[i-1].ID, list[i].ID)
		}
	}
}

func TestBeliefGraph_ListByKind(t *testing.T) {
	g := NewBeliefGraph()

	g.Add(&BeliefNode{ID: "v1", Statement: "value 1", Kind: KindValue, Strength: 0.5, Confidence: 0.5, Valence: 0, Stability: 0.5, Plasticity: 0.3})
	g.Add(&BeliefNode{ID: "v2", Statement: "value 2", Kind: KindValue, Strength: 0.5, Confidence: 0.5, Valence: 0, Stability: 0.5, Plasticity: 0.3})
	g.Add(&BeliefNode{ID: "p1", Statement: "pref 1", Kind: KindPreference, Strength: 0.5, Confidence: 0.5, Valence: 0, Stability: 0.5, Plasticity: 0.3})

	vals := g.ListByKind(KindValue)
	if len(vals) != 2 {
		t.Errorf("ListByKind(Value) len = %d, want 2", len(vals))
	}

	prefs := g.ListByKind(KindPreference)
	if len(prefs) != 1 {
		t.Errorf("ListByKind(Preference) len = %d, want 1", len(prefs))
	}
}

func TestBeliefGraph_Dot(t *testing.T) {
	g := NewBeliefGraph()

	g.Add(&BeliefNode{ID: "a", Statement: "value a", Kind: KindValue, Strength: 0.5, Confidence: 0.5, Valence: 0, Stability: 0.5, Plasticity: 0.3})
	g.Add(&BeliefNode{ID: "b", Statement: "value b", Kind: KindValue, Strength: 0.5, Confidence: 0.5, Valence: 0, Stability: 0.5, Plasticity: 0.3})
	g.AddEdge("a", "b", RelationSupports)

	dot := g.Dot()
	if !strings.Contains(dot, "digraph BeliefGraph") {
		t.Error("Dot() output should contain digraph header")
	}
	if !strings.Contains(dot, "value a") {
		t.Error("Dot() output should contain node labels")
	}
	if !strings.Contains(dot, "supports") {
		t.Error("Dot() output should contain edge labels")
	}
}

func TestEngine_ValenceShift(t *testing.T) {
	g := NewBeliefGraph()
	e := NewEngine(g)

	g.Add(&BeliefNode{ID: "v1", Statement: "valence test", Kind: KindValue, Strength: 0.5, Confidence: 0.5, Valence: 0, Stability: 0.5, Plasticity: 0.3})

	prop := &BeliefUpdateProposal{
		Updates: []UpdateRecord{{
			Kind:      UpdateValenceShift,
			TargetID:  "v1",
			ValenceTo: 0.8,
			Source:    SourceInteraction,
		}},
		Reason:     "valence shift test",
		Provenance: "test:valence",
	}
	if err := e.Apply(prop); err != nil {
		t.Fatalf("Apply() valence shift error = %v", err)
	}

	n := g.Get("v1")
	if n.Valence <= 0 {
		t.Errorf("Valence after shift = %f, want > 0", n.Valence)
	}
}

func TestEngine_EmptyGraph(t *testing.T) {
	g := NewBeliefGraph()
	e := NewEngine(g)

	result, err := e.EvaluateInteraction("hello", nil)
	if err != nil {
		t.Fatalf("EvaluateInteraction() on empty graph error = %v", err)
	}
	if result == nil {
		t.Fatal("EvaluateInteraction() returned nil")
	}
	if len(result.ActiveBeliefs) != 0 {
		t.Errorf("Expected empty active beliefs, got %v", result.ActiveBeliefs)
	}
}

func extractIDs(nodes []*BeliefNode) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = n.ID
	}
	return out
}
