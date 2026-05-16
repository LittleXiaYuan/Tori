package cognikernelpack

import (
	"encoding/json"
	"net/http"
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
	router *Router
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
		{Method: http.MethodGet, Path: RuntimePackStateRoute, Description: "Read live Cogni runtime-loop and pack-state gate status."},
	}
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
