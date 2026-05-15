package browserintent

import (
	"net/http"

	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.browser-intent"

// BrowserGateway is the narrow Gateway surface required by the Browser Intent
// pack. The pack owns route registration and enablement gates while Gateway
// continues to host the existing browser connector implementation during this
// bridge phase.
type BrowserGateway interface {
	HandleBrowserIntentPack(w http.ResponseWriter, r *http.Request)
	HandleBrowserIntentSession(w http.ResponseWriter, r *http.Request)
}

// Handler exposes browser connection, capture, OPP preview and extension
// scenario surfaces as a Pack Runtime backend module. The bridge keeps the
// migration reversible: disabling the pack removes the HTTP surface without
// touching the browser WebSocket hub or skill implementation.
type Handler struct {
	gateway BrowserGateway
}

func NewHandler(gateway BrowserGateway) *Handler {
	return &Handler{gateway: gateway}
}

func (h *Handler) PackID() string { return PackID }

func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Method: http.MethodGet, Path: "/v1/browser/status", Handler: h.gateway.HandleBrowserIntentPack},
		{Method: http.MethodGet, Path: "/v1/browser/config", Handler: h.gateway.HandleBrowserIntentPack},
		{Method: http.MethodPost, Path: "/v1/browser/navigate", Handler: h.gateway.HandleBrowserIntentPack},
		{Method: http.MethodGet, Path: "/v1/browser/screenshot", Handler: h.gateway.HandleBrowserIntentPack},
		{Method: http.MethodPost, Path: "/v1/browser/ocr", Handler: h.gateway.HandleBrowserIntentPack},
		{Method: http.MethodGet, Path: "/v1/browser/screenshot/latest", Handler: h.gateway.HandleBrowserIntentPack},
		{Method: http.MethodGet, Path: "/v1/browser/opp/pending", Handler: h.gateway.HandleBrowserIntentPack},
		{Method: http.MethodPost, Path: "/v1/browser/opp/decide", Handler: h.gateway.HandleBrowserIntentPack},
		{Method: http.MethodGet, Path: "/api/browser/ext/status", Handler: h.gateway.HandleBrowserIntentPack},
		{Method: http.MethodPost, Path: "/api/browser/ext/session", Auth: packruntime.BackendRouteAuthPassthrough, Handler: h.gateway.HandleBrowserIntentSession},
		{Method: http.MethodPost, Path: "/api/browser/ext/action", Handler: h.gateway.HandleBrowserIntentPack},
		{Method: http.MethodGet, Path: "/api/browser/ext/scenarios", Handler: h.gateway.HandleBrowserIntentPack},
		{Method: http.MethodPost, Path: "/api/browser/ext/scenarios/run", Handler: h.gateway.HandleBrowserIntentPack},
	}
}
