package models

import (
	"testing"
)

func TestRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	m := Model{ID: "m1", ModelID: "gpt-4o", Name: "GPT-4o", Type: TypeChat, ClientType: ClientOpenAI}
	if err := r.Register(m); err != nil {
		t.Fatalf("register: %v", err)
	}
	got, ok := r.Get("m1")
	if !ok || got.ModelID != "gpt-4o" {
		t.Fatalf("get: %+v", got)
	}
}

func TestRegisterEmptyID(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(Model{}); err == nil {
		t.Fatal("expected error for empty id")
	}
}

func TestGetByModelID(t *testing.T) {
	r := NewRegistry()
	r.Register(Model{ID: "m1", ModelID: "claude-3", Type: TypeChat, ClientType: ClientAnthropic})
	got, ok := r.GetByModelID("claude-3")
	if !ok || got.ID != "m1" {
		t.Fatal("GetByModelID failed")
	}
	_, ok = r.GetByModelID("nonexistent")
	if ok {
		t.Fatal("should not find nonexistent")
	}
}

func TestPrimary(t *testing.T) {
	r := NewRegistry()
	_, ok := r.Primary()
	if ok {
		t.Fatal("no primary initially")
	}
	r.Register(Model{ID: "m1", ModelID: "gpt-4", Type: TypeChat, ClientType: ClientOpenAI})
	p, ok := r.Primary()
	if !ok || p.ID != "m1" {
		t.Fatal("first chat model should be primary")
	}
}

func TestSetPrimary(t *testing.T) {
	r := NewRegistry()
	r.Register(Model{ID: "m1", ModelID: "gpt-4", Type: TypeChat, ClientType: ClientOpenAI})
	r.Register(Model{ID: "m2", ModelID: "claude", Type: TypeChat, ClientType: ClientAnthropic})
	if !r.SetPrimary("m2") {
		t.Fatal("set primary failed")
	}
	p, _ := r.Primary()
	if p.ID != "m2" {
		t.Fatal("primary should be m2")
	}
	if r.SetPrimary("nonexistent") {
		t.Fatal("should fail for nonexistent")
	}
}

func TestListByType(t *testing.T) {
	r := NewRegistry()
	r.Register(Model{ID: "m1", ModelID: "gpt-4", Type: TypeChat, ClientType: ClientOpenAI})
	r.Register(Model{ID: "e1", ModelID: "text-embedding", Type: TypeEmbedding, Dimensions: 1536})

	chat := r.List(TypeChat)
	if len(chat) != 1 || chat[0].Type != TypeChat {
		t.Fatalf("chat list: %+v", chat)
	}
	emb := r.List(TypeEmbedding)
	if len(emb) != 1 {
		t.Fatalf("embedding list: %+v", emb)
	}
	all := r.List("")
	if len(all) != 2 {
		t.Fatalf("all list: %d", len(all))
	}
}

func TestRemove(t *testing.T) {
	r := NewRegistry()
	r.Register(Model{ID: "m1", ModelID: "gpt-4", Type: TypeChat, ClientType: ClientOpenAI})
	if !r.Remove("m1") {
		t.Fatal("remove failed")
	}
	if r.Count() != 0 {
		t.Fatal("should be empty")
	}
	if r.Remove("nonexistent") {
		t.Fatal("should fail")
	}
}

func TestRemoveClearsPrimary(t *testing.T) {
	r := NewRegistry()
	r.Register(Model{ID: "m1", ModelID: "gpt-4", Type: TypeChat, ClientType: ClientOpenAI})
	r.Remove("m1")
	_, ok := r.Primary()
	if ok {
		t.Fatal("primary should be cleared")
	}
}

func TestModelModality(t *testing.T) {
	m := Model{InputModalities: []string{"text", "image"}}
	if !m.HasModality("image") {
		t.Fatal("should have image")
	}
	if m.HasModality("audio") {
		t.Fatal("should not have audio")
	}
	if !m.IsMultimodal() {
		t.Fatal("should be multimodal")
	}
	m2 := Model{InputModalities: []string{"text"}}
	if m2.IsMultimodal() {
		t.Fatal("text-only should not be multimodal")
	}
}
