package gateway

import (
	"net/http"
	"strings"

	"yunque-agent/internal/apperror"
)

// handleCognis is the legacy Cogni Kernel adapter. Pack Runtime owns the
// method-aware /v1/cognis* route mounting and the cognikernel pack owns the
// business handlers. This adapter is intentionally kept as a defensive fallback
// for stale direct registrations and should not regain Cogni business logic.
func (g *Gateway) handleCognis(w http.ResponseWriter, r *http.Request) {
	if g.cogniRegistry == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "cogni registry not configured")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/v1/cognis")
	path = strings.TrimPrefix(path, "/")

	switch {
	case path == "":
		apperror.WriteCode(w, apperror.CodeNotFound, "cogni collection is owned by cogni-kernel pack")
	case path == "runtime/pack-state":
		if g.cogniKernelRuntimeState == nil {
			apperror.WriteCode(w, apperror.CodeInternal, "cogni runtime state reporter not configured")
			return
		}
		g.cogniKernelRuntimeState(w, r)
	case path == "reload":
		apperror.WriteCode(w, apperror.CodeNotFound, "cogni reload is owned by cogni-kernel pack")
	case path == "generate":
		apperror.WriteCode(w, apperror.CodeNotFound, "cogni generate is owned by cogni-kernel pack")
	case path == "export":
		apperror.WriteCode(w, apperror.CodeNotFound, "cogni export is owned by cogni-kernel pack")
	case path == "import":
		apperror.WriteCode(w, apperror.CodeNotFound, "cogni import is owned by cogni-kernel pack")
	case path == "evolution":
		apperror.WriteCode(w, apperror.CodeNotFound, "cogni evolution is owned by cogni-kernel pack")
	default:
		segs := strings.SplitN(path, "/", 3)
		switch {
		case len(segs) == 1:
			apperror.WriteCode(w, apperror.CodeNotFound, "cogni declaration route is owned by cogni-kernel pack")
		case len(segs) == 2 && segs[1] == "enable":
			apperror.WriteCode(w, apperror.CodeNotFound, "cogni enable route is owned by cogni-kernel pack")
		case len(segs) == 2 && segs[1] == "disable":
			apperror.WriteCode(w, apperror.CodeNotFound, "cogni disable route is owned by cogni-kernel pack")
		case len(segs) == 2 && segs[1] == "evolve":
			apperror.WriteCode(w, apperror.CodeNotFound, "cogni evolve route is owned by cogni-kernel pack")
		case len(segs) == 2 && segs[1] == "evolution":
			apperror.WriteCode(w, apperror.CodeNotFound, "cogni evolution route is owned by cogni-kernel pack")
		case len(segs) == 2 && segs[1] == "expose":
			apperror.WriteCode(w, apperror.CodeNotFound, "cogni federation expose route is owned by cogni-kernel pack")
		case len(segs) == 2 && segs[1] == "unexpose":
			apperror.WriteCode(w, apperror.CodeNotFound, "cogni federation unexpose route is owned by cogni-kernel pack")
		default:
			apperror.WriteCode(w, apperror.CodeNotFound, "unknown cogni sub-resource")
		}
	}
}

// ServeCogniKernel is the temporary Gateway adapter for the Cogni Kernel pack's
// API interface. Pack Runtime owns the public /v1/cognis* route mounting and
// gates; Gateway only supplies existing business operations until those handlers
// are extracted behind a standalone Cogni service in later reversible steps.
func (g *Gateway) ServeCogniKernel(w http.ResponseWriter, r *http.Request) {
	g.handleCognis(w, r)
}

// ── Experience handlers ──
