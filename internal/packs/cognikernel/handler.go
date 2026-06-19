package cognikernelpack

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"yunque-agent/internal/apperror"
	"yunque-agent/pkg/cogni"
	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.cogni-kernel"

const (
	CollectionRoute         = "/v1/cognis"
	SubResourceRoute        = "/v1/cognis/"
	FederationRoute         = "/v1/cognis/federation"
	FederationPeersRoute    = "/v1/cognis/federation/peers"
	FederationDiscoverRoute = "/v1/cognis/federation/discover"
	EconomicsRoute          = "/v1/cognis/economics"
	RouteDecisionRoute      = "/v1/cognis/route"
	RuntimePackStateRoute   = "/v1/cognis/runtime/pack-state"
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

type BusProvider interface {
	CogniBus() *cogni.CogniBus
}

type FederationProvider interface {
	CogniFederation() *cogni.CogniFederation
}

type CostTrackerProvider interface {
	CogniCostTracker() *cogni.CostTracker
}

type ExperienceProvider interface {
	CogniExperiences() map[string]*cogni.ExperienceStore
}

type RegistryProvider interface {
	CogniRegistry() *cogni.Registry
}

type WorkflowEngineProvider interface {
	CogniWorkflowEngine() *cogni.WorkflowEngine
}

type TraceStoreProvider interface {
	CogniTraceStore() cogni.TraceStore
}

type SentinelProvider interface {
	CogniSentinel() *cogni.Sentinel
}

type DirectoryProvider interface {
	CogniDirectory() string
}

type Dependencies struct {
	BusProvider         BusProvider
	FederationProvider  FederationProvider
	CostTrackerProvider CostTrackerProvider
	ExperienceProvider  ExperienceProvider
	RegistryProvider    RegistryProvider
	WorkflowProvider    WorkflowEngineProvider
	TraceStoreProvider  TraceStoreProvider
	SentinelProvider    SentinelProvider
	DirectoryProvider   DirectoryProvider
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
	return NewHandlerWithDeps(api, reporter, inferDependencies(api))
}

func NewHandlerWithDeps(api API, reporter RuntimeStateReporter, deps Dependencies) *Handler {
	if reporter == nil {
		if inferred, ok := api.(RuntimeStateReporter); ok {
			reporter = inferred
		}
	}
	deps = mergeDependencies(deps, inferDependencies(api))
	return &Handler{router: NewRouterWithDeps(api, reporter, deps)}
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
	api                API
	reporter           RuntimeStateReporter
	busProvider        BusProvider
	federationProvider FederationProvider
	costTracker        CostTrackerProvider
	experienceProvider ExperienceProvider
	registryProvider   RegistryProvider
	workflowProvider   WorkflowEngineProvider
	traceProvider      TraceStoreProvider
	sentinelProvider   SentinelProvider
	directoryProvider  DirectoryProvider
}

func NewRouter(api API, reporter RuntimeStateReporter) *Router {
	return NewRouterWithDeps(api, reporter, inferDependencies(api))
}

func NewRouterWithBus(api API, reporter RuntimeStateReporter, busProvider BusProvider) *Router {
	return NewRouterWithDeps(api, reporter, Dependencies{BusProvider: busProvider})
}

func NewRouterWithDeps(api API, reporter RuntimeStateReporter, deps Dependencies) *Router {
	deps = mergeDependencies(deps, inferDependencies(api))
	return &Router{
		api:                api,
		reporter:           reporter,
		busProvider:        deps.BusProvider,
		federationProvider: deps.FederationProvider,
		costTracker:        deps.CostTrackerProvider,
		experienceProvider: deps.ExperienceProvider,
		registryProvider:   deps.RegistryProvider,
		workflowProvider:   deps.WorkflowProvider,
		traceProvider:      deps.TraceStoreProvider,
		sentinelProvider:   deps.SentinelProvider,
		directoryProvider:  deps.DirectoryProvider,
	}
}

func (r *Router) Routes() []packruntime.BackendRoute {
	routes := []packruntime.BackendRoute{
		{Methods: []string{http.MethodGet, http.MethodPost}, Path: CollectionRoute, Handler: r.ServeCogniKernel},
		{Methods: []string{http.MethodGet, http.MethodPost, http.MethodDelete}, Path: SubResourceRoute, Handler: r.ServeCogniKernel},
		{Methods: []string{http.MethodGet}, Path: FederationRoute, Handler: r.HandleFederationStatus},
		{Methods: []string{http.MethodGet, http.MethodPost}, Path: FederationPeersRoute, Handler: r.HandleFederationPeers},
		{Methods: []string{http.MethodPost}, Path: FederationDiscoverRoute, Handler: r.HandleFederationDiscover},
		{Methods: []string{http.MethodGet}, Path: EconomicsRoute, Handler: r.HandleEconomics},
		{Methods: []string{http.MethodGet}, Path: "/v1/cognis/traces", Handler: r.HandleTracesAll},
		{Methods: []string{http.MethodGet}, Path: "/v1/cognis/stats", Handler: r.HandleTraceStats},
		{Methods: []string{http.MethodGet}, Path: "/v1/cognis/health", Handler: r.HandleHealthAll},
		{Methods: []string{http.MethodGet}, Path: "/v1/cognis/alerts", Handler: r.HandleAlerts},
		{Methods: []string{http.MethodPost}, Path: "/v1/cognis/alerts/scan", Handler: r.HandleAlertsScan},
		{Methods: []string{http.MethodGet, http.MethodPost}, Path: "/v1/cognis/verify", Handler: r.HandleVerifyAll},
		{Methods: []string{http.MethodPost}, Path: RouteDecisionRoute, Handler: r.HandleRouteDecision},
		{Methods: []string{http.MethodGet}, Path: RuntimePackStateRoute, Handler: r.HandleRuntimePackState},
	}
	for _, route := range delegatedRouteSpecs() {
		routes = append(routes, packruntime.BackendRoute{
			Methods: route.methods,
			Path:    route.path,
			Handler: r.ServeCogniKernel,
		})
	}
	return routes
}

func (r *Router) RuntimeRoutes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Methods: []string{http.MethodGet}, Path: RuntimePackStateRoute, Handler: r.HandleRuntimePackState},
	}
}

func (r *Router) ServeCogniKernel(w http.ResponseWriter, req *http.Request) {
	if r.serveRegistry(w, req) {
		return
	}
	if r.serveExperience(w, req) {
		return
	}
	if r.serveWorkflow(w, req) {
		return
	}
	if r.serveObservability(w, req) {
		return
	}
	if r == nil || r.api == nil {
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]any{
			"error":   "cogni api handler not configured",
			"pack_id": PackID,
		})
		return
	}
	r.api.ServeCogniKernel(w, req)
}

func (r *Router) serveRegistry(w http.ResponseWriter, req *http.Request) bool {
	path := strings.TrimPrefix(req.URL.Path, CollectionRoute)
	path = strings.Trim(path, "/")
	if path == "" {
		r.HandleCollection(w, req)
		return true
	}
	if path == "reload" {
		r.HandleReload(w, req)
		return true
	}

	segs := strings.Split(path, "/")
	if len(segs) == 1 {
		if reservedCogniTopLevel(segs[0]) {
			return false
		}
		r.HandleByID(w, req, segs[0])
		return true
	}
	if len(segs) == 2 {
		switch segs[1] {
		case "enable":
			r.HandleSetEnabled(w, req, segs[0], true)
			return true
		case "disable":
			r.HandleSetEnabled(w, req, segs[0], false)
			return true
		}
	}
	return false
}

func reservedCogniTopLevel(path string) bool {
	switch path {
	case "runtime", "generate", "export", "import", "evolution", "federation", "economics", "route",
		"traces", "stats", "health", "alerts", "verify", "reload":
		return true
	default:
		return false
	}
}

func (r *Router) HandleCollection(w http.ResponseWriter, req *http.Request) {
	registry := r.cogniRegistry()
	if registry == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "cogni registry not configured")
		return
	}
	switch req.Method {
	case http.MethodGet:
		entries := registry.List()
		health := map[string]cogni.HealthMetrics{}
		if traces := r.cogniTraceStore(); traces != nil {
			mon := cogni.NewMonitor(traces)
			for _, hm := range mon.ComputeAll(0) {
				health[hm.ID] = hm
			}
		}
		writeJSON(w, map[string]any{
			"cognis":  entries,
			"health":  health,
			"count":   len(entries),
			"version": registry.Version(),
			"dir":     r.cogniDirectory(),
		})
	case http.MethodPost:
		var d cogni.Declaration
		if err := json.NewDecoder(req.Body).Decode(&d); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON body: "+err.Error())
			return
		}
		if err := registry.Add(&d, "api"); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
			return
		}
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, map[string]any{"status": "ok", "id": d.ID})
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET or POST")
	}
}

func (r *Router) HandleByID(w http.ResponseWriter, req *http.Request, id string) {
	registry := r.cogniRegistry()
	if registry == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "cogni registry not configured")
		return
	}
	switch req.Method {
	case http.MethodGet:
		decl, ok := registry.Get(id)
		if !ok {
			apperror.WriteCode(w, apperror.CodeNotFound, "cogni not found: "+id)
			return
		}
		writeJSON(w, map[string]any{
			"id":          decl.ID,
			"declaration": decl,
			"enabled":     registry.IsEnabled(id),
		})
	case http.MethodDelete:
		if !registry.Remove(id) {
			apperror.WriteCode(w, apperror.CodeNotFound, "cogni not found: "+id)
			return
		}
		writeJSON(w, map[string]any{"status": "removed", "id": id})
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET or DELETE")
	}
}

func (r *Router) HandleSetEnabled(w http.ResponseWriter, req *http.Request, id string, enabled bool) {
	if req.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	registry := r.cogniRegistry()
	if registry == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "cogni registry not configured")
		return
	}
	if err := registry.SetEnabled(id, enabled); err != nil {
		apperror.WriteCode(w, apperror.CodeNotFound, err.Error())
		return
	}
	state := "disabled"
	if enabled {
		state = "enabled"
	}
	writeJSON(w, map[string]any{"status": state, "id": id})
}

func (r *Router) HandleReload(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	registry := r.cogniRegistry()
	if registry == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "cogni registry not configured")
		return
	}
	dir := r.cogniDirectory()
	if dir == "" {
		apperror.WriteCode(w, apperror.CodeInternal, "cogni directory not configured")
		return
	}
	summary, err := registry.ReloadFromDir(dir)
	if err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, err.Error())
		return
	}

	errs := make([]map[string]string, 0, len(summary.Errors))
	for _, e := range summary.Errors {
		errs = append(errs, map[string]string{
			"file":  filepath.Base(e.Path),
			"path":  e.Path,
			"error": e.Err.Error(),
		})
	}

	writeJSON(w, map[string]any{
		"status":  "ok",
		"dir":     dir,
		"added":   summary.Added,
		"updated": summary.Updated,
		"removed": summary.Removed,
		"errors":  errs,
		"version": registry.Version(),
	})
}

func (r *Router) HandleFederationStatus(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	federation := r.cogniFederation()
	if federation == nil {
		writeJSON(w, map[string]any{"enabled": false})
		return
	}
	stats := federation.Stats()
	stats["enabled"] = true
	writeJSON(w, stats)
}

func (r *Router) HandleFederationPeers(w http.ResponseWriter, req *http.Request) {
	federation := r.cogniFederation()
	switch req.Method {
	case http.MethodGet:
		if federation == nil {
			writeJSON(w, map[string]any{"peers": []any{}})
			return
		}
		peers := federation.Peers()
		writeJSON(w, map[string]any{"peers": peers, "count": len(peers)})
	case http.MethodPost:
		if federation == nil {
			apperror.WriteCode(w, apperror.CodeInternal, "federation not configured")
			return
		}
		var peer cogni.FederationPeer
		if err := json.NewDecoder(req.Body).Decode(&peer); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
			return
		}
		federation.AddPeer(peer)
		writeJSON(w, map[string]any{"status": "ok", "id": peer.ID})
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET or POST")
	}
}

func (r *Router) HandleFederationDiscover(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	federation := r.cogniFederation()
	if federation == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "federation not configured")
		return
	}
	skills := federation.DiscoverRemoteSkills(req.Context())
	writeJSON(w, map[string]any{
		"skills": skills,
		"count":  len(skills),
	})
}

func (r *Router) HandleEconomics(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	tracker := r.cogniCostTracker()
	if tracker == nil {
		writeJSON(w, map[string]any{"enabled": false})
		return
	}
	writeJSON(w, map[string]any{
		"enabled": true,
		"summary": tracker.DailySummary(),
	})
}

func (r *Router) HandleHealthAll(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	traces := r.cogniTraceStore()
	if traces == nil {
		writeJSON(w, map[string]any{"health": []any{}, "count": 0})
		return
	}
	out := cogni.NewMonitor(traces).ComputeAll(traceLimit(req))
	writeJSON(w, map[string]any{
		"health": out,
		"count":  len(out),
	})
}

func (r *Router) HandleHealthByID(w http.ResponseWriter, req *http.Request, id string) {
	if req.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	traces := r.cogniTraceStore()
	if traces == nil {
		writeJSON(w, cogni.HealthMetrics{ID: id, Status: "idle"})
		return
	}
	writeJSON(w, cogni.NewMonitor(traces).ComputeFor(id, traceLimit(req)))
}

func (r *Router) HandleTracesAll(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	traces := r.cogniTraceStore()
	if traces == nil {
		writeJSON(w, map[string]any{"traces": []any{}, "count": 0})
		return
	}
	out := traces.Recent(traceLimit(req))
	writeJSON(w, map[string]any{
		"traces": out,
		"count":  len(out),
	})
}

func (r *Router) HandleTracesByID(w http.ResponseWriter, req *http.Request, id string) {
	if req.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	traces := r.cogniTraceStore()
	if traces == nil {
		writeJSON(w, map[string]any{"traces": []any{}, "count": 0})
		return
	}
	out := traces.ByCogni(id, traceLimit(req))
	writeJSON(w, map[string]any{
		"id":     id,
		"traces": out,
		"count":  len(out),
	})
}

func (r *Router) HandleTraceStats(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	traces := r.cogniTraceStore()
	if traces == nil {
		writeJSON(w, cogni.TraceStats{})
		return
	}
	writeJSON(w, traces.Stats())
}

func (r *Router) HandleAlerts(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	sentinel := r.cogniSentinel()
	if sentinel == nil {
		writeJSON(w, map[string]any{"alerts": []any{}, "count": 0})
		return
	}
	alerts := sentinel.Alerts()
	writeJSON(w, map[string]any{"alerts": alerts, "count": len(alerts)})
}

func (r *Router) HandleAlertsScan(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	sentinel := r.cogniSentinel()
	if sentinel == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "cogni sentinel not configured")
		return
	}
	alerts := sentinel.Scan()
	writeJSON(w, map[string]any{"alerts": alerts, "count": len(alerts)})
}

func (r *Router) HandleVerifyAll(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost && req.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET or POST")
		return
	}
	registry := r.cogniRegistry()
	if registry == nil {
		writeJSON(w, map[string]any{"results": map[string]any{}, "failures": []any{}})
		return
	}
	results := registry.VerifyAll()
	writeJSON(w, map[string]any{
		"results":  results,
		"failures": cogni.FailedChecks(results),
	})
}

func (r *Router) HandleVerifyByID(w http.ResponseWriter, req *http.Request, id string) {
	if req.Method != http.MethodPost && req.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET or POST")
		return
	}
	registry := r.cogniRegistry()
	if registry == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "cogni registry not configured")
		return
	}
	decl, ok := registry.Get(id)
	if !ok {
		apperror.WriteCode(w, apperror.CodeNotFound, "cogni not found: "+id)
		return
	}
	results := cogni.VerifyDeclaration(decl, nil)
	passed, failed := 0, 0
	for _, result := range results {
		if result.Passed {
			passed++
		} else if result.Reason != "no assertion configured (ignored)" {
			failed++
		}
	}
	writeJSON(w, map[string]any{
		"id":      id,
		"results": results,
		"passed":  passed,
		"failed":  failed,
	})
}

func (r *Router) HandleRouteDecision(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	if r == nil || r.busProvider == nil || r.busProvider.CogniBus() == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "cogni bus not configured")
		return
	}

	var body struct {
		Message  string   `json:"message"`
		TenantID string   `json:"tenant_id"`
		Channel  string   `json:"channel"`
		Tags     []string `json:"tags"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
		return
	}
	if body.Message == "" {
		apperror.WriteCode(w, apperror.CodeMissingField, "message is required")
		return
	}

	result := r.busProvider.CogniBus().Route(req.Context(), cogni.Session{
		Message:  body.Message,
		TenantID: body.TenantID,
		Channel:  body.Channel,
		Tags:     body.Tags,
	})
	writeJSON(w, result)
}

func (r *Router) serveExperience(w http.ResponseWriter, req *http.Request) bool {
	id, rest, ok := experienceRoute(req.URL.Path)
	if !ok {
		return false
	}
	switch {
	case rest == "experience":
		r.HandleExperience(w, req, id)
	case rest == "experience/record":
		r.HandleExperienceRecord(w, req, id)
	case strings.HasPrefix(rest, "experience/patterns/"):
		r.HandleExperiencePatternRoute(w, req, id, strings.TrimPrefix(rest, "experience/"))
	default:
		return false
	}
	return true
}

func experienceRoute(path string) (id, rest string, ok bool) {
	path = strings.TrimPrefix(path, SubResourceRoute)
	path = strings.Trim(path, "/")
	if path == "" {
		return "", "", false
	}
	segs := strings.SplitN(path, "/", 2)
	if len(segs) != 2 || segs[0] == "" {
		return "", "", false
	}
	return segs[0], segs[1], strings.HasPrefix(segs[1], "experience")
}

func (r *Router) HandleExperience(w http.ResponseWriter, req *http.Request, id string) {
	if req.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	if r.experienceStores() == nil {
		writeJSON(w, map[string]any{"enabled": false})
		return
	}
	es, ok := r.experienceStore(id)
	if !ok {
		writeJSON(w, map[string]any{"enabled": false, "id": id})
		return
	}
	writeJSON(w, map[string]any{
		"id":           id,
		"enabled":      true,
		"summary":      es.Summary(5),
		"stats":        es.Stats(),
		"tool_memory":  es.ToolMemory(""),
		"patterns":     es.Patterns(),
		"domain_facts": es.DomainFacts(),
	})
}

func (r *Router) HandleExperienceRecord(w http.ResponseWriter, req *http.Request, id string) {
	if req.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	es, ok := r.experienceStore(id)
	if !ok {
		apperror.WriteCode(w, apperror.CodeNotFound, "no experience store for cogni: "+id)
		return
	}

	var body struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON: "+err.Error())
		return
	}

	switch body.Type {
	case "tool_memory":
		var tm cogni.ToolExperience
		if err := json.Unmarshal(body.Data, &tm); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
			return
		}
		es.AddToolMemory(tm)
		writeJSON(w, map[string]any{"status": "ok", "type": "tool_memory"})
	case "pattern":
		var p cogni.BehaviorPattern
		if err := json.Unmarshal(body.Data, &p); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
			return
		}
		es.SuggestPattern(p)
		writeJSON(w, map[string]any{"status": "ok", "type": "pattern", "id": p.ID})
	case "fact":
		var f cogni.DomainFact
		if err := json.Unmarshal(body.Data, &f); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
			return
		}
		es.AddFact(f)
		writeJSON(w, map[string]any{"status": "ok", "type": "fact"})
	default:
		apperror.WriteCode(w, apperror.CodeBadRequest, "type must be tool_memory, pattern, or fact")
	}
}

func (r *Router) HandleExperiencePatternRoute(w http.ResponseWriter, req *http.Request, id, rest string) {
	parts := strings.Split(rest, "/")
	if len(parts) != 3 || parts[0] != "patterns" || parts[2] != "confirm" {
		apperror.WriteCode(w, apperror.CodeNotFound, "unknown cogni experience pattern sub-resource")
		return
	}
	r.HandleExperienceConfirmPattern(w, req, id, parts[1])
}

func (r *Router) HandleExperienceConfirmPattern(w http.ResponseWriter, req *http.Request, id, patternID string) {
	if req.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	if patternID == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "pattern id is required")
		return
	}
	es, ok := r.experienceStore(id)
	if !ok {
		apperror.WriteCode(w, apperror.CodeNotFound, "no experience store for cogni: "+id)
		return
	}
	if !es.ConfirmPattern(patternID) {
		apperror.WriteCode(w, apperror.CodeNotFound, "experience pattern not found: "+patternID)
		return
	}

	writeJSON(w, map[string]any{"status": "ok", "type": "pattern", "id": patternID, "confirmed": true})
}

func (r *Router) experienceStore(id string) (*cogni.ExperienceStore, bool) {
	stores := r.experienceStores()
	if stores == nil {
		return nil, false
	}
	es, ok := stores[id]
	return es, ok
}

func (r *Router) experienceStores() map[string]*cogni.ExperienceStore {
	if r == nil || r.experienceProvider == nil {
		return nil
	}
	return r.experienceProvider.CogniExperiences()
}

func (r *Router) serveWorkflow(w http.ResponseWriter, req *http.Request) bool {
	id, rest, ok := workflowRoute(req.URL.Path)
	if !ok {
		return false
	}
	switch {
	case rest == "workflows":
		r.HandleWorkflowsList(w, req, id)
	case strings.HasPrefix(rest, "workflow/"):
		r.HandleWorkflowRun(w, req, id, strings.TrimPrefix(rest, "workflow/"))
	default:
		return false
	}
	return true
}

func (r *Router) serveObservability(w http.ResponseWriter, req *http.Request) bool {
	id, rest, ok := dynamicSubresourceRoute(req.URL.Path)
	if !ok {
		return false
	}
	switch rest {
	case "trace":
		r.HandleTracesByID(w, req, id)
	case "health":
		r.HandleHealthByID(w, req, id)
	case "verify":
		r.HandleVerifyByID(w, req, id)
	default:
		return false
	}
	return true
}

func dynamicSubresourceRoute(path string) (id, rest string, ok bool) {
	path = strings.TrimPrefix(path, SubResourceRoute)
	path = strings.Trim(path, "/")
	if path == "" {
		return "", "", false
	}
	segs := strings.SplitN(path, "/", 2)
	if len(segs) != 2 || segs[0] == "" || segs[1] == "" {
		return "", "", false
	}
	return segs[0], segs[1], true
}

func workflowRoute(path string) (id, rest string, ok bool) {
	path = strings.TrimPrefix(path, SubResourceRoute)
	path = strings.Trim(path, "/")
	if path == "" {
		return "", "", false
	}
	segs := strings.SplitN(path, "/", 2)
	if len(segs) != 2 || segs[0] == "" {
		return "", "", false
	}
	return segs[0], segs[1], segs[1] == "workflows" || strings.HasPrefix(segs[1], "workflow/")
}

func (r *Router) HandleWorkflowsList(w http.ResponseWriter, req *http.Request, id string) {
	if req.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	decl, ok := r.cogniDeclaration(id)
	if !ok {
		apperror.WriteCode(w, apperror.CodeNotFound, "cogni not found: "+id)
		return
	}
	writeJSON(w, map[string]any{
		"id":        id,
		"workflows": decl.Workflows,
		"count":     len(decl.Workflows),
	})
}

func (r *Router) HandleWorkflowRun(w http.ResponseWriter, req *http.Request, id, workflowName string) {
	if req.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	engine := r.cogniWorkflowEngine()
	if engine == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "workflow engine not configured")
		return
	}
	decl, ok := r.cogniDeclaration(id)
	if !ok {
		apperror.WriteCode(w, apperror.CodeNotFound, "cogni not found: "+id)
		return
	}
	workflowName = strings.Trim(workflowName, "/")
	if workflowName == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "workflow name required: /v1/cognis/{id}/workflow/{name}")
		return
	}

	var wf *cogni.WorkflowDef
	for i := range decl.Workflows {
		if decl.Workflows[i].Name == workflowName {
			wf = &decl.Workflows[i]
			break
		}
	}
	if wf == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "workflow not found: "+workflowName)
		return
	}

	var input map[string]any
	if req.Body != nil {
		if err := json.NewDecoder(req.Body).Decode(&input); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON body: "+err.Error())
			return
		}
	}

	writeJSON(w, engine.Run(req.Context(), *wf, input))
}

func (r *Router) cogniDeclaration(id string) (*cogni.Declaration, bool) {
	registry := r.cogniRegistry()
	if registry == nil {
		return nil, false
	}
	return registry.Get(id)
}

func (r *Router) cogniRegistry() *cogni.Registry {
	if r == nil || r.registryProvider == nil {
		return nil
	}
	return r.registryProvider.CogniRegistry()
}

func (r *Router) cogniWorkflowEngine() *cogni.WorkflowEngine {
	if r == nil || r.workflowProvider == nil {
		return nil
	}
	return r.workflowProvider.CogniWorkflowEngine()
}

func (r *Router) cogniTraceStore() cogni.TraceStore {
	if r == nil || r.traceProvider == nil {
		return nil
	}
	return r.traceProvider.CogniTraceStore()
}

func (r *Router) cogniSentinel() *cogni.Sentinel {
	if r == nil || r.sentinelProvider == nil {
		return nil
	}
	return r.sentinelProvider.CogniSentinel()
}

func (r *Router) cogniDirectory() string {
	if r == nil || r.directoryProvider == nil {
		return ""
	}
	return r.directoryProvider.CogniDirectory()
}

func (r *Router) cogniFederation() *cogni.CogniFederation {
	if r == nil || r.federationProvider == nil {
		return nil
	}
	return r.federationProvider.CogniFederation()
}

func (r *Router) cogniCostTracker() *cogni.CostTracker {
	if r == nil || r.costTracker == nil {
		return nil
	}
	return r.costTracker.CogniCostTracker()
}

func traceLimit(req *http.Request) int {
	limit := 50
	if raw := req.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 500 {
		limit = 500
	}
	return limit
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

type delegatedRouteSpec struct {
	path    string
	methods []string
}

func delegatedRouteSpecs() []delegatedRouteSpec {
	seen := map[string]int{}
	var out []delegatedRouteSpec
	for _, spec := range RouteSpecs() {
		switch spec.Path {
		case CollectionRoute,
			SubResourceRoute,
			FederationRoute,
			FederationPeersRoute,
			FederationDiscoverRoute,
			EconomicsRoute,
			"/v1/cognis/traces",
			"/v1/cognis/stats",
			"/v1/cognis/health",
			"/v1/cognis/alerts",
			"/v1/cognis/alerts/scan",
			"/v1/cognis/verify",
			RouteDecisionRoute,
			RuntimePackStateRoute:
			continue
		}
		method := spec.Method
		if method == "" {
			continue
		}
		if idx, ok := seen[spec.Path]; ok {
			out[idx].methods = append(out[idx].methods, method)
			continue
		}
		seen[spec.Path] = len(out)
		out = append(out, delegatedRouteSpec{path: spec.Path, methods: []string{method}})
	}
	return out
}

func inferDependencies(api API) Dependencies {
	var deps Dependencies
	if inferred, ok := api.(BusProvider); ok {
		deps.BusProvider = inferred
	}
	if inferred, ok := api.(FederationProvider); ok {
		deps.FederationProvider = inferred
	}
	if inferred, ok := api.(CostTrackerProvider); ok {
		deps.CostTrackerProvider = inferred
	}
	if inferred, ok := api.(ExperienceProvider); ok {
		deps.ExperienceProvider = inferred
	}
	if inferred, ok := api.(RegistryProvider); ok {
		deps.RegistryProvider = inferred
	}
	if inferred, ok := api.(WorkflowEngineProvider); ok {
		deps.WorkflowProvider = inferred
	}
	if inferred, ok := api.(TraceStoreProvider); ok {
		deps.TraceStoreProvider = inferred
	}
	if inferred, ok := api.(SentinelProvider); ok {
		deps.SentinelProvider = inferred
	}
	if inferred, ok := api.(DirectoryProvider); ok {
		deps.DirectoryProvider = inferred
	}
	return deps
}

func mergeDependencies(primary, fallback Dependencies) Dependencies {
	if primary.BusProvider == nil {
		primary.BusProvider = fallback.BusProvider
	}
	if primary.FederationProvider == nil {
		primary.FederationProvider = fallback.FederationProvider
	}
	if primary.CostTrackerProvider == nil {
		primary.CostTrackerProvider = fallback.CostTrackerProvider
	}
	if primary.ExperienceProvider == nil {
		primary.ExperienceProvider = fallback.ExperienceProvider
	}
	if primary.RegistryProvider == nil {
		primary.RegistryProvider = fallback.RegistryProvider
	}
	if primary.WorkflowProvider == nil {
		primary.WorkflowProvider = fallback.WorkflowProvider
	}
	if primary.TraceStoreProvider == nil {
		primary.TraceStoreProvider = fallback.TraceStoreProvider
	}
	if primary.SentinelProvider == nil {
		primary.SentinelProvider = fallback.SentinelProvider
	}
	if primary.DirectoryProvider == nil {
		primary.DirectoryProvider = fallback.DirectoryProvider
	}
	return primary
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
