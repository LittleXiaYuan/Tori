package packruntime

import (
	"context"
	"net/http"
	"time"

	"yunque-agent/pkg/skills"
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

// CapabilityIndexEntry is a read-only projection of one capability declared by
// an installed pack manifest. It gives frontends, SDKs, runtime skill gates and
// release checks a stable way to answer "which pack owns this capability?" from
// registry state instead of hard-coded feature lists.
type CapabilityIndexEntry struct {
	Capability    string   `json:"capability"`
	PackID        string   `json:"pack_id"`
	PackName      string   `json:"pack_name"`
	PackStatus    string   `json:"pack_status"`
	Enabled       bool     `json:"enabled"`
	Optional      bool     `json:"optional"`
	Routes        []string `json:"routes,omitempty"`
	Permissions   []string `json:"permissions,omitempty"`
	SDKTypeScript string   `json:"sdk_typescript,omitempty"`
	FrontendPaths []string `json:"frontend_paths,omitempty"`
}

// CapabilityIndexReport is returned by /v1/packs/capabilities. It is intentionally
// manifest-derived and side-effect free, so it can be used before wiring a real
// runtime skill gate or operator policy engine.
type CapabilityIndexReport struct {
	GeneratedAt         time.Time              `json:"generated_at"`
	Packs               int                    `json:"packs"`
	EnabledPacks        int                    `json:"enabled_packs"`
	Capabilities        int                    `json:"capabilities"`
	EnabledCapabilities int                    `json:"enabled_capabilities"`
	Entries             []CapabilityIndexEntry `json:"entries"`
}

// CapabilityResolveReport is returned by /v1/packs/capabilities/resolve.
// It keeps capability lookup side-effect free while giving external shells,
// operators, and runtime gates a concrete next action: use an enabled pack,
// enable an installed pack, or install a pack that provides the capability.
type CapabilityResolveReport struct {
	GeneratedAt    time.Time              `json:"generated_at"`
	Capability     string                 `json:"capability"`
	Found          bool                   `json:"found"`
	Enabled        bool                   `json:"enabled"`
	Action         string                 `json:"action"`
	Preferred      *CapabilityIndexEntry  `json:"preferred,omitempty"`
	Entries        []CapabilityIndexEntry `json:"entries"`
	EnabledEntries []CapabilityIndexEntry `json:"enabled_entries"`
}

// CapabilityGateReport is returned by /v1/packs/capabilities/gate.
// Unlike the resolver, it is shaped for runtime preflight checks: a caller can
// ask whether a capability is currently usable, and receive the concrete block
// reason without mutating installed pack state.
type CapabilityGateReport struct {
	GeneratedAt time.Time                `json:"generated_at"`
	Capability  string                   `json:"capability"`
	Allowed     bool                     `json:"allowed"`
	Action      string                   `json:"action"`
	Reason      string                   `json:"reason,omitempty"`
	Resolution  CapabilityResolveReport  `json:"resolution"`
	RouteAudit  []BackendRouteAuditEntry `json:"route_audit,omitempty"`
}

// CapabilityPlanReport is returned by /v1/packs/capabilities/plan. It rolls up
// several capability gates into a single enterprise workflow preflight so
// callers can prepare the minimal set of enabled or installable packs before
// executing a task.
type CapabilityPlanReport struct {
	GeneratedAt           time.Time                 `json:"generated_at"`
	Capabilities          []string                  `json:"capabilities"`
	Allowed               bool                      `json:"allowed"`
	Action                string                    `json:"action"`
	AllowedCount          int                       `json:"allowed_count"`
	BlockedCount          int                       `json:"blocked_count"`
	UseCount              int                       `json:"use_count"`
	EnableCount           int                       `json:"enable_count"`
	InstallCount          int                       `json:"install_count"`
	RouteAuditIssueCount  int                       `json:"route_audit_issue_count"`
	Gates                 []CapabilityGateReport    `json:"gates"`
	RequiredPacks         []CapabilityIndexEntry    `json:"required_packs,omitempty"`
	EnablePacks           []CapabilityIndexEntry    `json:"enable_packs,omitempty"`
	InstallCapabilities   []string                  `json:"install_capabilities,omitempty"`
	CatalogInstallHints   []PackCatalogEntry        `json:"catalog_install_hints,omitempty"`
	CatalogDownloadHints  []PackCatalogEntry        `json:"catalog_download_hints,omitempty"`
	CatalogSourceReports  []PackCatalogSourceReport `json:"catalog_source_reports,omitempty"`
	RouteAuditIssues      []BackendRouteAuditEntry  `json:"route_audit_issues,omitempty"`
	UnavailableReasons    []string                  `json:"unavailable_reasons,omitempty"`
	DownloadablePackHints []CapabilityIndexEntry    `json:"downloadable_pack_hints,omitempty"`
}

// CapabilityPrepareStep is one operator-facing action inside a read-only
// capability preparation plan. It is intentionally descriptive: callers still
// use explicit install/enable endpoints for every mutation.
type CapabilityPrepareStep struct {
	Action         string                `json:"action"`
	PackID         string                `json:"pack_id,omitempty"`
	PackName       string                `json:"pack_name,omitempty"`
	Capability     string                `json:"capability,omitempty"`
	ManifestPath   string                `json:"manifest_path,omitempty"`
	ManifestURL    string                `json:"manifest_url,omitempty"`
	PackageURL     string                `json:"package_url,omitempty"`
	FrontendURL    string                `json:"frontend_url,omitempty"`
	SHA256         string                `json:"sha256,omitempty"`
	SizeBytes      int64                 `json:"size_bytes,omitempty"`
	Installed      bool                  `json:"installed"`
	Enabled        bool                  `json:"enabled"`
	Downloadable   bool                  `json:"downloadable"`
	Reason         string                `json:"reason,omitempty"`
	CatalogEntry   *PackCatalogEntry     `json:"catalog_entry,omitempty"`
	CapabilityInfo *CapabilityIndexEntry `json:"capability_info,omitempty"`
}

// CapabilityPrepareReport is returned by /v1/packs/capabilities/prepare. It
// turns workflow preflight into a minimal package preparation checklist for
// enterprise consoles and SDK callers: use, enable, install/download, or fix
// route drift.
type CapabilityPrepareReport struct {
	GeneratedAt          time.Time                 `json:"generated_at"`
	Capabilities         []string                  `json:"capabilities"`
	Allowed              bool                      `json:"allowed"`
	Action               string                    `json:"action"`
	Plan                 CapabilityPlanReport      `json:"plan"`
	UseSteps             []CapabilityPrepareStep   `json:"use_steps,omitempty"`
	EnableSteps          []CapabilityPrepareStep   `json:"enable_steps,omitempty"`
	InstallSteps         []CapabilityPrepareStep   `json:"install_steps,omitempty"`
	DownloadSteps        []CapabilityPrepareStep   `json:"download_steps,omitempty"`
	RouteAuditFixSteps   []CapabilityPrepareStep   `json:"route_audit_fix_steps,omitempty"`
	CatalogSourceReports []PackCatalogSourceReport `json:"catalog_source_reports,omitempty"`
	Steps                []CapabilityPrepareStep   `json:"steps"`
	StepCount            int                       `json:"step_count"`
	DownloadCount        int                       `json:"download_count"`
	EnableCount          int                       `json:"enable_count"`
	InstallCount         int                       `json:"install_count"`
	RouteAuditIssueCount int                       `json:"route_audit_issue_count"`
	ReadyCount           int                       `json:"ready_count"`
	UnavailableReasons   []string                  `json:"unavailable_reasons,omitempty"`
	RouteAuditIssues     []BackendRouteAuditEntry  `json:"route_audit_issues,omitempty"`
}

// PackCatalogEntry describes one installable pack manifest discovered from a
// local or remote catalog source. It is read-only metadata: installing or
// downloading still goes through the explicit Pack Runtime mutation endpoints.
type PackCatalogEntry struct {
	ManifestPath string     `json:"manifest_path,omitempty"`
	ManifestURL  string     `json:"manifest_url,omitempty"`
	PackageURL   string     `json:"package_url,omitempty"`
	Source       string     `json:"source,omitempty"`
	Manifest     Manifest   `json:"manifest"`
	Installed    bool       `json:"installed"`
	Enabled      bool       `json:"enabled"`
	Status       PackStatus `json:"status,omitempty"`
	UpdateAction string     `json:"update_action"`
	Downloadable bool       `json:"downloadable"`
}

// PackCatalogSourceReport describes how one configured catalog source was
// scanned. It keeps the read-only catalog observable for operator consoles and
// SDK callers without requiring them to parse free-form error strings.
type PackCatalogSourceReport struct {
	Source         string   `json:"source"`
	OK             bool     `json:"ok"`
	ManifestCount  int      `json:"manifest_count"`
	MatchedEntries int      `json:"matched_entries"`
	Errors         []string `json:"errors,omitempty"`
}

// PackCatalogReport is returned by /v1/packs/catalog. It lets enterprise
// consoles and SDK callers map missing capabilities to concrete incremental
// packages without mutating the registry.
type PackCatalogReport struct {
	GeneratedAt   time.Time                 `json:"generated_at"`
	Sources       []string                  `json:"sources"`
	SourceReports []PackCatalogSourceReport `json:"source_reports,omitempty"`
	Count         int                       `json:"count"`
	Installed     int                       `json:"installed"`
	Enabled       int                       `json:"enabled"`
	Downloadable  int                       `json:"downloadable"`
	Capabilities  int                       `json:"capabilities"`
	Capability    string                    `json:"capability,omitempty"`
	Query         string                    `json:"query,omitempty"`
	Entries       []PackCatalogEntry        `json:"entries"`
	InstallHints  []PackCatalogEntry        `json:"install_hints,omitempty"`
	EnableHints   []PackCatalogEntry        `json:"enable_hints,omitempty"`
	DownloadHints []PackCatalogEntry        `json:"download_hints,omitempty"`
	Errors        []string                  `json:"errors,omitempty"`
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
//
// This is the v1 interface (routes only). New packs should implement the v2
// Module interface below, which adds a lifecycle and depends on the kernel Host
// contract instead of the concrete Gateway. BackendModule stays for backward
// compatibility — AsModule adapts it to Module.
type BackendModule interface {
	PackID() string
	Routes() []BackendRoute
}

// Module is the v2 capability-pack contract (see doc/MICROKERNEL-PACK-BLUEPRINT.md).
// Unlike BackendModule it has a real lifecycle and receives the kernel Host, so a
// pack can own a full vertical capability (routes + background workers) while
// depending only on the kernel contract — never on *gateway.Gateway.
type Module interface {
	PackID() string
	// Init wires the pack against the kernel. Called once before Routes/Start.
	Init(host Host) error
	// Routes returns the HTTP surface to mount (may be empty).
	Routes() []BackendRoute
	// Start launches background workers (index build, schedulers, …). It must
	// return promptly; long-running work belongs in a goroutine bound to ctx.
	Start(ctx context.Context) error
	// Stop tears down background workers on disable/uninstall.
	Stop(ctx context.Context) error
}

// ContextProvider is an optional capability: a Module that contributes dynamic
// context into the prompt-assembly path.
type ContextProvider interface {
	BuildContext(ctx context.Context, message, tenant string) string
}

// DependencyAware is an optional capability: a Module that declares the pack IDs
// it must load after, so the runtime can topologically order Init/Start.
type DependencyAware interface {
	DependsOn() []string
}

// SkillProvider is an optional capability: a Module that contributes agent tools
// (skills) the planner can invoke. When the pack is enabled the host registers
// these into the skill registry, so a Pack's enablement adds callable capability
// to the agent (Tier 0 microkernel "tool line"); disabling removes them.
type SkillProvider interface {
	Skills() []skills.Skill
}

// AsModule adapts any BackendModule to the v2 Module interface. A value that
// already implements Module is returned as-is; a v1 BackendModule is wrapped
// with no-op Init/Start/Stop so existing packs keep working unchanged.
func AsModule(m BackendModule) Module {
	if mod, ok := m.(Module); ok {
		return mod
	}
	return legacyModule{inner: m}
}

// legacyModule wraps a v1 BackendModule as a v2 Module.
type legacyModule struct{ inner BackendModule }

func (l legacyModule) PackID() string             { return l.inner.PackID() }
func (l legacyModule) Init(Host) error            { return nil }
func (l legacyModule) Routes() []BackendRoute     { return l.inner.Routes() }
func (l legacyModule) Start(context.Context) error { return nil }
func (l legacyModule) Stop(context.Context) error  { return nil }
