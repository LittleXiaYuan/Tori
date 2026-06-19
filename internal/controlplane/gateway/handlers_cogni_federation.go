package gateway

import (
	"encoding/json"
	"net/http"

	"yunque-agent/internal/apperror"
)

func (g *Gateway) cogniFederationExpose(w http.ResponseWriter, r *http.Request, id string, expose bool) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	if g.cogniFederation == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "federation not configured")
		return
	}
	if expose {
		if err := g.cogniFederation.Expose(id); err != nil {
			apperror.WriteCode(w, apperror.CodeNotFound, err.Error())
			return
		}
	} else {
		g.cogniFederation.Unexpose(id)
	}
	action := "unexposed"
	if expose {
		action = "exposed"
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"status": action, "id": id})
}
