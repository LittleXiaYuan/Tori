// Package toripack mounts Tori account binding and usage endpoints as a native
// capability pack.
package toripack

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"yunque-agent/internal/security/ssrf"
	"yunque-agent/internal/tori"
	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.tori"

type Gateway interface {
	ToriTokenStore() *tori.TokenStore
}

type tokenStore interface {
	IsBound() bool
	Get() *tori.StoredToken
	Store(*tori.TokenResponse, *tori.UserInfo, string) error
	Clear() error
}

type bindStarter func(context.Context, tori.OAuthConfig) (string, <-chan tori.BindResult, error)
type healthChecker func(string) (*tori.HealthStatus, error)
type usageFetcher func(string, string) (*tori.UsageSummary, error)

type Handler struct {
	storeOf     func() tokenStore
	startBind   bindStarter
	checkHealth healthChecker
	fetchUsage  usageFetcher
	openBrowser func(string)
	host        packruntime.Host
	started     atomic.Bool
}

func New(gateway Gateway) *Handler {
	if gateway == nil {
		return NewProvider(nil)
	}
	return NewProvider(func() tokenStore { return gateway.ToriTokenStore() })
}

func NewProvider(storeOf func() tokenStore) *Handler {
	return &Handler{
		storeOf: storeOf,
		startBind: func(ctx context.Context, cfg tori.OAuthConfig) (string, <-chan tori.BindResult, error) {
			return tori.StartBindFlow(ctx, cfg)
		},
		checkHealth: tori.CheckHealth,
		fetchUsage:  tori.FetchUsage,
		openBrowser: openBrowser,
	}
}

func (h *Handler) WithBindStarter(fn bindStarter) *Handler {
	if fn != nil {
		h.startBind = fn
	}
	return h
}

func (h *Handler) WithHealthChecker(fn healthChecker) *Handler {
	if fn != nil {
		h.checkHealth = fn
	}
	return h
}

func (h *Handler) WithUsageFetcher(fn usageFetcher) *Handler {
	if fn != nil {
		h.fetchUsage = fn
	}
	return h
}

func (h *Handler) WithBrowserOpener(fn func(string)) *Handler {
	if fn != nil {
		h.openBrowser = fn
	}
	return h
}

var _ packruntime.Module = (*Handler)(nil)

func (h *Handler) PackID() string { return PackID }

func (h *Handler) Init(host packruntime.Host) error {
	h.host = host
	return nil
}

func (h *Handler) Start(ctx context.Context) error {
	h.started.Store(true)
	if h.host != nil {
		h.host.Logger().Info("tori pack started", "pack", PackID)
	}
	return nil
}

func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Method: http.MethodPost, Path: "/v1/tori/bind", Handler: h.Bind},
		{Method: http.MethodGet, Path: "/v1/tori/status", Handler: h.Status},
		{Method: http.MethodPost, Path: "/v1/tori/unbind", Handler: h.Unbind},
		{Method: http.MethodGet, Path: "/v1/tori/health", Handler: h.Health},
		{Method: http.MethodGet, Path: "/v1/tori/usage", Handler: h.Usage},
	}
}

func RouteSpecs() []packruntime.BackendRouteSpec {
	return []packruntime.BackendRouteSpec{
		{Method: http.MethodPost, Path: "/v1/tori/bind", Description: "Start the OAuth2 PKCE flow to bind a Tori account."},
		{Method: http.MethodGet, Path: "/v1/tori/status", Description: "Read the current Tori binding status."},
		{Method: http.MethodPost, Path: "/v1/tori/unbind", Description: "Remove the current Tori binding."},
		{Method: http.MethodGet, Path: "/v1/tori/health", Description: "Check the bound Tori instance health."},
		{Method: http.MethodGet, Path: "/v1/tori/usage", Description: "Fetch usage summary from the bound Tori instance."},
	}
}

func Paths() []string {
	return []string{"/v1/tori/bind", "/v1/tori/status", "/v1/tori/unbind", "/v1/tori/health", "/v1/tori/usage"}
}

func (h *Handler) store() tokenStore {
	if h.storeOf == nil {
		return nil
	}
	return h.storeOf()
}

func (h *Handler) Bind(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	store := h.store()
	if store == nil {
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]string{"error": "tori module not initialized"})
		return
	}
	if store.IsBound() {
		writeJSONStatus(w, http.StatusConflict, map[string]string{"error": "already bound, unbind first"})
		return
	}

	var body struct {
		ToriURL string `json:"tori_url"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	sanitizedURL, err := ValidateToriURL(body.ToriURL)
	if err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	body.ToriURL = sanitizedURL

	cfg := tori.DefaultOAuthConfig()
	if body.ToriURL != "" {
		cfg.ToriBaseURL = body.ToriURL
	}
	authorizeURL, resultCh, err := h.startBind(context.Background(), cfg)
	if err != nil {
		slog.Error("tori: start bind flow failed", "err", err)
		writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if h.openBrowser != nil {
		h.openBrowser(authorizeURL)
	}
	go h.consumeBindResults(store, body.ToriURL, cfg.ToriBaseURL, resultCh)

	writeJSONStatus(w, http.StatusOK, map[string]any{
		"status":        "pending",
		"authorize_url": authorizeURL,
		"message":       "Please complete authorization in your browser",
	})
}

func (h *Handler) consumeBindResults(store tokenStore, requestedURL, defaultURL string, resultCh <-chan tori.BindResult) {
	for result := range resultCh {
		if result.Err != nil {
			slog.Error("tori: bind failed", "err", result.Err)
			return
		}
		if result.Token == nil {
			continue
		}
		storeURL := requestedURL
		if storeURL == "" {
			storeURL = defaultURL
		}
		if err := store.Store(result.Token, result.UserInfo, storeURL); err != nil {
			slog.Error("tori: store token failed", "err", err)
			return
		}
		apiKey := ""
		username := ""
		if result.UserInfo != nil {
			apiKey = result.UserInfo.APIKey
			username = result.UserInfo.Username
		}
		tori.ApplyLLMConfig(storeURL, apiKey)
		slog.Info("tori: bind successful", "user", username, "tori_url", storeURL)
	}
}

func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	store := h.store()
	if store == nil {
		writeJSONStatus(w, http.StatusOK, tori.BindingStatus{Bound: false})
		return
	}
	token := store.Get()
	if token == nil {
		writeJSONStatus(w, http.StatusOK, tori.BindingStatus{Bound: false})
		return
	}
	writeJSONStatus(w, http.StatusOK, tori.BindingStatus{
		Bound:            true,
		Username:         token.Username,
		ToriURL:          token.ToriBaseURL,
		ExpiresAt:        token.ExpiresAt.Format(time.RFC3339),
		SandboxAvailable: os.Getenv("SANDBOX_CLOUD_API_KEY") != "" || token.APIKey != "",
	})
}

func (h *Handler) Unbind(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	store := h.store()
	if store == nil || !store.IsBound() {
		writeJSONStatus(w, http.StatusOK, map[string]string{"status": "not_bound"})
		return
	}
	if err := store.Clear(); err != nil {
		slog.Error("tori: clear token failed", "err", err)
		writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	tori.RestoreLLMConfig()
	slog.Info("tori: unbound")
	writeJSONStatus(w, http.StatusOK, map[string]string{"status": "unbound"})
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	store := h.store()
	if store == nil || !store.IsBound() {
		writeJSONStatus(w, http.StatusOK, map[string]any{"status": "not_bound"})
		return
	}
	token := store.Get()
	if token == nil {
		writeJSONStatus(w, http.StatusOK, map[string]any{"status": "not_bound"})
		return
	}
	health, err := h.checkHealth(token.ToriBaseURL)
	if err != nil {
		writeJSONStatus(w, http.StatusOK, map[string]any{"status": "unreachable", "error": err.Error()})
		return
	}
	writeJSONStatus(w, http.StatusOK, health)
}

func (h *Handler) Usage(w http.ResponseWriter, r *http.Request) {
	store := h.store()
	if store == nil || !store.IsBound() {
		writeJSONStatus(w, http.StatusOK, map[string]any{"error": "not bound"})
		return
	}
	token := store.Get()
	if token == nil {
		writeJSONStatus(w, http.StatusOK, map[string]any{"error": "not bound"})
		return
	}
	usage, err := h.fetchUsage(token.ToriBaseURL, token.APIKey)
	if err != nil {
		writeJSONStatus(w, http.StatusOK, map[string]any{"error": err.Error()})
		return
	}
	writeJSONStatus(w, http.StatusOK, usage)
}

// ValidateToriURL rejects operator-supplied Tori URLs that point at local or
// metadata addresses, which would turn OAuth binding into an SSRF oracle.
func ValidateToriURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid tori_url: %w", err)
	}
	if err := ssrf.ValidateTarget(u); err != nil {
		return "", fmt.Errorf("tori_url rejected: %w", err)
	}
	allow := strings.TrimSpace(os.Getenv("TORI_URL_ALLOWLIST"))
	if allow != "" {
		host := strings.ToLower(u.Hostname())
		ok := false
		for _, entry := range strings.FieldsFunc(allow, func(r rune) bool { return r == ',' || r == ';' }) {
			entry = strings.ToLower(strings.TrimSpace(entry))
			if entry == "" {
				continue
			}
			if host == entry || strings.HasSuffix(host, "."+entry) {
				ok = true
				break
			}
		}
		if !ok {
			return "", fmt.Errorf("tori_url host %q is not in TORI_URL_ALLOWLIST", host)
		}
	}
	return raw, nil
}

func writeJSONStatus(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
