package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func tempDB(t *testing.T) *SQLite {
	t.Helper()
	dir := t.TempDir()
	db, err := New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestMemoryPutGet(t *testing.T) {
	db := tempDB(t)
	ctx := context.Background()

	item := MemoryItem{
		ID: "m1", Key: "user.name", Value: "Alice", Source: "chat",
		Category: "fact", CreatedAt: time.Now(),
	}
	if err := db.MemoryPut(ctx, "t1", item); err != nil {
		t.Fatal(err)
	}
	got, err := db.MemoryGet(ctx, "t1", "user.name")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Value != "Alice" {
		t.Fatalf("expected Alice, got %v", got)
	}
}

func TestMemorySearch(t *testing.T) {
	db := tempDB(t)
	ctx := context.Background()

	db.MemoryPut(ctx, "t1", MemoryItem{ID: "1", Key: "color", Value: "blue is favorite", CreatedAt: time.Now()})
	db.MemoryPut(ctx, "t1", MemoryItem{ID: "2", Key: "food", Value: "likes pizza", CreatedAt: time.Now()})

	results, err := db.MemorySearch(ctx, "t1", "blue", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestMemoryDelete(t *testing.T) {
	db := tempDB(t)
	ctx := context.Background()

	db.MemoryPut(ctx, "t1", MemoryItem{ID: "1", Key: "k1", Value: "v1", CreatedAt: time.Now()})
	db.MemoryDelete(ctx, "t1", "k1")
	got, _ := db.MemoryGet(ctx, "t1", "k1")
	if got != nil {
		t.Fatal("expected nil after delete")
	}
}

func TestSessionRepo(t *testing.T) {
	db := tempDB(t)
	ctx := context.Background()

	if err := db.GetOrCreate(ctx, "s1", "t1"); err != nil {
		t.Fatal(err)
	}
	if err := db.Append(ctx, "s1", "user", "hello"); err != nil {
		t.Fatal(err)
	}
	if err := db.Append(ctx, "s1", "assistant", "hi there"); err != nil {
		t.Fatal(err)
	}

	msgs, err := db.GetMessages(ctx, "s1", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[1].Role != "assistant" {
		t.Fatal("message order wrong")
	}
}

func TestBotPersistence(t *testing.T) {
	db := tempDB(t)
	ctx := context.Background()

	bot := BotRow{
		ID: "b1", Name: "TestBot", Description: "test", Status: "idle",
		IsActive: true, Config: "{}", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := db.PutBot(ctx, bot); err != nil {
		t.Fatal(err)
	}
	got, err := db.GetBot(ctx, "b1")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Name != "TestBot" {
		t.Fatal("bot not found")
	}

	bots, err := db.ListBots(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(bots) != 1 {
		t.Fatalf("expected 1 bot, got %d", len(bots))
	}

	db.DeleteBot(ctx, "b1")
	got, _ = db.GetBot(ctx, "b1")
	if got != nil {
		t.Fatal("expected nil after delete")
	}
}

func TestInboxPersistence(t *testing.T) {
	db := tempDB(t)
	ctx := context.Background()

	db.PushInbox(ctx, InboxRow{ID: "i1", Source: "telegram", Content: "msg1", CreatedAt: time.Now()})
	db.PushInbox(ctx, InboxRow{ID: "i2", Source: "email", Content: "msg2", CreatedAt: time.Now()})

	items, err := db.ListInbox(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2, got %d", len(items))
	}

	total, unread, err := db.InboxCounts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || unread != 2 {
		t.Fatalf("counts wrong: total=%d unread=%d", total, unread)
	}

	n, err := db.MarkAllInboxRead(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("expected 2 marked, got %d", n)
	}
}

func TestModelPersistence(t *testing.T) {
	db := tempDB(t)
	ctx := context.Background()

	m := ModelRow{
		ID: "m1", ModelID: "gpt-4o", Name: "GPT-4o", Type: "chat",
		ClientType: "openai", InputModalities: `["text","image"]`,
		SupportsReasoning: true, CreatedAt: time.Now(),
	}
	if err := db.PutModel(ctx, m); err != nil {
		t.Fatal(err)
	}

	models, err := db.ListModels(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].ModelID != "gpt-4o" {
		t.Fatal("model not found")
	}

	db.DeleteModel(ctx, "m1")
	models, _ = db.ListModels(ctx)
	if len(models) != 0 {
		t.Fatal("expected 0 after delete")
	}
}

func TestNewCreatesDir(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "sub", "deep", "test.db")
	os.MkdirAll(filepath.Dir(dbPath), 0755)
	db, err := New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
}
