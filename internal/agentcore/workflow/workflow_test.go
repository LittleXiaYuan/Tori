package workflow

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDurationMarshalJSON(t *testing.T) {
	d := Duration{Duration: 30 * time.Second}
	data, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `"30s"` {
		t.Errorf("Marshal = %s, want \"30s\"", string(data))
	}
}

func TestDurationUnmarshalJSON(t *testing.T) {
	var d Duration
	if err := json.Unmarshal([]byte(`"5m0s"`), &d); err != nil {
		t.Fatal(err)
	}
	if d.Duration != 5*time.Minute {
		t.Errorf("Unmarshal = %v, want 5m", d.Duration)
	}
}

func TestJSONStoreDefinition(t *testing.T) {
	dir := t.TempDir()
	store := NewJSONStore(dir)

	def := &Definition{
		Name:        "test-workflow",
		Description: "A test",
		Version:     1,
		TenantID:    "tenant1",
		Nodes: []Node{
			{ID: "n1", Name: "Start", Type: NodeSkill},
			{ID: "n2", Name: "End", Type: NodeLLM},
		},
		Edges: []Edge{
			{ID: "e1", FromNode: "n1", ToNode: "n2"},
		},
	}

	if err := store.SaveDefinition(def); err != nil {
		t.Fatal(err)
	}
	if def.ID == "" {
		t.Error("ID should be assigned")
	}

	got, err := store.GetDefinition(def.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "test-workflow" {
		t.Errorf("Name = %q, want test-workflow", got.Name)
	}
	if len(got.Nodes) != 2 {
		t.Errorf("Nodes = %d, want 2", len(got.Nodes))
	}

	// List
	defs, err := store.ListDefinitions("tenant1")
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) != 1 {
		t.Errorf("ListDefinitions = %d, want 1", len(defs))
	}

	// List with different tenant
	defs, err = store.ListDefinitions("other")
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) != 0 {
		t.Errorf("ListDefinitions(other) = %d, want 0", len(defs))
	}

	// Delete
	if err := store.DeleteDefinition(def.ID); err != nil {
		t.Fatal(err)
	}
	_, err = store.GetDefinition(def.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestJSONStoreInstance(t *testing.T) {
	dir := t.TempDir()
	store := NewJSONStore(dir)

	def := &Definition{
		Name:     "wf1",
		Version:  1,
		TenantID: "t1",
		Variables: []Variable{
			{Name: "input", Type: "string", Required: true},
			{Name: "count", Type: "number", DefaultValue: 5},
		},
	}
	store.SaveDefinition(def)

	// Create instance with partial variables
	inst, err := store.CreateInstance(def.ID, "t1", map[string]any{"input": "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if inst.ID == "" {
		t.Error("instance ID should be set")
	}
	if inst.Status != InstancePending {
		t.Errorf("Status = %v, want pending", inst.Status)
	}
	// Default value applied
	if v, ok := inst.Variables["count"]; !ok || v != 5 && v != float64(5) {
		t.Errorf("count default = %v, want 5", inst.Variables["count"])
	}
	if inst.Variables["input"] != "hello" {
		t.Errorf("input = %v, want hello", inst.Variables["input"])
	}

	// Get
	got, err := store.GetInstance(inst.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.DefinitionID != def.ID {
		t.Error("DefinitionID mismatch")
	}

	// Save update
	inst.Status = InstanceRunning
	if err := store.SaveInstance(inst); err != nil {
		t.Fatal(err)
	}

	// List
	insts, err := store.ListInstances("t1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(insts) != 1 {
		t.Errorf("ListInstances = %d, want 1", len(insts))
	}

	// Get nonexistent
	_, err = store.GetInstance("nope")
	if err == nil {
		t.Error("expected error for nonexistent instance")
	}
}

func TestJSONStoreCreateInstanceNoDef(t *testing.T) {
	dir := t.TempDir()
	store := NewJSONStore(dir)

	_, err := store.CreateInstance("nonexistent", "t1", nil)
	if err == nil {
		t.Error("expected error for nonexistent definition")
	}
}

func TestJSONStoreListLimit(t *testing.T) {
	dir := t.TempDir()
	store := NewJSONStore(dir)

	def := &Definition{Name: "wf", Version: 1}
	store.SaveDefinition(def)

	for i := 0; i < 5; i++ {
		store.CreateInstance(def.ID, "t1", nil)
	}

	insts, _ := store.ListInstances("t1", 3)
	if len(insts) != 3 {
		t.Errorf("ListInstances(limit=3) = %d, want 3", len(insts))
	}

	insts, _ = store.ListInstances("t1", 0)
	if len(insts) != 5 {
		t.Errorf("ListInstances(limit=0) = %d, want 5", len(insts))
	}
}

func TestNodeTypes(t *testing.T) {
	types := []NodeType{
		NodeSkill, NodeLLM, NodeCondition, NodeParallel, NodeJoin,
		NodeSubflow, NodeInput, NodeTransform, NodeBrowser, NodeCode, NodeKnowledge,
	}
	for _, nt := range types {
		if nt == "" {
			t.Error("node type should not be empty")
		}
	}
}

func TestInstanceStatuses(t *testing.T) {
	statuses := []InstanceStatus{
		InstancePending, InstanceRunning, InstancePaused,
		InstanceCompleted, InstanceFailed, InstanceCancelled,
	}
	for _, s := range statuses {
		if s == "" {
			t.Error("instance status should not be empty")
		}
	}
}

func TestJSONStorePersistence(t *testing.T) {
	dir := t.TempDir()

	// Create and save
	store1 := NewJSONStore(dir)
	def := &Definition{Name: "persist-test", Version: 1, TenantID: "t1"}
	store1.SaveDefinition(def)

	// Reload from disk
	store2 := NewJSONStore(dir)
	got, err := store2.GetDefinition(def.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "persist-test" {
		t.Errorf("reloaded Name = %q, want persist-test", got.Name)
	}
}
