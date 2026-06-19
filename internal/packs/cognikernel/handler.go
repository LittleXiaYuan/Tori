package cognikernelpack

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"

	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.cogni-kernel"

const (
	CollectionRoute       = "/v1/cognis"
	SubResourceRoute      = "/v1/cognis/"
	RuntimePackStateRoute = "/v1/cognis/runtime/pack-state"
)

// API is the pack-owned Cogni Kernel HTTP surface. The current Gateway still
// implements it as an adapter during the migration, but the pack no longer
// depends on a Gateway-named bridge method. A standalone Cogni API service can
// replace the adapter without changing Pack Runtime route ownership.
type API interface {
	ServeCogniKernel(w http.ResponseWriter, r *http.Request)
}

type RuntimeStateReporter interface {
	CogniKernelRuntimeState() RuntimeStateReport
}

type RuntimeStateReport struct {
	PackID                    string    `json:"pack_id"`
	Stage                     string    `json:"stage"`
	PackInstalled             bool      `json:"pack_installed"`
	PackEnabled               bool      `json:"pack_enabled"`
	PackStatus                string    `json:"pack_status"`
	RuntimeLoopPackStateReady bool      `json:"runtime_loop_pack_state_ready"`
	RuntimeLoopRunning        bool      `json:"runtime_loop_running"`
	StopsRuntimeLoops         bool      `json:"stops_runtime_loops"`
	StartsRuntimeLoops        bool      `json:"starts_runtime_loops"`
	ClearsRuntimeState        bool      `json:"clears_runtime_state"`
	SentinelReady             bool      `json:"sentinel_ready"`
	SchedulerReady            bool      `json:"scheduler_ready"`
	BusReady                  bool      `json:"bus_ready"`
	ExperienceStoreReady      bool      `json:"experience_store_ready"`
	ActiveBusCognis           int       `json:"active_bus_cognis"`
	ExperienceStoreCount      int       `json:"experience_store_count"`
	GeneratedAt               time.Time `json:"generated_at"`
	Capabilities              []string  `json:"capabilities"`
	Artifacts                 []string  `json:"artifacts"`
	Notes                     []string  `json:"notes,omitempty"`
}

// Handler exposes CogniKernel/Cognis management as a Pack Runtime backend
// module. Business operations may still be served by a Gateway adapter during
// this migration phase, but route dispatch, runtime-state handling, enablement
// and method gates are owned by this package.
type Handler struct {
	router  *Router
	host    packruntime.Host
	started atomic.Bool
}

func NewHandler(api API) *Handler {
	return NewHandlerWithRuntimeState(api, nil)
}

func NewHandlerWithRuntimeState(api API, reporter RuntimeStateReporter) *Handler {
	if reporter == nil {
		if inferred, ok := api.(RuntimeStateReporter); ok {
			reporter = inferred
		}
	}
	return &Handler{router: NewRouter(api, reporter)}
}

// compile-time assertion: Cogni Kernel is a v2 capability Module (Tier 0 microkernel).
var _ packruntime.Module = (*Handler)(nil)

// Init wires the pack against the kernel Host (deps arrive via the API +
// RuntimeStateReporter interfaces, not the concrete Gateway).
func (h *Handler) Init(host packruntime.Host) error {
	h.host = host
	return nil
}

// Start marks the pack live on enable.
func (h *Handler) Start(ctx context.Context) error {
	h.started.Store(true)
	if h.host != nil {
		h.host.Logger().Info("cogni-kernel pack started", "pack", PackID)
	}
	return nil
}

// Stop marks the pack stopped on disable.
func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

// Router is the pack-owned route dispatcher for Cogni Kernel. It keeps runtime
// pack-state as a first-class pack route and delegates declaration operations to
// the supplied API adapter.
type Router struct {
	api      API
	reporter RuntimeStateReporter
}

func NewRouter(api API, reporter RuntimeStateReporter) *Router {
	return &Router{api: api, reporter: reporter}
}

func (r *Router) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Methods: []string{http.MethodGet, http.MethodPost}, Path: CollectionRoute, Handler: r.ServeCogniKernel},
		{Methods: []string{http.MethodGet, http.MethodPost, http.MethodDelete}, Path: SubResourceRoute, Handler: r.ServeCogniKernel},
		{Methods: []string{http.MethodGet}, Path: RuntimePackStateRoute, Handler: r.HandleRuntimePackState},
	}
}

func (r *Router) RuntimeRoutes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Methods: []string{http.MethodGet}, Path: RuntimePackStateRoute, Handler: r.HandleRuntimePackState},
	}
}

func (r *Router) ServeCogniKernel(w http.ResponseWriter, req *http.Request) {
	if r == nil || r.api == nil {
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]any{
			"error":   "cogni api handler not configured",
			"pack_id": PackID,
		})
		return
	}
	r.api.ServeCogniKernel(w, req)
}

func (r *Router) HandleRuntimePackState(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeJSONStatus(w, http.StatusMethodNotAllowed, map[string]any{"error": "GET only"})
		return
	}
	if r == nil || r.reporter == nil {
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]any{
			"error":   "cogni runtime state reporter not configured",
			"pack_id": PackID,
		})
		return
	}
	writeJSON(w, r.reporter.CogniKernelRuntimeState())
}

func RuntimeRouteSpecs() []packruntime.BackendRouteSpec {
	return []packruntime.BackendRouteSpec{
		{Method: http.MethodGet, Path: "/v1/cognis/runtime/pack-state", Description: "Read live Cogni runtime-loop and pack-state gate status."},
	}
}

func RouteSpecs() []packruntime.BackendRouteSpec {
	return []packruntime.BackendRouteSpec{
		{Method: http.MethodGet, Path: "/v1/cognis", Description: "List Cogni declarations with health summaries."},
		{Method: http.MethodPost, Path: "/v1/cognis", Description: "Create an inline Cogni declaration."},
		{Method: http.MethodGet, Path: "/v1/cognis/", Description: "Read one Cogni declaration or its sub-resources."},
		{Method: http.MethodPost, Path: "/v1/cognis/", Description: "Run Cogni mutations such as reload, enable, disable, verify, generate, import, evolve, experience record, workflow run, federation update, or routing."},
		{Method: http.MethodDelete, Path: "/v1/cognis/", Description: "Remove one Cogni declaration."},
		{Method: http.MethodPost, Path: "/v1/cognis/reload", Description: "Reload Cogni declarations from disk."},
		{Method: http.MethodGet, Path: "/v1/cognis/traces", Description: "List recent per-turn Cogni evaluation traces."},
		{Method: http.MethodGet, Path: "/v1/cognis/stats", Description: "Read Cogni trace activation statistics."},
		{Method: http.MethodGet, Path: "/v1/cognis/health", Description: "Read health metrics for all recently observed Cogni declarations."},
		{Method: http.MethodGet, Path: "/v1/cognis/alerts", Description: "List Cogni sentinel alerts."},
		{Method: http.MethodPost, Path: "/v1/cognis/alerts/scan", Description: "Run a Cogni sentinel alert scan."},
		{Method: http.MethodGet, Path: "/v1/cognis/verify", Description: "Verify all Cogni declarations."},
		{Method: http.MethodPost, Path: "/v1/cognis/verify", Description: "Verify all Cogni declarations."},
		{Method: http.MethodPost, Path: "/v1/cognis/generate", Description: "Generate a Cogni declaration from a natural-language description."},
		{Method: http.MethodGet, Path: "/v1/cognis/export", Description: "Export Cogni declarations as a bundle."},
		{Method: http.MethodPost, Path: "/v1/cognis/export", Description: "Export Cogni declarations as a bundle."},
		{Method: http.MethodPost, Path: "/v1/cognis/import", Description: "Import a Cogni bundle and persist accepted declarations."},
		{Method: http.MethodGet, Path: "/v1/cognis/evolution", Description: "List Cogni evolution experiments."},
		{Method: http.MethodGet, Path: "/v1/cognis/federation", Description: "Read Cogni federation status."},
		{Method: http.MethodGet, Path: "/v1/cognis/federation/peers", Description: "List Cogni federation peers."},
		{Method: http.MethodPost, Path: "/v1/cognis/federation/peers", Description: "Add a Cogni federation peer."},
		{Method: http.MethodPost, Path: "/v1/cognis/federation/discover", Description: "Discover remote Cogni federation skills."},
		{Method: http.MethodGet, Path: "/v1/cognis/economics", Description: "Read Cogni economics and cost summary."},
		{Method: http.MethodPost, Path: "/v1/cognis/route", Description: "Route a message through Cogni candidates."},
		{Method: http.MethodGet, Path: "/v1/cognis/runtime/pack-state", Description: "Read live Cogni runtime-loop and pack-state gate status."},
	}
}

func Paths() []string {
	seen := map[string]bool{}
	paths := []string{}
	for _, spec := range RouteSpecs() {
		if seen[spec.Path] {
			continue
		}
		seen[spec.Path] = true
		paths = append(paths, spec.Path)
	}
	return paths
}

func (h *Handler) PackID() string { return PackID }

func (h *Handler) Routes() []packruntime.BackendRoute {
	return h.router.Routes()
}

func (h *Handler) RuntimeRoutes() []packruntime.BackendRoute {
	return h.router.RuntimeRoutes()
}

func (h *Handler) HandleRuntimePackState(w http.ResponseWriter, r *http.Request) {
	h.router.HandleRuntimePackState(w, r)
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func writeJSONStatus(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
