// Package personapack mounts the persona HTTP surface as a native capability
// pack. Persona modes remain in the separate modes pack for now.
package personapack

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"

	"yunque-agent/internal/agentcore/persona"
	"yunque-agent/internal/apperror"
	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.persona"

type Gateway interface {
	Persona() *persona.Persona
	PersonaChain() *persona.PriorityChain
}

type Handler struct {
	personaOf      func() *persona.Persona
	personaChainOf func() *persona.PriorityChain
	host           packruntime.Host
	started        atomic.Bool
}

func New(gateway Gateway) *Handler {
	if gateway == nil {
		return NewProvider(nil, nil)
	}
	return NewProvider(gateway.Persona, gateway.PersonaChain)
}

func NewProvider(personaOf func() *persona.Persona, personaChainOf func() *persona.PriorityChain) *Handler {
	return &Handler{personaOf: personaOf, personaChainOf: personaChainOf}
}

var _ packruntime.Module = (*Handler)(nil)

func (h *Handler) PackID() string { return PackID }

func (h *Handler) Init(host packruntime.Host) error {
	h.host = host
	return nil
}

func (h *Handler) Start(ctx context.Context) error {
	h.started.Store(true)
	if h.host != nil {
		h.host.Logger().Info("persona pack started", "pack", PackID)
	}
	return nil
}

func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Methods: []string{http.MethodGet, http.MethodPut}, Path: "/v1/persona", Handler: h.Persona},
		{Methods: []string{http.MethodGet, http.MethodPost, http.MethodDelete}, Path: "/v1/persona/skills", Handler: h.PersonaSkills},
		{Methods: []string{http.MethodGet, http.MethodPost}, Path: "/v1/persona/presets", Handler: h.Presets},
		{Methods: []string{http.MethodPost, http.MethodDelete}, Path: "/v1/persona/presets/custom", Handler: h.CustomPreset},
		{Method: http.MethodPut, Path: "/v1/persona/presets/features", Handler: h.PresetFeatures},
	}
}

func RouteSpecs() []packruntime.BackendRouteSpec {
	return []packruntime.BackendRouteSpec{
		{Method: http.MethodGet, Path: "/v1/persona", Description: "Read the base persona identity, soul, and persona skills."},
		{Method: http.MethodPut, Path: "/v1/persona", Description: "Update the base persona identity and soul prompt."},
		{Method: http.MethodGet, Path: "/v1/persona/skills", Description: "List persona skill prompt fragments."},
		{Method: http.MethodPost, Path: "/v1/persona/skills", Description: "Add a persona skill prompt fragment."},
		{Method: http.MethodDelete, Path: "/v1/persona/skills", Description: "Delete a persona skill prompt fragment."},
		{Method: http.MethodGet, Path: "/v1/persona/presets", Description: "List persona presets and the active preset."},
		{Method: http.MethodPost, Path: "/v1/persona/presets", Description: "Switch the active persona preset."},
		{Method: http.MethodPost, Path: "/v1/persona/presets/custom", Description: "Create a custom persona preset."},
		{Method: http.MethodDelete, Path: "/v1/persona/presets/custom", Description: "Remove a custom persona preset."},
		{Method: http.MethodPut, Path: "/v1/persona/presets/features", Description: "Update feature flags for a persona preset."},
	}
}

func Paths() []string {
	return []string{
		"/v1/persona",
		"/v1/persona/skills",
		"/v1/persona/presets",
		"/v1/persona/presets/custom",
		"/v1/persona/presets/features",
	}
}

func (h *Handler) basePersona() *persona.Persona {
	if h.personaOf == nil {
		return nil
	}
	return h.personaOf()
}

func (h *Handler) chain() *persona.PriorityChain {
	if h.personaChainOf == nil {
		return nil
	}
	return h.personaChainOf()
}

func (h *Handler) presets() *persona.PresetManager {
	chain := h.chain()
	if chain == nil {
		return nil
	}
	return chain.Presets()
}

func (h *Handler) Persona(w http.ResponseWriter, r *http.Request) {
	p := h.basePersona()
	if p == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "persona not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"identity": p.Identity(),
			"soul":     p.Soul(),
			"skills":   p.Skills(),
		})
	case http.MethodPut:
		var req struct {
			Identity *string `json:"identity"`
			Soul     *string `json:"soul"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid json")
			return
		}
		if req.Identity != nil {
			if err := p.SetIdentity(*req.Identity); err != nil {
				apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "set identity", err))
				return
			}
		}
		if req.Soul != nil {
			if err := p.SetSoul(*req.Soul); err != nil {
				apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "set soul", err))
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET or PUT")
	}
}

func (h *Handler) PersonaSkills(w http.ResponseWriter, r *http.Request) {
	p := h.basePersona()
	if p == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "persona not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		skills := p.Skills()
		if skills == nil {
			skills = []persona.Skill{}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"skills": skills})
	case http.MethodPost:
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Content     string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "name is required")
			return
		}
		if err := p.AddSkill(req.Name, req.Description, req.Content); err != nil {
			apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "add skill", err))
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	case http.MethodDelete:
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "name is required")
			return
		}
		_ = p.DeleteSkill(req.Name)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET, POST, or DELETE")
	}
}

func (h *Handler) Presets(w http.ResponseWriter, r *http.Request) {
	pm := h.presets()
	if pm == nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"presets": []any{}, "active": ""})
		return
	}

	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"presets": pm.List(),
			"active":  pm.ActiveID(),
		})
	case http.MethodPost:
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
			http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
			return
		}
		if err := pm.Switch(req.ID); err != nil {
			http.Error(w, `{"error":"preset not found"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "active": req.ID})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) PresetFeatures(w http.ResponseWriter, r *http.Request) {
	pm := h.presets()
	if pm == nil {
		http.Error(w, `{"error":"presets not configured"}`, http.StatusNotFound)
		return
	}
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID       string          `json:"id"`
		Features map[string]bool `json:"features"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		http.Error(w, `{"error":"id and features required"}`, http.StatusBadRequest)
		return
	}
	if err := pm.SetFeatures(req.ID, req.Features); err != nil {
		http.Error(w, `{"error":"preset not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) CustomPreset(w http.ResponseWriter, r *http.Request) {
	pm := h.presets()
	if pm == nil {
		http.Error(w, `{"error":"presets not configured"}`, http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodPost:
		var req struct {
			ID          string          `json:"id"`
			Name        string          `json:"name"`
			Description string          `json:"description"`
			Tone        string          `json:"tone"`
			Style       string          `json:"style"`
			Greeting    string          `json:"greeting"`
			SystemNote  string          `json:"system_note"`
			Features    map[string]bool `json:"features,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" || req.Name == "" {
			http.Error(w, `{"error":"id and name required"}`, http.StatusBadRequest)
			return
		}
		pm.AddCustom(persona.Preset{
			ID:          req.ID,
			Name:        req.Name,
			Description: req.Description,
			Tone:        req.Tone,
			Style:       req.Style,
			Greeting:    req.Greeting,
			SystemNote:  req.SystemNote,
			Features:    req.Features,
		})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "id": req.ID})
	case http.MethodDelete:
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
			http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
			return
		}
		if err := pm.RemoveCustom(req.ID); err != nil {
			http.Error(w, `{"error":"not found or not custom"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
