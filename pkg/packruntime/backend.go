package packruntime

import "net/http"

// BackendRoute describes one HTTP surface provided by a backend capability
// pack. The route is mounted by the host Gateway, while enablement is still
// controlled by the installed pack manifest.
type BackendRoute struct {
	Method  string
	Methods []string
	Path    string
	Handler http.HandlerFunc
	Auth    BackendRouteAuthMode
}

// BackendRouteAuthMode describes how the host Gateway should authenticate a
// backend pack route. The default mode uses the host's normal tenant/JWT/API key
// auth middleware. Passthrough is reserved for bridge routes that must perform
// their own protocol-specific authentication inside the handler while still
// keeping Pack Runtime's method and enabled-state gates.
type BackendRouteAuthMode string

const (
	BackendRouteAuthDefault     BackendRouteAuthMode = ""
	BackendRouteAuthPassthrough BackendRouteAuthMode = "passthrough"
)

// BackendRouteInfo is the serializable route metadata exposed by the host for
// pack runtime introspection. It intentionally omits the handler so external
// consoles and SDKs can audit the mounted backend surface without gaining
// executable references.
type BackendRouteInfo struct {
	Method  string   `json:"method,omitempty"`
	Methods []string `json:"methods,omitempty"`
	Path    string   `json:"path"`
	Auth    string   `json:"auth,omitempty"`
}

// BackendModuleInfo describes a backend capability pack that has been mounted
// into the host Gateway.
type BackendModuleInfo struct {
	PackID string             `json:"pack_id"`
	Routes []BackendRouteInfo `json:"routes"`
}

// BackendModule is implemented by backend capability packs that can be mounted
// into the host Gateway without hard-coding their individual routes there.
type BackendModule interface {
	PackID() string
	Routes() []BackendRoute
}
