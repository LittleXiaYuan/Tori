package gateway

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// This file owns the planner resume-job store: the in-memory map guarded by
// g.plannerResumeJobsMu plus its append-only JSONL persistence. It was split
// out of handlers_planner.go to keep the HTTP handlers focused on request
// handling rather than storage mechanics. All functions here operate on
// Gateway fields declared in gateway.go (plannerResumeJobs*, …).

func (g *Gateway) SetPlannerResumeJobStore(path string) {
	g.plannerResumeJobsMu.Lock()
	g.plannerResumeJobsPath = strings.TrimSpace(path)
	g.plannerResumeJobsMu.Unlock()
	if err := g.loadPlannerResumeJobs(); err != nil {
		slog.Warn("planner resume job store load failed", "err", err)
	}
}

func (g *Gateway) savePlannerResumeJob(job plannerCheckpointResumePlanJob) {
	g.plannerResumeJobsMu.Lock()
	if g.plannerResumeJobs == nil {
		g.plannerResumeJobs = make(map[string]plannerCheckpointResumePlanJob)
	}
	g.plannerResumeJobs[job.ID] = job
	path := g.plannerResumeJobsPath
	g.plannerResumeJobsMu.Unlock()
	if path != "" {
		if err := appendPlannerResumeJob(path, job); err != nil {
			slog.Warn("planner resume job store append failed", "job", job.ID, "err", err)
		}
	}
}

func (g *Gateway) reservePlannerResumeJob(planID, sessionID, taskID, tenantID, action string) (plannerCheckpointResumePlanJob, bool) {
	planID = strings.TrimSpace(planID)
	sessionID = strings.TrimSpace(sessionID)
	tenantID = strings.TrimSpace(tenantID)
	action = normalizeCheckpointAction(action)
	if err := g.loadPlannerResumeJobs(); err != nil {
		slog.Warn("planner resume job store reload failed", "err", err)
	}
	now := time.Now().UTC()
	job := plannerCheckpointResumePlanJob{
		ID:        fmt.Sprintf("resume-plan-%d", now.UnixNano()),
		Status:    "running",
		Action:    action,
		TenantID:  tenantID,
		PlanID:    planID,
		SessionID: sessionID,
		TaskID:    strings.TrimSpace(taskID),
		StartedAt: now.Format(time.RFC3339),
	}
	g.plannerResumeJobsMu.Lock()
	if g.plannerResumeJobs == nil {
		g.plannerResumeJobs = make(map[string]plannerCheckpointResumePlanJob)
	}
	for _, existing := range g.plannerResumeJobs {
		if plannerResumeJobTenantMatches(existing, tenantID) && existing.PlanID == planID && normalizeCheckpointAction(existing.Action) == action && existing.Status == "running" {
			g.plannerResumeJobsMu.Unlock()
			return existing, true
		}
	}
	g.plannerResumeJobs[job.ID] = job
	path := g.plannerResumeJobsPath
	g.plannerResumeJobsMu.Unlock()
	if path != "" {
		if err := appendPlannerResumeJob(path, job); err != nil {
			slog.Warn("planner resume job store append failed", "job", job.ID, "err", err)
		}
	}
	return job, false
}

func (g *Gateway) getPlannerResumeJob(id, tenantID string) (plannerCheckpointResumePlanJob, bool) {
	tenantID = strings.TrimSpace(tenantID)
	g.plannerResumeJobsMu.Lock()
	if g.plannerResumeJobs != nil {
		job, ok := g.plannerResumeJobs[id]
		g.plannerResumeJobsMu.Unlock()
		return job, ok && plannerResumeJobTenantMatches(job, tenantID)
	}
	g.plannerResumeJobsMu.Unlock()
	if err := g.loadPlannerResumeJobs(); err != nil {
		slog.Warn("planner resume job store reload failed", "err", err)
	}
	g.plannerResumeJobsMu.Lock()
	defer g.plannerResumeJobsMu.Unlock()
	if g.plannerResumeJobs == nil {
		return plannerCheckpointResumePlanJob{}, false
	}
	job, ok := g.plannerResumeJobs[id]
	return job, ok && plannerResumeJobTenantMatches(job, tenantID)
}

func (g *Gateway) getLatestPlannerResumeJobForPlan(planID, tenantID string) (plannerCheckpointResumePlanJob, bool) {
	planID = strings.TrimSpace(planID)
	tenantID = strings.TrimSpace(tenantID)
	if planID == "" {
		return plannerCheckpointResumePlanJob{}, false
	}
	if err := g.loadPlannerResumeJobs(); err != nil {
		slog.Warn("planner resume job store reload failed", "err", err)
	}
	g.plannerResumeJobsMu.Lock()
	defer g.plannerResumeJobsMu.Unlock()
	var latest plannerCheckpointResumePlanJob
	ok := false
	for _, job := range g.plannerResumeJobs {
		if job.PlanID != planID || !plannerResumeJobTenantMatches(job, tenantID) {
			continue
		}
		if !ok || job.StartedAt > latest.StartedAt || (job.StartedAt == latest.StartedAt && job.ID > latest.ID) {
			latest = job
			ok = true
		}
	}
	return latest, ok
}

func plannerResumeJobTenantMatches(job plannerCheckpointResumePlanJob, tenantID string) bool {
	return strings.TrimSpace(job.TenantID) == strings.TrimSpace(tenantID)
}

func (g *Gateway) loadPlannerResumeJobs() error {
	g.plannerResumeJobsMu.Lock()
	path := g.plannerResumeJobsPath
	g.plannerResumeJobsMu.Unlock()
	if path == "" {
		return nil
	}
	jobs, err := readPlannerResumeJobs(path)
	if err != nil {
		return err
	}
	g.plannerResumeJobsMu.Lock()
	defer g.plannerResumeJobsMu.Unlock()
	if g.plannerResumeJobs == nil {
		g.plannerResumeJobs = make(map[string]plannerCheckpointResumePlanJob, len(jobs))
	}
	for id, job := range jobs {
		g.plannerResumeJobs[id] = job
	}
	return nil
}

func appendPlannerResumeJob(path string, job plannerCheckpointResumePlanJob) error {
	if strings.TrimSpace(path) == "" || strings.TrimSpace(job.ID) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(data, '\n'))
	return err
}

func readPlannerResumeJobs(path string) (map[string]plannerCheckpointResumePlanJob, error) {
	out := make(map[string]plannerCheckpointResumePlanJob)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var job plannerCheckpointResumePlanJob
		if err := json.Unmarshal(scanner.Bytes(), &job); err == nil && strings.TrimSpace(job.ID) != "" {
			out[job.ID] = job
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
