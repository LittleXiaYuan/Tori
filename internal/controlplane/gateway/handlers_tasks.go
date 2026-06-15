package gateway

import (
	"yunque-agent/internal/agentcore/task"
	"yunque-agent/internal/controlplane/gateway/workflowapi"
)

// TaskStore / TaskRunner expose the task subsystems to the work pack
// (internal/packs/work), which owns the de-shelled task surface natively.
func (g *Gateway) TaskStore() task.Store    { return g.taskStore }
func (g *Gateway) TaskRunner() *task.Runner { return g.taskRunner }

// GapAnalyzer / TemplateStore / WorkMemManager / ThreadManager expose the
// remaining task subsystems to the work pack (de-shell batch 3). May be nil.
func (g *Gateway) GapAnalyzer() *task.GapAnalyzer            { return g.gapAnalyzer }
func (g *Gateway) TemplateStore() *task.TemplateStore        { return g.templateStore }
func (g *Gateway) WorkMemManager() *task.WorkingMemoryManager { return g.workMemMgr }
func (g *Gateway) ThreadManager() *task.ThreadManager        { return g.threadMgr }

// WorkflowHandler exposes the workflow engine handler to the work pack, which
// mounts /v1/workflows* so workflow is part of the task platform. May be nil.
func (g *Gateway) WorkflowHandler() *workflowapi.Handler { return g.workflowAPIHandler }

// Task collection handlers (create/list/delete) were de-shelled into the work
// pack (internal/packs/work); they run natively there via TaskStore()/TenantOf().
//
// Task lifecycle handlers (run/cancel/pause/resume/restart) were de-shelled into
// the work pack (internal/packs/work); they run natively there via the
// TaskStore()/TaskRunner() accessors. Create/list and the rest stay here for now.

// All task-surface HTTP handlers (collection, lifecycle, gaps, working memory,
// templates, threads) were de-shelled into the work pack (internal/packs/work).
// This file now only exposes the typed accessors the pack reaches them through.
