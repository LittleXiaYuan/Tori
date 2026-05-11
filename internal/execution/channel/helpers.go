package channel

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"yunque-agent/pkg/safego"
)

// ──────────────────────────────────────────────
// Shared Channel Utilities
// ──────────────────────────────────────────────

// SplitMessage splits a text message into chunks that fit within maxLen runes.
// It tries to break at natural boundaries (newlines, sentence endings).
// If maxLen <= 0, it returns the text as a single part.
func SplitMessage(text string, maxLen int) []string {
	if maxLen <= 0 || utf8.RuneCountInString(text) <= maxLen {
		return []string{text}
	}

	var parts []string
	runes := []rune(text)
	for len(runes) > 0 {
		end := maxLen
		if end > len(runes) {
			end = len(runes)
		}

		// Try to split at sentence/paragraph boundary
		if end < len(runes) {
			chunk := runes[:end]
			// Search within last ~200 runes for a natural break
			searchStart := end - 200
			if searchStart < 0 {
				searchStart = 0
			}
			for i := end - 1; i > searchStart; i-- {
				c := chunk[i]
				if c == '\n' || c == '。' || c == '.' || c == '！' || c == '？' || c == '!' || c == '?' {
					end = i + 1
					break
				}
			}
		}

		parts = append(parts, string(runes[:end]))
		runes = runes[end:]
	}
	return parts
}

// SplitMessageBytes splits text by byte length using string-based separators.
// Useful for platforms that measure limits in bytes rather than runes.
func SplitMessageBytes(text string, maxLen int) []string {
	if maxLen <= 0 || len(text) <= maxLen {
		return []string{text}
	}
	var parts []string
	for len(text) > 0 {
		if len(text) <= maxLen {
			parts = append(parts, text)
			break
		}
		cutAt := maxLen
		for _, sep := range []string{"\n\n", "\n", "。", ".", "！", "!", "？", "?"} {
			idx := strings.LastIndex(text[:maxLen], sep)
			if idx > maxLen/2 {
				cutAt = idx + len(sep)
				break
			}
		}
		parts = append(parts, text[:cutAt])
		text = text[cutAt:]
	}
	return parts
}

// ──────────────────────────────────────────────
// Webhook Server Helper
// ──────────────────────────────────────────────

// WebhookServer wraps the common pattern of starting an HTTP server for webhook callbacks
// and processing messages from a buffered channel.
type WebhookServer struct {
	Addr string
	Mux  *http.ServeMux
	srv  *http.Server
}

// NewWebhookServer creates a webhook server ready to start.
func NewWebhookServer(bindAddr, port string) *WebhookServer {
	mux := http.NewServeMux()
	addr := bindAddr + ":" + port
	return &WebhookServer{
		Addr: addr,
		Mux:  mux,
		srv: &http.Server{
			Addr:              addr,
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
		},
	}
}

// Serve starts the HTTP server and blocks until ctx is cancelled.
// Returns nil if the server was shut down cleanly via context cancellation.
func (ws *WebhookServer) Serve(ctx context.Context) error {
	safego.Go("webhook-shutdown-watcher", func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = ws.srv.Shutdown(shutCtx)
	})

	err := ws.srv.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// ──────────────────────────────────────────────
// Duplicate Message Tracker
// ──────────────────────────────────────────────

// DuplicateTracker tracks recently seen message IDs to filter duplicates.
type DuplicateTracker struct {
	mu   sync.Mutex
	seen map[string]time.Time
	ttl  time.Duration
}

// NewDuplicateTracker creates a tracker with the given TTL for seen IDs.
func NewDuplicateTracker(ttl time.Duration) *DuplicateTracker {
	return &DuplicateTracker{
		seen: make(map[string]time.Time),
		ttl:  ttl,
	}
}

// IsDuplicate returns true if the messageID was already seen within the TTL window.
// It also cleans up expired entries.
func (dt *DuplicateTracker) IsDuplicate(messageID string) bool {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	now := time.Now()
	// Cleanup expired entries
	for k, t := range dt.seen {
		if now.Sub(t) > dt.ttl {
			delete(dt.seen, k)
		}
	}

	if _, ok := dt.seen[messageID]; ok {
		return true
	}
	dt.seen[messageID] = now
	return false
}

// ──────────────────────────────────────────────
// Token Manager for OAuth-style tokens
// ──────────────────────────────────────────────

// TokenRefreshFunc is a function that fetches a new token and its expiry duration.
type TokenRefreshFunc func(ctx context.Context) (token string, expiresIn time.Duration, err error)

// TokenManager manages access tokens with automatic refresh.
type TokenManager struct {
	mu        sync.RWMutex
	token     string
	expiresAt time.Time
	refresh   TokenRefreshFunc
}

// NewTokenManager creates a TokenManager with the given refresh function.
func NewTokenManager(refreshFn TokenRefreshFunc) *TokenManager {
	return &TokenManager{refresh: refreshFn}
}

// Get returns the current valid token, refreshing if needed.
func (tm *TokenManager) Get(ctx context.Context) (string, error) {
	tm.mu.RLock()
	if tm.token != "" && time.Now().Before(tm.expiresAt) {
		t := tm.token
		tm.mu.RUnlock()
		return t, nil
	}
	tm.mu.RUnlock()

	return tm.ForceRefresh(ctx)
}

// ForceRefresh refreshes the token regardless of expiry.
func (tm *TokenManager) ForceRefresh(ctx context.Context) (string, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Double-check after acquiring write lock
	if tm.token != "" && time.Now().Before(tm.expiresAt) {
		return tm.token, nil
	}

	token, expiresIn, err := tm.refresh(ctx)
	if err != nil {
		return "", err
	}
	tm.token = token
	tm.expiresAt = time.Now().Add(expiresIn)
	return token, nil
}

// StartRefreshLoop starts a background loop that refreshes the token at the given interval.
func (tm *TokenManager) StartRefreshLoop(ctx context.Context, interval time.Duration) {
	safego.Go("token-refresh-loop", func() {
		// Initial refresh
		if _, err := tm.ForceRefresh(ctx); err != nil {
			slog.Warn("token initial refresh failed", "err", err)
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := tm.ForceRefresh(ctx); err != nil {
					slog.Warn("token refresh failed", "err", err)
				}
			}
		}
	})
}

// ──────────────────────────────────────────────
// JSON API Call Helper
// ──────────────────────────────────────────────

// CallJSONAPI makes a JSON POST request and returns the response body.
func CallJSONAPI(ctx context.Context, client *http.Client, method, url string, body any, headers map[string]string) ([]byte, int, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("new request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}

	return respBody, resp.StatusCode, nil
}

// ──────────────────────────────────────────────
// File / Image Component Loader
// ──────────────────────────────────────────────

// LoadComponentBytes loads raw bytes from a Component (ImageComponent or FileComponent).
// It resolves the source in priority order: Base64 → file:// path → http(s) URL.
// Returns (data, filename, mimeType, error).
func LoadComponentBytes(ctx context.Context, client *http.Client, comp Component) (data []byte, filename, mimeType string, err error) {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	switch c := comp.(type) {
	case *ImageComponent:
		filename = "image.png"
		mimeType = "image/png"
		if c.Alt != "" {
			filename = sanitizeFilename(c.Alt) + ".png"
		}
		if c.Base64 != "" {
			data, err = base64.StdEncoding.DecodeString(c.Base64)
			return
		}
		if c.URL != "" {
			data, filename, mimeType, err = loadFromURL(ctx, client, c.URL, filename)
			return
		}
		err = fmt.Errorf("image component has no data source")

	case *FileComponent:
		filename = c.FileName
		if filename == "" {
			filename = "file"
		}
		mimeType = c.MimeType
		if mimeType == "" {
			mimeType = mime.TypeByExtension(filepath.Ext(filename))
		}
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		if c.URL != "" {
			data, filename, mimeType, err = loadFromURL(ctx, client, c.URL, filename)
			return
		}
		err = fmt.Errorf("file component has no URL")

	default:
		err = fmt.Errorf("unsupported component type: %T", comp)
	}
	return
}

// loadFromURL fetches bytes from an HTTP URL or a file:// path.
func loadFromURL(ctx context.Context, client *http.Client, rawURL, defaultFilename string) ([]byte, string, string, error) {
	filename := defaultFilename

	if strings.HasPrefix(rawURL, "file://") {
		path := strings.TrimPrefix(rawURL, "file://")
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, "", "", fmt.Errorf("read file %q: %w", path, err)
		}
		name := filepath.Base(path)
		if name != "" && name != "." {
			filename = name
		}
		mt := mime.TypeByExtension(filepath.Ext(filename))
		if mt == "" {
			mt = "application/octet-stream"
		}
		return data, filename, mt, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", "", fmt.Errorf("build request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", "", fmt.Errorf("fetch %q: %w", rawURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", "", fmt.Errorf("fetch %q status %d", rawURL, resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 50<<20)) // 50 MB cap
	if err != nil {
		return nil, "", "", fmt.Errorf("read body: %w", err)
	}

	// Try to get filename from Content-Disposition
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if _, params, e := mime.ParseMediaType(cd); e == nil {
			if fn := params["filename"]; fn != "" {
				filename = sanitizeFilename(fn)
			}
		}
	}
	mt := resp.Header.Get("Content-Type")
	if mt == "" {
		mt = mime.TypeByExtension(filepath.Ext(filename))
	}
	if mt == "" {
		mt = "application/octet-stream"
	}
	// Strip parameters (e.g. "image/jpeg; charset=utf-8" → "image/jpeg")
	if idx := strings.Index(mt, ";"); idx > 0 {
		mt = strings.TrimSpace(mt[:idx])
	}
	return data, filename, mt, nil
}

func sanitizeFilename(name string) string {
	// Replace characters that are invalid in filenames
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_",
		"?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	cleaned := replacer.Replace(name)
	if cleaned == "" {
		return "file"
	}
	return cleaned
}

// ──────────────────────────────────────────────
// Reply Helpers
// ──────────────────────────────────────────────

// IsEmptyReply returns true if the reply has no meaningful content after button fallback.
// Use in handler callbacks to skip sending blank replies.
func IsEmptyReply(reply Reply) bool {
	return strings.TrimSpace(ContentWithButtonFallback(reply)) == ""
}

// ──────────────────────────────────────────────
// Non-blocking Channel Send
// ──────────────────────────────────────────────

// TrySendMessage attempts a non-blocking send to a message channel.
// Returns false if the channel is full.
func TrySendMessage(ch chan<- Message, msg Message, channelType string) bool {
	select {
	case ch <- msg:
		return true
	default:
		slog.Warn("message channel full, dropping message", "channel", channelType, "user_id", msg.UserID)
		return false
	}
}
