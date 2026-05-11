package gateway

import (
	"encoding/json"
	"net/http"

	"yunque-agent/internal/agentcore/task"
	"yunque-agent/internal/apperror"
)

func (g *Gateway) handleSkills(w http.ResponseWriter, r *http.Request) {
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
	if g.metrics != nil {
		snap := g.metrics.Snapshot()
		for _, ss := range snap.Skills {
			usageMap[ss.Name] = struct {
				total       int64
				successRate float64
			}{total: ss.Total, successRate: ss.SuccessRate}
		}
	}

	out := make([]skillInfo, 0)
	cats := make([]catInfo, 0)
	if g.registry != nil {
		for _, s := range g.registry.All() {
			u := usageMap[s.Name()]
			out = append(out, skillInfo{
				Name:        s.Name(),
				Description: s.Description(),
				Parameters:  s.Parameters(),
				Category:    g.registry.CategoryOf(s.Name()),
				UsageTotal:  u.total,
				SuccessRate: u.successRate,
			})
		}
		for _, c := range g.registry.Categories() {
			cats = append(cats, catInfo{ID: c.ID, Name: c.Name, Description: c.Description})
		}
	}
	json.NewEncoder(w).Encode(map[string]any{"skills": out, "count": len(out), "categories": cats})
}

func (g *Gateway) handleSkillsDynamicGet(w http.ResponseWriter, r *http.Request) {
	if g.registry == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "skill registry not configured")
		return
	}
	allSkills := g.registry.All()
	var dynamic []task.DynamicSkillDef
	for _, sk := range allSkills {
		if ds, ok := sk.(*task.DynamicSkill); ok {
			dynamic = append(dynamic, ds.Def())
		}
	}
	if dynamic == nil {
		dynamic = []task.DynamicSkillDef{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"skills": dynamic})
}

func (g *Gateway) handleSkillsDynamicApprove(w http.ResponseWriter, r *http.Request) {
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
	sk, ok := g.registry.Get(req.Name)
	if !ok {
		apperror.WriteCode(w, apperror.CodeNotFound, "skill not found")
		return
	}
	if ds, ok := sk.(*task.DynamicSkill); ok {
		ds.SetApprovalStatus("approved")
		if req.Instruction != "" {
			ds.UpdateInstruction(req.Instruction)
		}
		if err := task.SaveDynamicSkills(g.registry, "data/dynamic_skills.json"); err != nil {
			apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "save dynamic skills", err))
			return
		}
	} else {
		apperror.WriteCode(w, apperror.CodeInvalidField, "not a dynamic skill")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (g *Gateway) handleSkillsDynamicReject(w http.ResponseWriter, r *http.Request) {
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
	sk, ok := g.registry.Get(req.Name)
	if !ok {
		apperror.WriteCode(w, apperror.CodeNotFound, "skill not found")
		return
	}
	if _, ok := sk.(*task.DynamicSkill); !ok {
		apperror.WriteCode(w, apperror.CodeInvalidField, "not a dynamic skill")
		return
	}
	g.registry.Remove(req.Name)
	if err := task.SaveDynamicSkills(g.registry, "data/dynamic_skills.json"); err != nil {
		apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "save dynamic skills", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
