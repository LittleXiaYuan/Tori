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
}

// BackendRouteInfo is the serializable route metadata exposed by the host for
// pack runtime introspection. It intentionally omits the handler so external
// consoles and SDKs can audit the mounted backend surface without gaining
// executable references.
type BackendRouteInfo struct {
	Method  string   `json:"method,omitempty"`
	Methods []string `json:"methods,omitempty"`
	Path    string   `json:"path"`
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
