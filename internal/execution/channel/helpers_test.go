package channel

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"
)

// ──────────────────────────────────────────────
// SplitMessage Tests
// ──────────────────────────────────────────────

func TestSplitMessageEmpty(t *testing.T) {
	parts := SplitMessage("", 100)
	if len(parts) != 1 || parts[0] != "" {
		t.Errorf("expected [''], got %v", parts)
	}
}

func TestSplitMessageShort(t *testing.T) {
	parts := SplitMessage("hello world", 100)
	if len(parts) != 1 || parts[0] != "hello world" {
		t.Errorf("expected single part, got %v", parts)
	}
}

func TestSplitMessageExactLimit(t *testing.T) {
	text := "abcde" // 5 runes
	parts := SplitMessage(text, 5)
	if len(parts) != 1 || parts[0] != text {
		t.Errorf("expected single part at exact limit, got %v", parts)
	}
}

func TestSplitMessageLong(t *testing.T) {
	// 100 chars, split at 30
	text := "这是一段很长的中文文字。这里有一个句号。然后继续写下去，直到超过三十个字符长度再停止"
	parts := SplitMessage(text, 20)
	if len(parts) < 2 {
		t.Errorf("expected multiple parts, got %d", len(parts))
	}
	// Reconstruct
	var rebuilt string
	for _, p := range parts {
		rebuilt += p
	}
	if rebuilt != text {
		t.Errorf("reconstructed text mismatch")
	}
}

func TestSplitMessageSentenceBoundary(t *testing.T) {
	text := "第一句话。第二句话。第三句话"
	parts := SplitMessage(text, 8) // Should split at sentence boundaries
	if len(parts) < 2 {
		t.Errorf("expected split at sentence boundary, got %d parts", len(parts))
	}
}

func TestSplitMessageNewline(t *testing.T) {
	text := "line one\nline two\nline three"
	parts := SplitMessage(text, 15)
	if len(parts) < 2 {
		t.Fatalf("expected 2+ parts, got %d", len(parts))
	}
}

func TestSplitMessageZeroLimit(t *testing.T) {
	parts := SplitMessage("hello", 0)
	if len(parts) != 1 || parts[0] != "hello" {
		t.Errorf("zero limit should return original, got %v", parts)
	}
}

func TestSplitMessageCJK(t *testing.T) {
	// 10 CJK characters, each 3 bytes but 1 rune
	text := "一二三四五六七八九十"
	parts := SplitMessage(text, 5)
	if len(parts) != 2 {
		t.Errorf("expected 2 parts for 10 CJK runes at limit 5, got %d", len(parts))
	}
}

// ──────────────────────────────────────────────
// SplitMessageBytes Tests
// ──────────────────────────────────────────────

func TestSplitMessageBytesShort(t *testing.T) {
	parts := SplitMessageBytes("hello", 100)
	if len(parts) != 1 {
		t.Errorf("expected 1 part, got %d", len(parts))
	}
}

func TestSplitMessageBytesLong(t *testing.T) {
	text := "First paragraph.\n\nSecond paragraph.\n\nThird paragraph."
	parts := SplitMessageBytes(text, 30)
	if len(parts) < 2 {
		t.Errorf("expected 2+ parts, got %d", len(parts))
	}
	// Reconstruct
	var rebuilt string
	for _, p := range parts {
		rebuilt += p
	}
	if rebuilt != text {
		t.Errorf("reconstructed text mismatch")
	}
}

func TestSplitMessageBytesZero(t *testing.T) {
	parts := SplitMessageBytes("hello", 0)
	if len(parts) != 1 {
		t.Errorf("zero limit should return original, got %d parts", len(parts))
	}
}

// ──────────────────────────────────────────────
// WebhookServer Tests
// ──────────────────────────────────────────────

func TestNewWebhookServer(t *testing.T) {
	ws := NewWebhookServer("127.0.0.1", "0")
	if ws.Addr != "127.0.0.1:0" {
		t.Errorf("expected 127.0.0.1:0, got %s", ws.Addr)
	}
	if ws.Mux == nil {
		t.Error("mux should not be nil")
	}
}

func TestWebhookServerServe(t *testing.T) {
	ws := NewWebhookServer("127.0.0.1", "0")

	called := make(chan bool, 1)
	ws.Mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		called <- true
	})

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		// Give server time to start
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := ws.Serve(ctx)
	if err != nil {
		t.Errorf("expected nil error on clean shutdown, got %v", err)
	}
}

// ──────────────────────────────────────────────
// DuplicateTracker Tests
// ──────────────────────────────────────────────

func TestDuplicateTrackerNew(t *testing.T) {
	dt := NewDuplicateTracker(time.Minute)
	if dt == nil {
		t.Fatal("tracker should not be nil")
	}
}

func TestDuplicateTrackerFirstSeen(t *testing.T) {
	dt := NewDuplicateTracker(time.Minute)
	if dt.IsDuplicate("msg1") {
		t.Error("first time should not be duplicate")
	}
}

func TestDuplicateTrackerSecondSeen(t *testing.T) {
	dt := NewDuplicateTracker(time.Minute)
	dt.IsDuplicate("msg1")
	if !dt.IsDuplicate("msg1") {
		t.Error("second time should be duplicate")
	}
}

func TestDuplicateTrackerDifferentIDs(t *testing.T) {
	dt := NewDuplicateTracker(time.Minute)
	dt.IsDuplicate("msg1")
	if dt.IsDuplicate("msg2") {
		t.Error("different ID should not be duplicate")
	}
}

func TestDuplicateTrackerExpiry(t *testing.T) {
	dt := NewDuplicateTracker(10 * time.Millisecond)
	dt.IsDuplicate("msg1")
	time.Sleep(20 * time.Millisecond)
	if dt.IsDuplicate("msg1") {
		t.Error("expired entry should not be duplicate")
	}
}

func TestDuplicateTrackerConcurrency(t *testing.T) {
	dt := NewDuplicateTracker(time.Minute)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			dt.IsDuplicate(id)
		}("msg" + string(rune('0'+i%10)))
	}
	wg.Wait()
}

// ──────────────────────────────────────────────
// TokenManager Tests
// ──────────────────────────────────────────────

func TestTokenManagerGet(t *testing.T) {
	calls := 0
	tm := NewTokenManager(func(ctx context.Context) (string, time.Duration, error) {
		calls++
		return "token123", 10 * time.Second, nil
	})

	tok, err := tm.Get(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok != "token123" {
		t.Errorf("expected token123, got %s", tok)
	}

	// Second call should use cached value
	tok2, _ := tm.Get(context.Background())
	if tok2 != "token123" {
		t.Errorf("expected cached token123, got %s", tok2)
	}
	if calls != 1 {
		t.Errorf("expected 1 refresh call, got %d", calls)
	}
}

func TestTokenManagerExpiry(t *testing.T) {
	calls := 0
	tm := NewTokenManager(func(ctx context.Context) (string, time.Duration, error) {
		calls++
		return "token" + string(rune('0'+calls)), 10 * time.Millisecond, nil
	})

	tm.Get(context.Background())
	time.Sleep(20 * time.Millisecond)
	tok, _ := tm.Get(context.Background())
	if calls < 2 {
		t.Errorf("expected token refresh after expiry, calls=%d", calls)
	}
	if tok == "" {
		t.Error("token should not be empty")
	}
}

func TestTokenManagerForceRefresh(t *testing.T) {
	calls := 0
	tm := NewTokenManager(func(ctx context.Context) (string, time.Duration, error) {
		calls++
		return "tok", time.Hour, nil
	})

	tm.Get(context.Background())
	tm.ForceRefresh(context.Background())
	// ForceRefresh should skip due to double-check (still valid)
	if calls != 1 {
		t.Errorf("expected 1 call (double-check prevents re-fetch), got %d", calls)
	}
}

func TestTokenManagerConcurrency(t *testing.T) {
	tm := NewTokenManager(func(ctx context.Context) (string, time.Duration, error) {
		return "tok", time.Minute, nil
	})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tm.Get(context.Background())
		}()
	}
	wg.Wait()
}

// ──────────────────────────────────────────────
// TrySendMessage Tests
// ──────────────────────────────────────────────

func TestTrySendMessageSuccess(t *testing.T) {
	ch := make(chan Message, 1)
	msg := Message{ChannelType: "test", UserID: "u1"}
	if !TrySendMessage(ch, msg, "test") {
		t.Error("should succeed on non-full channel")
	}
	if len(ch) != 1 {
		t.Error("message should be in channel")
	}
}

func TestTrySendMessageFull(t *testing.T) {
	ch := make(chan Message, 1)
	ch <- Message{} // fill it
	msg := Message{ChannelType: "test", UserID: "u2"}
	if TrySendMessage(ch, msg, "test") {
		t.Error("should fail on full channel")
	}
}

// ──────────────────────────────────────────────
// CallJSONAPI Tests
// ──────────────────────────────────────────────

func TestCallJSONAPINilBody(t *testing.T) {
	// Test with nil body (GET request)
	ctx := context.Background()
	client := &http.Client{Timeout: 2 * time.Second}
	// This will fail to connect but should not panic
	_, _, err := CallJSONAPI(ctx, client, http.MethodGet, "http://127.0.0.1:1/nonexistent", nil, nil)
	if err == nil {
		t.Error("expected connection error")
	}
}
