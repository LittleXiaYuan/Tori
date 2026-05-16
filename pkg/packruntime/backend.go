package packruntime

import (
	"net/http"
	"time"
)

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

// BackendRouteAuditEntry compares one manifest-declared backend route or one
// mounted Gateway backend route against the other side of the Pack Runtime
// contract. It is intentionally data-only so SDKs and consoles can audit the
// route surface without reaching into host handlers.
type BackendRouteAuditEntry struct {
	PackID      string   `json:"pack_id"`
	PackName    string   `json:"pack_name,omitempty"`
	PackStatus  string   `json:"pack_status,omitempty"`
	Enabled     bool     `json:"enabled"`
	Status      string   `json:"status"`
	Declared    bool     `json:"declared"`
	Mounted     bool     `json:"mounted"`
	Method      string   `json:"method,omitempty"`
	Methods     []string `json:"methods,omitempty"`
	Path        string   `json:"path"`
	Auth        string   `json:"auth,omitempty"`
	Description string   `json:"description,omitempty"`
	Issues      []string `json:"issues,omitempty"`
}

// BackendRouteAuditReport is returned by /v1/packs/backend-route-audit. It
// makes manifest routeSpecs, installed-pack enablement, and mounted Gateway
// backend modules comparable from frontend shells, SDKs, and release gates.
type BackendRouteAuditReport struct {
	GeneratedAt      time.Time                `json:"generated_at"`
	Packs            int                      `json:"packs"`
	EnabledPacks     int                      `json:"enabled_packs"`
	MountedModules   int                      `json:"mounted_modules"`
	DeclaredRoutes   int                      `json:"declared_routes"`
	MountedRoutes    int                      `json:"mounted_routes"`
	OKRoutes         int                      `json:"ok_routes"`
	MissingRoutes    int                      `json:"missing_routes"`
	MethodMismatches int                      `json:"method_mismatches"`
	UndeclaredRoutes int                      `json:"undeclared_routes"`
	Entries          []BackendRouteAuditEntry `json:"entries"`
}

// BackendModule is implemented by backend capability packs that can be mounted
// into the host Gateway without hard-coding their individual routes there.
type BackendModule interface {
	PackID() string
	Routes() []BackendRoute
}
