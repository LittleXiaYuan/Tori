package identity

import (
	"testing"
	"time"
)

func TestGenerateAndBind(t *testing.T) {
	store := NewBindingStore()

	code, err := store.GenerateCode("user1", "telegram")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(code.Code) != bindCodeLength {
		t.Fatalf("expected code length %d, got %d", bindCodeLength, len(code.Code))
	}

	binding, err := store.Bind(code.Code, "tg_12345", "Alice")
	if err != nil {
		t.Fatalf("bind: %v", err)
	}
	if binding.UserID != "user1" {
		t.Fatalf("expected user1, got %s", binding.UserID)
	}
	if binding.Platform != "telegram" {
		t.Fatalf("expected telegram, got %s", binding.Platform)
	}
	if binding.ExternalID != "tg_12345" {
		t.Fatalf("expected tg_12345, got %s", binding.ExternalID)
	}
}

func TestBindCodeExpired(t *testing.T) {
	store := NewBindingStore()

	code, _ := store.GenerateCode("user1", "discord")
	// Manually expire it
	store.mu.Lock()
	store.codes[code.Code].ExpiresAt = time.Now().Add(-time.Second)
	store.mu.Unlock()

	_, err := store.Bind(code.Code, "dc_999", "Bob")
	if err == nil {
		t.Fatal("expected error for expired code")
	}
}

func TestBindCodeInvalid(t *testing.T) {
	store := NewBindingStore()

	_, err := store.Bind("nonexistent", "ext1", "Name")
	if err == nil {
		t.Fatal("expected error for invalid code")
	}
}

func TestBindCodeConsumed(t *testing.T) {
	store := NewBindingStore()

	code, _ := store.GenerateCode("user1", "feishu")
	store.Bind(code.Code, "fs_111", "Charlie")

	// Second use should fail
	_, err := store.Bind(code.Code, "fs_222", "Dave")
	if err == nil {
		t.Fatal("expected error for consumed code")
	}
}

func TestResolve(t *testing.T) {
	store := NewBindingStore()

	code, _ := store.GenerateCode("user1", "telegram")
	store.Bind(code.Code, "tg_12345", "Alice")

	code2, _ := store.GenerateCode("user1", "discord")
	store.Bind(code2.Code, "dc_67890", "Alice")

	// Resolve from telegram
	uid, ok := store.Resolve("telegram", "tg_12345")
	if !ok || uid != "user1" {
		t.Fatalf("expected user1, got %s ok=%v", uid, ok)
	}

	// Resolve from discord
	uid, ok = store.Resolve("discord", "dc_67890")
	if !ok || uid != "user1" {
		t.Fatalf("expected user1, got %s ok=%v", uid, ok)
	}

	// Unknown
	_, ok = store.Resolve("slack", "unknown")
	if ok {
		t.Fatal("expected not found")
	}
}

func TestGetBindings(t *testing.T) {
	store := NewBindingStore()

	c1, _ := store.GenerateCode("user1", "telegram")
	store.Bind(c1.Code, "tg_1", "A")

	c2, _ := store.GenerateCode("user1", "discord")
	store.Bind(c2.Code, "dc_1", "A")

	bindings := store.GetBindings("user1")
	if len(bindings) != 2 {
		t.Fatalf("expected 2 bindings, got %d", len(bindings))
	}
}

func TestUnbind(t *testing.T) {
	store := NewBindingStore()

	c, _ := store.GenerateCode("user1", "telegram")
	store.Bind(c.Code, "tg_1", "A")

	ok := store.Unbind("user1", "telegram", "tg_1")
	if !ok {
		t.Fatal("expected successful unbind")
	}

	bindings := store.GetBindings("user1")
	if len(bindings) != 0 {
		t.Fatalf("expected 0 bindings after unbind, got %d", len(bindings))
	}

	// Resolve should fail
	_, found := store.Resolve("telegram", "tg_1")
	if found {
		t.Fatal("expected not found after unbind")
	}
}

func TestDuplicateBinding(t *testing.T) {
	store := NewBindingStore()

	c1, _ := store.GenerateCode("user1", "telegram")
	store.Bind(c1.Code, "tg_1", "A")

	// Bind same external ID again
	c2, _ := store.GenerateCode("user1", "telegram")
	b, err := store.Bind(c2.Code, "tg_1", "A")
	if err != nil {
		t.Fatalf("duplicate bind should succeed: %v", err)
	}
	if b.ExternalID != "tg_1" {
		t.Fatal("expected same binding returned")
	}

	// Should still be only 1 binding
	bindings := store.GetBindings("user1")
	if len(bindings) != 1 {
		t.Fatalf("expected 1 binding (no dup), got %d", len(bindings))
	}
}

func TestGenerateCodeValidation(t *testing.T) {
	store := NewBindingStore()

	_, err := store.GenerateCode("", "telegram")
	if err == nil {
		t.Fatal("expected error for empty user_id")
	}

	_, err = store.GenerateCode("user1", "")
	if err == nil {
		t.Fatal("expected error for empty platform")
	}
}
