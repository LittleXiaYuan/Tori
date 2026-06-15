// Package sample is a reference v2 capability pack (packruntime.Module) that
// depends ONLY on the kernel Host contract — never on *gateway.Gateway. It is
// the proof-of-model for the Tier 0 microkernel authoring style described in
// doc/MICROKERNEL-PACK-BLUEPRINT.md: a pack declares its routes, wires its
// dependencies from Host in Init, and runs background work between Start/Stop.
package sample

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"

	"yunque-agent/pkg/packruntime"
	"yunque-agent/pkg/skills"
)

// PackID is the stable identifier of the reference pack.
const PackID = "yunque.pack.kernel-sample"

// Module is a minimal capability pack built entirely on packruntime.Host.
type Module struct {
	host    packruntime.Host
	started atomic.Bool
}

// New constructs the reference module. Routes/Start require Init first.
func New() *Module { return &Module{} }

// compile-time assertion: this is a valid v2 Module.
var _ packruntime.Module = (*Module)(nil)

func (m *Module) PackID() string { return PackID }

// Init wires the pack against the kernel. It keeps only the Host handle — no
// reference to any concrete host type leaks in.
func (m *Module) Init(host packruntime.Host) error {
	m.host = host
	return nil
}

// Routes is the HTTP surface mounted by the kernel.
func (m *Module) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Method: http.MethodGet, Path: "/v1/packs/sample/ping", Handler: m.handlePing},
		{Method: http.MethodPost, Path: "/v1/packs/sample/echo", Handler: m.handleEcho},
	}
}

// Start marks the pack live. A real pack would launch its workers here, bound to
// ctx so Stop (or ctx cancellation) tears them down.
func (m *Module) Start(ctx context.Context) error {
	m.started.Store(true)
	if m.host != nil {
		m.host.Logger().Info("kernel-sample pack started", "pack", PackID)
		m.host.Events().Emit("pack.sample.started", map[string]any{"pack": PackID})
	}
	return nil
}

// Stop marks the pack stopped.
func (m *Module) Stop(ctx context.Context) error {
	m.started.Store(false)
	return nil
}

// compile-time assertion: the reference pack also contributes prompt context.
var _ packruntime.ContextProvider = (*Module)(nil)

// BuildContext makes the reference pack a ContextProvider: when the pack is
// enabled (started), it injects a context section into the agent's prompt. This
// proves a Pack's enablement flows into 云雀's reasoning, not just its routes.
func (m *Module) BuildContext(ctx context.Context, message, tenant string) string {
	if !m.started.Load() {
		return ""
	}
	return "## kernel-sample pack\n(reference pack enabled — this line proves an enabled pack can inject context into the agent's prompt.)"
}

// compile-time assertion: the reference pack also contributes an agent tool.
var _ packruntime.SkillProvider = (*Module)(nil)

// Skills makes the reference pack a SkillProvider: enabling the pack adds a tool
// the planner can call (the host registers it into the skill registry), proving
// a downloaded/enabled pack can give the agent a new callable capability —
// disabling removes it. Declared unconditionally; the host gates by enablement.
func (m *Module) Skills() []skills.Skill {
	return []skills.Skill{samplePingSkill{}}
}

// samplePingSkill is a trivial agent tool contributed by the reference pack.
type samplePingSkill struct{}

func (samplePingSkill) Name() string { return "kernel_sample_ping" }

func (samplePingSkill) Description() string {
	return "Reference pack tool: returns 'pong'. Demonstrates that an enabled capability pack can add a tool the agent invokes."
}

func (samplePingSkill) Parameters() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}

func (samplePingSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	return "pong", nil
}

func (m *Module) handlePing(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"pong":    true,
		"pack":    PackID,
		"started": m.started.Load(),
	})
}

// handleEcho demonstrates calling a kernel port (KV) from a pack that only knows
// Host: it persists the value via the kernel's pack-scoped KV and echoes it back.
func (m *Module) handleEcho(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	stored := false
	if m.host != nil && body.Key != "" {
		if err := m.host.KV().Put(r.Context(), body.Key, body.Value); err == nil {
			stored = true
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"echo":       body.Value,
		"stored_key": body.Key,
		"stored":     stored,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
