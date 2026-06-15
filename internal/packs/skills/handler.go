// Package skillspack mounts the skill-management HTTP surface (/v1/skills/*) as a
// Pack Runtime backend module.
//
// Migration status: the pack owns route registration + the enable/disable gate.
// The skill listing read surface (/v1/skills) has been "filled in" — it is served
// natively here from the skill registry + metrics. The scan/dynamic/approve/reject
// routes stay on the gateway bridge for now. SkillHub / market keep their own
// gateway routes.
package skillspack

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"

	"yunque-agent/internal/agentcore/task"
	"yunque-agent/internal/apperror"
	"yunque-agent/internal/observe"
	"yunque-agent/pkg/packruntime"
	"yunque-agent/pkg/skills"
)

const PackID = "yunque.pack.skills"

// SkillsGateway is the narrow gateway surface the still-bridged skills routes need.
type SkillsGateway interface {
	HandleSkillsPack(w http.ResponseWriter, r *http.Request)
}

// SkillsRegistry is the narrow registry surface the native handlers need
// (listing + dynamic skill review). *skills.Registry satisfies it.
type SkillsRegistry interface {
	All() []skills.Skill
	CategoryOf(name string) string
	Categories() []*skills.SkillCategory
	Get(name string) (skills.Skill, bool)
	Remove(name string)
}

// MetricsSource exposes the metrics snapshot for per-skill usage stats.
type MetricsSource interface {
	Snapshot() observe.MetricsSnapshot
}

// Handler is the skills pack's backend module. registry/metrics may be nil (e.g.
// in tests that only exercise the route gates).
type Handler struct {
	gateway     SkillsGateway
	registry    SkillsRegistry
	metrics     MetricsSource
	host        packruntime.Host
	started     atomic.Bool
	saveDynamic func() error // persists dynamic-skill changes (injected by host)
}

// SetDynamicSave injects the persistence hook for dynamic-skill approve/reject.
// The host wires it to task.SaveDynamicSkills so the pack owns the handlers
// without importing the concrete registry persistence path.
func (h *Handler) SetDynamicSave(fn func() error) { h.saveDynamic = fn }

// NewHandler builds the skills pack backed only by the gateway bridge.
func NewHandler(gateway SkillsGateway) *Handler { return &Handler{gateway: gateway} }

// NewHandlerWithService builds the skills pack with the registry + metrics wired,
// so the /v1/skills listing is served natively by this package.
func NewHandlerWithService(gateway SkillsGateway, registry SkillsRegistry, metrics MetricsSource) *Handler {
	return &Handler{gateway: gateway, registry: registry, metrics: metrics}
}

// PackID returns the stable manifest id.
func (h *Handler) PackID() string { return PackID }

// compile-time assertion: Skills is a v2 capability Module (Tier 0 microkernel).
var _ packruntime.Module = (*Handler)(nil)

// Init wires the pack against the kernel Host (deps arrive via narrow interfaces).
func (h *Handler) Init(host packruntime.Host) error {
	h.host = host
	return nil
}

// Start marks the pack live on enable.
func (h *Handler) Start(ctx context.Context) error {
	h.started.Store(true)
	if h.host != nil {
		h.host.Logger().Info("skills pack started", "pack", PackID)
	}
	return nil
}

// Stop marks the pack stopped on disable.
func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

// Routes mounts the core /v1/skills/* surface. /v1/skills is served natively; the
// scan/dynamic/approve/reject routes are dispatched to the gateway bridge.
func (h *Handler) Routes() []packruntime.BackendRoute {
	bridge := h.gateway.HandleSkillsPack
	methods := []string{
		http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch,
	}
	mk := func(path string, handler http.HandlerFunc) packruntime.BackendRoute {
		return packruntime.BackendRoute{Methods: methods, Path: path, Handler: handler}
	}
	return []packruntime.BackendRoute{
		mk("/v1/skills", h.handleSkills),
		mk("/v1/skills/scan", bridge),
		// dynamic / approve / reject are de-shelled — served natively here
		// (logic moved out of the gateway). Only scan remains bridged.
		mk("/v1/skills/dynamic", h.handleDynamicGet),
		mk("/v1/skills/approve", h.handleApprove),
		mk("/v1/skills/reject", h.handleReject),
	}
}

// handleApprove approves a pending dynamic skill (de-shelled from the gateway):
// it flips the skill's approval status, optionally updates its instruction, and
// persists via the injected saveDynamic hook.
func (h *Handler) handleApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	var req struct {
		Name        string `json:"name"`
		Instruction string `json:"instruction,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeInvalidField, "invalid request")
		return
	}
	if h.registry == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "skill registry not configured")
		return
	}
	sk, ok := h.registry.Get(req.Name)
	if !ok {
		apperror.WriteCode(w, apperror.CodeNotFound, "skill not found")
		return
	}
	ds, ok := sk.(*task.DynamicSkill)
	if !ok {
		apperror.WriteCode(w, apperror.CodeInvalidField, "not a dynamic skill")
		return
	}
	ds.SetApprovalStatus("approved")
	if req.Instruction != "" {
		ds.UpdateInstruction(req.Instruction)
	}
	if h.saveDynamic != nil {
		if err := h.saveDynamic(); err != nil {
			apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "save dynamic skills", err))
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleReject removes a pending dynamic skill (de-shelled from the gateway).
func (h *Handler) handleReject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeInvalidField, "invalid request")
		return
	}
	if h.registry == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "skill registry not configured")
		return
	}
	sk, ok := h.registry.Get(req.Name)
	if !ok {
		apperror.WriteCode(w, apperror.CodeNotFound, "skill not found")
		return
	}
	if _, ok := sk.(*task.DynamicSkill); !ok {
		apperror.WriteCode(w, apperror.CodeInvalidField, "not a dynamic skill")
		return
	}
	h.registry.Remove(req.Name)
	if h.saveDynamic != nil {
		if err := h.saveDynamic(); err != nil {
			apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "save dynamic skills", err))
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleDynamicGet lists pending/dynamic skills natively from the registry — the
// logic was moved out of the gateway (Gateway.handleSkillsDynamicGet) so the
// pack owns this surface end-to-end, not just its route. Degrades to an empty
// list when the registry is not configured, matching the pack's other native
// handlers.
func (h *Handler) handleDynamicGet(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	dynamic := []task.DynamicSkillDef{}
	if h.registry != nil {
		for _, sk := range h.registry.All() {
			if ds, ok := sk.(*task.DynamicSkill); ok {
				dynamic = append(dynamic, ds.Def())
			}
		}
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"skills": dynamic})
}

func (h *Handler) handleSkills(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	type skillInfo struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Parameters  map[string]any `json:"parameters"`
		Category    string         `json:"category,omitempty"`
		UsageTotal  int64          `json:"usage_total"`
		SuccessRate float64        `json:"success_rate"`
	}
	type catInfo struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	usageMap := make(map[string]struct {
		total       int64
		successRate float64
	})
	if h.metrics != nil {
		snap := h.metrics.Snapshot()
		for _, ss := range snap.Skills {
			usageMap[ss.Name] = struct {
				total       int64
				successRate float64
			}{total: ss.Total, successRate: ss.SuccessRate}
		}
	}

	out := make([]skillInfo, 0)
	cats := make([]catInfo, 0)
	if h.registry != nil {
		for _, s := range h.registry.All() {
			u := usageMap[s.Name()]
			out = append(out, skillInfo{
				Name:        s.Name(),
				Description: s.Description(),
				Parameters:  s.Parameters(),
				Category:    h.registry.CategoryOf(s.Name()),
				UsageTotal:  u.total,
				SuccessRate: u.successRate,
			})
		}
		for _, c := range h.registry.Categories() {
			cats = append(cats, catInfo{ID: c.ID, Name: c.Name, Description: c.Description})
		}
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"skills": out, "count": len(out), "categories": cats})
}
