package cognikernelpack

import (
	"encoding/json"
	"net/http"
	"time"

	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.cogni-kernel"

// CogniGateway is the narrow Gateway surface required by the Cogni Kernel
// pack. Keeping the pack behind this interface avoids importing Gateway from
// the pack package and makes the bridge easy to replace with a standalone API
// handler later.
type CogniGateway interface {
	HandleCogniKernelPack(w http.ResponseWriter, r *http.Request)
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
// module. The business logic still lives in Gateway during this bridge phase;
// route ownership, enablement and method gates now belong to Pack Runtime.
type Handler struct {
	gateway  CogniGateway
	reporter RuntimeStateReporter
}

func NewHandler(gateway CogniGateway) *Handler {
	h := &Handler{gateway: gateway}
	if reporter, ok := gateway.(RuntimeStateReporter); ok {
		h.reporter = reporter
	}
	return h
}

func NewHandlerWithRuntimeState(gateway CogniGateway, reporter RuntimeStateReporter) *Handler {
	return &Handler{gateway: gateway, reporter: reporter}
}

func RuntimeRouteSpecs() []packruntime.BackendRouteSpec {
	return []packruntime.BackendRouteSpec{
		{Method: http.MethodGet, Path: "/v1/cognis/runtime/pack-state", Description: "Read live Cogni runtime-loop and pack-state gate status."},
	}
}

func (h *Handler) PackID() string { return PackID }

func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Methods: []string{http.MethodGet, http.MethodPost}, Path: "/v1/cognis", Handler: h.gateway.HandleCogniKernelPack},
		{Methods: []string{http.MethodGet, http.MethodPost, http.MethodDelete}, Path: "/v1/cognis/", Handler: h.gateway.HandleCogniKernelPack},
		{Methods: []string{http.MethodGet}, Path: "/v1/cognis/runtime/pack-state", Handler: h.HandleRuntimePackState},
	}
}

func (h *Handler) RuntimeRoutes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Methods: []string{http.MethodGet}, Path: "/v1/cognis/runtime/pack-state", Handler: h.HandleRuntimePackState},
	}
}

func (h *Handler) HandleRuntimePackState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONStatus(w, http.StatusMethodNotAllowed, map[string]any{"error": "GET only"})
		return
	}
	if h.reporter == nil {
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]any{
			"error":   "cogni runtime state reporter not configured",
			"pack_id": PackID,
		})
		return
	}
	writeJSON(w, h.reporter.CogniKernelRuntimeState())
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
