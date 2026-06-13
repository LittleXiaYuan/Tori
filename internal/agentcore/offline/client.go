// Package offline drives the local background inference engine — 小羽 (RWKV-7)
// running on Ascend NPUs. It is the "slow brain": a latency-tolerant, zero
// API-cost model that powers the CogniKernel dreaming / self-evolution loops.
//
// Unlike the front-stage llm.Client (stateless OpenAI-compatible chat), RWKV-7
// exposes O(1) recurrent state, so this driver speaks a stateful contract:
// POST /v1/reverie/dream carries a SessionID (the RWKV state handle) plus the
// previous day's reflection-failure log, and returns distilled Experiences
// together with the next state handle for continuity across cycles.
//
// The driver never sits on the user-facing path; only the dreaming kernel calls
// it, and the gateway hard-blocks any front-stage request that targets it.
package offline

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// Experience is a distilled lesson returned by a dream cycle. Field names match
// the durable experience store so the wiring layer can map it 1:1.
type Experience struct {
	Category string   `json:"category"` // skill_usage | error_pattern | strategy | domain | preference
	Outcome  string   `json:"outcome"`  // success | failure | partial
	Lesson   string   `json:"lesson"`   // the insight extracted
	Context  string   `json:"context"`  // what was being chewed on
	Tags     []string `json:"tags,omitempty"`
}

// DreamRequest is a stateful reverie invocation against the offline engine.
type DreamRequest struct {
	// SessionID is the RWKV-7 recurrent-state handle. Pass the handle returned
	// by the previous Dream call to continue from yesterday's mental state and
	// exploit O(1) memory continuity; empty starts a fresh state.
	SessionID string `json:"session_id"`
	TenantID  string `json:"tenant_id,omitempty"`
	// LogEvent is the previous day's reflection-failure log for the engine to
	// chew on and distill into actionable experience.
	LogEvent  string   `json:"log_event"`
	Hints     []string `json:"hints,omitempty"`
	MaxTokens int      `json:"max_tokens,omitempty"`
}

// DreamResponse is the distilled output of one dream cycle.
type DreamResponse struct {
	// SessionID is the next RWKV-7 state handle to persist for the following
	// cycle (O(1) state complexity → cheap durable continuity).
	SessionID   string       `json:"session_id"`
	Experiences []Experience `json:"experiences"`
	Summary     string       `json:"summary,omitempty"`
}

// XiaoyuClient is the HTTP driver for the local 小羽 (RWKV-7) reverie service.
type XiaoyuClient struct {
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
	timeout time.Duration
}

// NewXiaoyuClient builds a driver for the offline reverie service. baseURL is
// the service root (e.g. http://127.0.0.1:8900); the dream endpoint is resolved
// relative to it. No global HTTP timeout is set — generation can run for a long
// time, so deadlines come from the context (or the long default in Dream).
func NewXiaoyuClient(baseURL, apiKey, model string) *XiaoyuClient {
	if strings.TrimSpace(model) == "" {
		model = "xiaoyu-rwkv7"
	}
	return &XiaoyuClient{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		apiKey:  apiKey,
		model:   model,
		timeout: 30 * time.Minute,
		http: &http.Client{Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   15 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          10,
			MaxIdleConnsPerHost:   4,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   15 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			// Local model may "think" for a long time before the first byte.
			ResponseHeaderTimeout: 30 * time.Minute,
			TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		}},
	}
}

// Model returns the configured offline model ID.
func (c *XiaoyuClient) Model() string { return c.model }

// BaseURL returns the offline service root.
func (c *XiaoyuClient) BaseURL() string { return c.baseURL }

// Dream runs one stateful reverie cycle. If the context has no deadline a long
// default (30m) is applied so a slow local generation is not cut short. Callers
// that need non-blocking behavior should invoke Dream from a goroutine — the
// dreaming kernel does exactly this.
func (c *XiaoyuClient) Dream(ctx context.Context, req DreamRequest) (*DreamResponse, error) {
	if c == nil || c.baseURL == "" {
		return nil, fmt.Errorf("offline: xiaoyu client not configured")
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("offline: marshal dream request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, joinURL(c.baseURL, "/v1/reverie/dream"), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("offline: dream request: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("offline: dream api %d: %.300s", resp.StatusCode, string(raw))
	}

	var out DreamResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("offline: decode dream response: %w", err)
	}
	// Preserve continuity if the server echoed no new handle.
	if out.SessionID == "" {
		out.SessionID = req.SessionID
	}
	return &out, nil
}

// Healthy reports whether the offline service answers /healthz. Used to gate the
// dreaming scheduler so a busy/offline NPU does not produce repeated errors.
func (c *XiaoyuClient) Healthy(ctx context.Context) bool {
	if c == nil || c.baseURL == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, joinURL(c.baseURL, "/healthz"), nil)
	if err != nil {
		return false
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	return resp.StatusCode == http.StatusOK
}

// joinURL appends path to base, avoiding a doubled /v1 when base already ends in
// it (so both "http://host" and "http://host/v1" roots work).
func joinURL(base, path string) string {
	base = strings.TrimRight(base, "/")
	if strings.HasSuffix(base, "/v1") && strings.HasPrefix(path, "/v1/") {
		return base + strings.TrimPrefix(path, "/v1")
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path
}
