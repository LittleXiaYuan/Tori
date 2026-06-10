package lora

import (
	"net/http"

	"yunque-agent/internal/agentcore/localbrain"
	"yunque-agent/internal/controlplane/gateway/loraapi"
	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.lora"

// Handler exposes the LoRA/LAA evolution surface as an optional backend pack.
// The implementation delegates to the existing LoRA API handler, but Gateway
// mounting, enablement and method gates are now owned by Pack Runtime.
type Handler struct {
	api *loraapi.Handler
}

type Options struct {
	Scheduler *localbrain.LoRAScheduler
	Metrics   *localbrain.TrainingMetrics
	Evolution *localbrain.EvolutionCoordinator
	Distill   *localbrain.SelfDistillPipeline
}

func NewHandler(opts Options) *Handler {
	return &Handler{api: &loraapi.Handler{
		Scheduler: opts.Scheduler,
		Metrics:   opts.Metrics,
		Evolution: opts.Evolution,
		Distill:   opts.Distill,
	}}
}

func (h *Handler) PackID() string { return PackID }

func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Method: http.MethodGet, Path: "/v1/lora/status", Handler: h.api.HandleStatus},
		{Method: http.MethodGet, Path: "/v1/lora/history", Handler: h.api.HandleHistory},
		{Method: http.MethodGet, Path: "/v1/lora/summary", Handler: h.api.HandleSummary},
		{Method: http.MethodGet, Path: "/v1/lora/preview", Handler: h.api.HandlePreview},
		{Method: http.MethodPost, Path: "/v1/lora/trigger", Handler: h.api.HandleTrigger},
		{Method: http.MethodPost, Path: "/v1/lora/rollback", Handler: h.api.HandleRollback},
		{Method: http.MethodGet, Path: "/v1/lora/evolution", Handler: h.api.HandleEvolution},
		{Methods: []string{http.MethodGet, http.MethodPut, http.MethodPatch}, Path: "/v1/lora/config", Handler: h.api.HandleConfig},
		{Methods: []string{http.MethodGet, http.MethodPost}, Path: "/v1/lora/distill", Handler: h.api.HandleDistill},
	}
}
