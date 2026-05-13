package cognikernelpack

import (
	"net/http"

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

// Handler exposes CogniKernel/Cognis management as a Pack Runtime backend
// module. The business logic still lives in Gateway during this bridge phase;
// route ownership, enablement and method gates now belong to Pack Runtime.
type Handler struct {
	gateway CogniGateway
}

func NewHandler(gateway CogniGateway) *Handler {
	return &Handler{gateway: gateway}
}

func (h *Handler) PackID() string { return PackID }

func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Methods: []string{http.MethodGet, http.MethodPost}, Path: "/v1/cognis", Handler: h.gateway.HandleCogniKernelPack},
		{Methods: []string{http.MethodGet, http.MethodPost, http.MethodDelete}, Path: "/v1/cognis/", Handler: h.gateway.HandleCogniKernelPack},
	}
}
