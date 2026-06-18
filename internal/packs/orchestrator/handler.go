package orchestratorpack

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"sync/atomic"

	"yunque-agent/internal/orchestrator"
	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.orchestrator"

// Gateway is the narrow host surface needed by the IDE/work orchestration pack.
type Gateway interface {
	OrchDaemon() *orchestrator.Daemon
	OrchLauncher() *orchestrator.Launcher
	BaseContext() context.Context
}

type Handler struct {
	daemonOf   func() *orchestrator.Daemon
	launcherOf func() *orchestrator.Launcher
	baseCtxOf  func() context.Context
	host       packruntime.Host
	started    atomic.Bool
}

func New(gateway Gateway) *Handler {
	if gateway == nil {
		return NewProvider(nil, nil, nil)
	}
	return NewProvider(gateway.OrchDaemon, gateway.OrchLauncher, gateway.BaseContext)
}

func NewProvider(daemon func() *orchestrator.Daemon, launcher func() *orchestrator.Launcher, baseCtx func() context.Context) *Handler {
	return &Handler{
		daemonOf:   daemon,
		launcherOf: launcher,
		baseCtxOf:  baseCtx,
	}
}

func (h *Handler) PackID() string { return PackID }

var _ packruntime.Module = (*Handler)(nil)

func (h *Handler) Init(host packruntime.Host) error {
	h.host = host
	return nil
}

func (h *Handler) Start(ctx context.Context) error {
	h.started.Store(true)
	if h.host != nil {
		h.host.Logger().Info("orchestrator pack started", "pack", PackID)
	}
	return nil
}

func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

func (h *Handler) Routes() []packruntime.BackendRoute {
	specs := h.routeSpecs()
	routes := make([]packruntime.BackendRoute, 0, len(specs))
	for _, spec := range specs {
		routes = append(routes, packruntime.BackendRoute{
			Method:  spec.Method,
			Methods: append([]string(nil), spec.Methods...),
			Path:    spec.Path,
			Handler: spec.Handler,
		})
	}
	return routes
}

type routeSpec struct {
	Method      string
	Methods     []string
	Path        string
	Handler     http.HandlerFunc
	Description string
}

func (h *Handler) routeSpecs() []routeSpec {
	return []routeSpec{
		{Method: http.MethodGet, Path: "/v1/orchestrator/status", Handler: h.status, Description: "Read IDE/work orchestrator runtime status."},
		{Method: http.MethodPost, Path: "/v1/orchestrator/toggle", Handler: h.toggle, Description: "Start or stop the IDE/work orchestrator daemon."},
		{Method: http.MethodGet, Path: "/v1/orchestrator/sessions", Handler: h.sessions, Description: "List active orchestrator worker sessions."},
		{Method: http.MethodGet, Path: "/v1/orchestrator/detect", Handler: h.detectIDEs, Description: "Detect locally available IDE worker adapters."},
		{Method: http.MethodPost, Path: "/v1/orchestrator/adapters/add", Handler: h.addCustomAdapter, Description: "Register a custom IDE worker adapter."},
		{Method: http.MethodGet, Path: "/v1/orchestrator/events", Handler: h.events, Description: "List recent orchestrator events."},
		{Method: http.MethodGet, Path: "/v1/orchestrator/events/task", Handler: h.taskTimeline, Description: "List orchestrator events for one task."},
		{Methods: []string{http.MethodGet, http.MethodPut}, Path: "/v1/orchestrator/policy", Handler: h.policy, Description: "Read or update the orchestrator automation policy."},
	}
}

func RouteSpecs() []packruntime.BackendRouteSpec {
	specs := (&Handler{}).routeSpecs()
	routes := make([]packruntime.BackendRouteSpec, 0, len(specs)+1)
	for _, spec := range specs {
		if len(spec.Methods) > 0 {
			for _, method := range spec.Methods {
				routes = append(routes, packruntime.BackendRouteSpec{Method: method, Path: spec.Path, Description: spec.Description})
			}
			continue
		}
		routes = append(routes, packruntime.BackendRouteSpec{Method: spec.Method, Path: spec.Path, Description: spec.Description})
	}
	return routes
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

func (h *Handler) daemon() *orchestrator.Daemon {
	if h.daemonOf == nil {
		return nil
	}
	return h.daemonOf()
}

func (h *Handler) launcher() *orchestrator.Launcher {
	if h.launcherOf == nil {
		return nil
	}
	return h.launcherOf()
}

func (h *Handler) baseContext() context.Context {
	if h.baseCtxOf == nil {
		return context.Background()
	}
	if ctx := h.baseCtxOf(); ctx != nil {
		return ctx
	}
	return context.Background()
}

func (h *Handler) status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	daemon := h.daemon()
	launcher := h.launcher()
	running := false
	if daemon != nil {
		running = daemon.IsRunning()
	}
	adapters := []string{}
	if launcher != nil {
		adapters = launcher.AvailableAdapters()
	}
	sessionCount := 0
	if launcher != nil {
		sessionCount = len(launcher.ActiveSessions())
	}

	resp := map[string]any{
		"running":         running,
		"adapters":        adapters,
		"active_sessions": sessionCount,
	}
	if daemon != nil {
		resp["policy"] = daemon.Policy()
		if daemon.Events() != nil {
			resp["event_count"] = daemon.Events().Count()
		}
	}
	writeJSON(w, resp)
}

func (h *Handler) toggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	daemon := h.daemon()
	if daemon == nil {
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]string{"error": "orchestrator not configured"})
		return
	}

	var body struct {
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	switch body.Action {
	case "start":
		daemon.Start(h.baseContext())
		writeJSON(w, map[string]string{"status": "started"})
	case "stop":
		daemon.Stop()
		writeJSON(w, map[string]string{"status": "stopped"})
	default:
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "action must be start or stop"})
	}
}

func (h *Handler) sessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	launcher := h.launcher()
	if launcher == nil {
		writeJSON(w, map[string]any{"sessions": []any{}})
		return
	}

	sessions := launcher.ActiveSessions()
	out := make([]map[string]any, len(sessions))
	for i, s := range sessions {
		out[i] = map[string]any{
			"session_id": s.SessionID,
			"adapter":    s.AdapterName,
			"task_id":    s.TaskID,
			"started_at": s.StartedAt,
		}
	}
	writeJSON(w, map[string]any{"sessions": out})
}

func (h *Handler) detectIDEs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string]any{"ides": orchestrator.DetectIDEs()})
}

func (h *Handler) events(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	daemon := h.daemon()
	if daemon == nil || daemon.Events() == nil {
		writeJSON(w, map[string]any{"events": []any{}})
		return
	}

	n := 50
	if q := r.URL.Query().Get("limit"); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 && v <= 500 {
			n = v
		}
	}
	writeJSON(w, map[string]any{"events": daemon.Events().Recent(n), "total": daemon.Events().Count()})
}

func (h *Handler) taskTimeline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	taskID := r.URL.Query().Get("task_id")
	if taskID == "" {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "task_id required"})
		return
	}
	daemon := h.daemon()
	if daemon == nil || daemon.Events() == nil {
		writeJSON(w, map[string]any{"events": []any{}})
		return
	}
	writeJSON(w, map[string]any{"task_id": taskID, "events": daemon.Events().ForTask(taskID)})
}

func (h *Handler) policy(w http.ResponseWriter, r *http.Request) {
	daemon := h.daemon()
	if daemon == nil {
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]string{"error": "orchestrator not configured"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, daemon.Policy())
	case http.MethodPut:
		var p orchestrator.DaemonPolicy
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		daemon.SetPolicy(p)
		writeJSON(w, map[string]any{"status": "updated", "policy": p})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) addCustomAdapter(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	launcher := h.launcher()
	if launcher == nil {
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]string{"error": "orchestrator not configured"})
		return
	}

	var cfg orchestrator.GenericAdapterConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if cfg.AdapterName == "" || cfg.Binary == "" || cfg.MCPConfigPath == "" {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "adapter_name, binary, and mcp_config_path are required"})
		return
	}

	adapter := orchestrator.NewGenericAdapter(cfg)
	launcher.RegisterAdapter(adapter)
	writeJSONStatus(w, http.StatusCreated, map[string]any{
		"status":    "registered",
		"name":      cfg.AdapterName,
		"available": adapter.Available(),
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeJSONStatus(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
