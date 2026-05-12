// Package yunque is the official Go SDK for writing Yunque Agent plugins.
//
// It provides access to all agent capabilities through a simple API,
// enabling plugins to consume AND extend the agent's functionality.
//
// # Quick Start
//
//	package main
//
//	import "yunque-agent/sdk/go/yunque"
//
//	func main() {
//	    yunque.RegisterSkill("my_tool", "Does something", myHandler)
//	    yunque.Run()
//	}
//
//	func myHandler(ctx context.Context, args map[string]any) (string, error) {
//	    reply, _ := yunque.LLM(ctx, "You are helpful", args["input"].(string))
//	    return reply, nil
//	}
//
// # Environment Variables
//
// The agent runtime injects these when launching plugin processes:
//
//	YUNQUE_API_BASE      - Agent API base URL (default: http://localhost:9090)
//	YUNQUE_PLUGIN_TOKEN  - Plugin-scoped API token (permissions limited by manifest)
//	YUNQUE_PLUGIN_NAME   - Plugin identifier
//	YUNQUE_PLUGIN_DIR    - Plugin directory path
package yunque

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	apiBase     = envOr("YUNQUE_API_BASE", "http://localhost:9090")
	pluginToken = os.Getenv("YUNQUE_PLUGIN_TOKEN")
	pluginName  = envOr("YUNQUE_PLUGIN_NAME", os.Getenv("PLUGIN_NAME"))
	pluginDir   = envOr("YUNQUE_PLUGIN_DIR", os.Getenv("PLUGIN_DIR"))
	httpClient  = &http.Client{Timeout: 30 * time.Second}
)

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// PluginName returns the plugin identifier.
func PluginName() string { return pluginName }

// PluginDir returns the plugin directory path.
func PluginDir() string { return pluginDir }

// ── API Call ──

func apiCall(ctx context.Context, method, path string, body any) (map[string]any, error) {
	var result map[string]any
	if err := apiCallInto(ctx, method, path, body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func apiCallInto(ctx context.Context, method, path string, body any, target any) error {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, apiBase+path, reqBody)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if pluginToken != "" {
		req.Header.Set("Authorization", "Bearer "+pluginToken)
	}
	req.Header.Set("X-Plugin-Name", pluginName)

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("api call %s: %w", path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("api %s HTTP %d: %s", path, resp.StatusCode, apiErrorMessage(respBody))
	}

	if err := json.Unmarshal(respBody, target); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func apiErrorMessage(body []byte) string {
	text := strings.TrimSpace(string(body))
	if text == "" {
		return "request failed"
	}
	var parsed any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return text
	}
	if msg := errorMessageFromJSON(parsed); msg != "" {
		return msg
	}
	return text
}

func errorMessageFromJSON(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case map[string]any:
		for _, key := range []string{"message", "detail", "reason"} {
			if msg, ok := v[key].(string); ok && strings.TrimSpace(msg) != "" {
				return strings.TrimSpace(msg)
			}
		}
		if nested, ok := v["error"]; ok {
			if msg := errorMessageFromJSON(nested); msg != "" {
				if m, ok := nested.(map[string]any); ok {
					if code, ok := m["code"].(string); ok && strings.TrimSpace(code) != "" && !strings.HasPrefix(msg, code+":") {
						return strings.TrimSpace(code) + ": " + msg
					}
				}
				return msg
			}
		}
	}
	return ""
}

// ── Reflection Experience ──

// Reflect provides access to the lightweight reflection experience API.
var Reflect = &reflectNamespace{}

type reflectNamespace struct{}

// ReflectExperienceOptions filters reflection experiences.
type ReflectExperienceOptions struct {
	Query    string
	Source   string
	Category string
	Outcome  string
	Tag      string
	Limit    int
}

// ReflectStrategyOptions filters compiled reflection strategies.
type ReflectStrategyOptions = ReflectExperienceOptions

// ReflectExperience is a structured reflection lesson captured by the agent.
type ReflectExperience struct {
	ID        string    `json:"id,omitempty"`
	Source    string    `json:"source,omitempty"`
	SourceID  string    `json:"source_id,omitempty"`
	Category  string    `json:"category,omitempty"`
	Outcome   string    `json:"outcome,omitempty"`
	Lesson    string    `json:"lesson,omitempty"`
	Context   string    `json:"context,omitempty"`
	Tags      []string  `json:"tags,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

// ReflectExperiencesResponse is returned by /v1/reflect/experiences.
type ReflectExperiencesResponse struct {
	Experiences []ReflectExperience `json:"experiences"`
	Total       int                 `json:"total"`
}

// ReflectExperienceStats summarizes reflection experiences.
type ReflectExperienceStats struct {
	Total      int            `json:"total"`
	BySource   map[string]int `json:"by_source"`
	ByCategory map[string]int `json:"by_category"`
	ByOutcome  map[string]int `json:"by_outcome"`
	Recent7D   int            `json:"recent_7d"`
}

// Experiences lists reflection experiences with optional source/category/outcome/tag/query filters.
func (r *reflectNamespace) Experiences(ctx context.Context, opts ReflectExperienceOptions) (ReflectExperiencesResponse, error) {
	resp, err := apiCall(ctx, "GET", "/v1/reflect/experiences"+reflectExperienceQuery(opts, false), nil)
	if err != nil {
		return ReflectExperiencesResponse{}, err
	}
	var out ReflectExperiencesResponse
	if err := decodeMapResponse(resp, &out); err != nil {
		return ReflectExperiencesResponse{}, err
	}
	return out, nil
}

// Stats returns reflection experience counters using the same filters as Experiences.
func (r *reflectNamespace) Stats(ctx context.Context, opts ReflectExperienceOptions) (ReflectExperienceStats, error) {
	resp, err := apiCall(ctx, "GET", "/v1/reflect/experiences"+reflectExperienceQuery(opts, true), nil)
	if err != nil {
		return ReflectExperienceStats{}, err
	}
	var out ReflectExperienceStats
	if err := decodeMapResponse(resp, &out); err != nil {
		return ReflectExperienceStats{}, err
	}
	return out, nil
}

// Strategies returns compiled reflection strategy hints. Limit defaults to the server setting.
func (r *reflectNamespace) Strategies(ctx context.Context, limit int) (string, error) {
	return r.StrategiesWithOptions(ctx, ReflectStrategyOptions{Limit: limit})
}

// StrategiesWithOptions returns compiled strategy hints scoped by optional experience filters.
func (r *reflectNamespace) StrategiesWithOptions(ctx context.Context, opts ReflectStrategyOptions) (string, error) {
	resp, err := apiCall(ctx, "GET", "/v1/reflect/strategies"+reflectExperienceQuery(ReflectExperienceOptions(opts), false), nil)
	if err != nil {
		return "", err
	}
	strategies, _ := resp["strategies"].(string)
	return strategies, nil
}

func reflectExperienceQuery(opts ReflectExperienceOptions, stats bool) string {
	q := url.Values{}
	if opts.Query != "" {
		q.Set("q", opts.Query)
	}
	if opts.Source != "" {
		q.Set("source", opts.Source)
	}
	if opts.Category != "" {
		q.Set("category", opts.Category)
	}
	if opts.Outcome != "" {
		q.Set("outcome", opts.Outcome)
	}
	if opts.Tag != "" {
		q.Set("tag", opts.Tag)
	}
	if opts.Limit > 0 {
		q.Set("limit", strconv.Itoa(opts.Limit))
	}
	if stats {
		q.Set("stats", "true")
	}
	if len(q) == 0 {
		return ""
	}
	return "?" + q.Encode()
}

func decodeMapResponse(resp map[string]any, target any) error {
	raw, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}

// ── Agent Kit bundle ──

// AgentKit groups the common SDK-first surfaces for external Go sidecars, CLIs,
// plugins, and automation binaries.
//
// It is intentionally just a thin bundle over the existing lightweight
// namespaces, so callers can reach State Kernel, Reflection Experience,
// Mission Parse, Scheduler, Triggers, and Plugin API Runtime helpers without linking to
// platform internals or a broad generated client.
type AgentKit struct {
	State       *stateNamespace
	Reflect     *reflectNamespace
	Missions    *missionsNamespace
	Scheduler   *schedulerNamespace
	CronSystem  *cronSystemNamespace
	Triggers    *triggersNamespace
	MemoryCore  *memoryCoreNamespace
	Plugin      *pluginRuntimeNamespace
	Memory      *memoryNamespace
	AgentMemory *agentMemoryNamespace
	Knowledge   *knowledgeNamespace
	Cron        *cronNamespace
}

// Plugin groups top-level Plugin API Runtime helpers under one namespace for
// AgentKit-style callers.
var Plugin = &pluginRuntimeNamespace{}

type pluginRuntimeNamespace struct{}

// ── Mission Parse ──

// Missions provides focused access to natural-language mission parsing.
var Missions = &missionsNamespace{}

type missionsNamespace struct{}

// MissionParseResult is the structured task/workflow/cron/trigger draft
// returned by /v1/missions/parse.
type MissionParseResult struct {
	Type        string         `json:"type,omitempty"`
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Config      map[string]any `json:"config,omitempty"`
	Confidence  float64        `json:"confidence,omitempty"`
	Explanation string         `json:"explanation,omitempty"`
}

// Parse turns a natural-language mission description into a structured draft.
func (m *missionsNamespace) Parse(ctx context.Context, description string) (MissionParseResult, error) {
	var out MissionParseResult
	if err := apiCallInto(ctx, http.MethodPost, "/v1/missions/parse", map[string]any{"description": description}, &out); err != nil {
		return MissionParseResult{}, err
	}
	if out.Config == nil {
		out.Config = map[string]any{}
	}
	return out, nil
}

// ── Trigger Automation ──

// Triggers provides focused access to Triggers v2 automation definitions,
// event emission, and recent trigger history.
var Triggers = &triggersNamespace{}

type triggersNamespace struct{}

// TriggerDef is a Triggers v2 automation definition.
type TriggerDef struct {
	ID       string         `json:"id,omitempty"`
	Name     string         `json:"name,omitempty"`
	TenantID string         `json:"tenant_id,omitempty"`
	Type     string         `json:"type,omitempty"`
	Status   string         `json:"status,omitempty"`
	Actions  []any          `json:"actions,omitempty"`
	Extra    map[string]any `json:"-"`
}

// TriggerListOptions filters Triggers v2 definitions.
type TriggerListOptions struct {
	TenantID string
	Type     string
	Status   string
}

// TriggerListResponse is returned by /v1/triggers/v2.
type TriggerListResponse struct {
	Triggers []TriggerDef `json:"triggers"`
	Total    int          `json:"total"`
}

// TriggerPayload is accepted by /v1/triggers/v2/emit.
type TriggerPayload struct {
	Event     string         `json:"event"`
	Text      string         `json:"text,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
	Timestamp time.Time      `json:"timestamp,omitempty"`
}

// TriggerEmitResponse is returned by /v1/triggers/v2/emit.
type TriggerEmitResponse struct {
	Status string `json:"status"`
	Event  string `json:"event"`
}

// TriggerDeleteResponse is returned by DELETE /v1/triggers/v2?id=...
type TriggerDeleteResponse struct {
	Deleted string `json:"deleted"`
}

// TriggerHistoryOptions filters trigger runs and events.
type TriggerHistoryOptions struct {
	TriggerID string
	Limit     int
}

// TriggerRunsResponse is returned by /v1/triggers/v2/runs.
type TriggerRunsResponse struct {
	Runs  []map[string]any `json:"runs"`
	Total int              `json:"total"`
}

// TriggerEventsResponse is returned by /v1/triggers/v2/events.
type TriggerEventsResponse struct {
	Events []map[string]any `json:"events"`
	Total  int              `json:"total"`
}

// List returns Triggers v2 definitions with optional tenant/type/status filters.
func (t *triggersNamespace) List(ctx context.Context, opts TriggerListOptions) (TriggerListResponse, error) {
	var out TriggerListResponse
	if err := apiCallInto(ctx, http.MethodGet, "/v1/triggers/v2"+triggerListQuery(opts), nil, &out); err != nil {
		return TriggerListResponse{}, err
	}
	if out.Triggers == nil {
		out.Triggers = []TriggerDef{}
	}
	return out, nil
}

// Get returns one Triggers v2 definition by id.
func (t *triggersNamespace) Get(ctx context.Context, id string) (TriggerDef, error) {
	var out TriggerDef
	if err := apiCallInto(ctx, http.MethodGet, "/v1/triggers/v2?id="+url.QueryEscape(id), nil, &out); err != nil {
		return TriggerDef{}, err
	}
	return out, nil
}

// Create creates a Triggers v2 definition.
func (t *triggersNamespace) Create(ctx context.Context, def TriggerDef) (TriggerDef, error) {
	var out TriggerDef
	if err := apiCallInto(ctx, http.MethodPost, "/v1/triggers/v2", def, &out); err != nil {
		return TriggerDef{}, err
	}
	return out, nil
}

// Update updates a Triggers v2 definition.
func (t *triggersNamespace) Update(ctx context.Context, def TriggerDef) (TriggerDef, error) {
	var out TriggerDef
	if err := apiCallInto(ctx, http.MethodPut, "/v1/triggers/v2", def, &out); err != nil {
		return TriggerDef{}, err
	}
	return out, nil
}

// Delete removes a Triggers v2 definition by id.
func (t *triggersNamespace) Delete(ctx context.Context, id string) (TriggerDeleteResponse, error) {
	var out TriggerDeleteResponse
	if err := apiCallInto(ctx, http.MethodDelete, "/v1/triggers/v2?id="+url.QueryEscape(id), nil, &out); err != nil {
		return TriggerDeleteResponse{}, err
	}
	return out, nil
}

// Emit sends an event to the Triggers v2 automation runtime.
func (t *triggersNamespace) Emit(ctx context.Context, payload TriggerPayload) (TriggerEmitResponse, error) {
	var out TriggerEmitResponse
	if err := apiCallInto(ctx, http.MethodPost, "/v1/triggers/v2/emit", payload, &out); err != nil {
		return TriggerEmitResponse{}, err
	}
	return out, nil
}

// Runs lists recent trigger runs.
func (t *triggersNamespace) Runs(ctx context.Context, opts TriggerHistoryOptions) (TriggerRunsResponse, error) {
	var out TriggerRunsResponse
	if err := apiCallInto(ctx, http.MethodGet, "/v1/triggers/v2/runs"+triggerHistoryQuery(opts), nil, &out); err != nil {
		return TriggerRunsResponse{}, err
	}
	if out.Runs == nil {
		out.Runs = []map[string]any{}
	}
	return out, nil
}

// Events lists recent trigger events.
func (t *triggersNamespace) Events(ctx context.Context, opts TriggerHistoryOptions) (TriggerEventsResponse, error) {
	var out TriggerEventsResponse
	if err := apiCallInto(ctx, http.MethodGet, "/v1/triggers/v2/events"+triggerHistoryQuery(opts), nil, &out); err != nil {
		return TriggerEventsResponse{}, err
	}
	if out.Events == nil {
		out.Events = []map[string]any{}
	}
	return out, nil
}

func triggerListQuery(opts TriggerListOptions) string {
	q := url.Values{}
	if opts.TenantID != "" {
		q.Set("tenant_id", opts.TenantID)
	}
	if opts.Type != "" {
		q.Set("type", opts.Type)
	}
	if opts.Status != "" {
		q.Set("status", opts.Status)
	}
	if len(q) == 0 {
		return ""
	}
	return "?" + q.Encode()
}

func triggerHistoryQuery(opts TriggerHistoryOptions) string {
	q := url.Values{}
	if opts.TriggerID != "" {
		q.Set("trigger_id", opts.TriggerID)
	}
	if opts.Limit > 0 {
		q.Set("limit", strconv.Itoa(opts.Limit))
	}
	if len(q) == 0 {
		return ""
	}
	return "?" + q.Encode()
}

// ── Cron System ──

// CronSystem provides focused access to the host /v1/cron/* scheduled task API.
// It is separate from Cron, which targets plugin-owned /v1/plugin-api/cron/* jobs.
var CronSystem = &cronSystemNamespace{}

type cronSystemNamespace struct{}

type CronSchedule struct {
	Type     string     `json:"type"`
	At       *time.Time `json:"at,omitempty"`
	EveryMs  int64      `json:"every_ms,omitempty"`
	CronExpr string     `json:"cron_expr,omitempty"`
	Timezone string     `json:"timezone,omitempty"`
}

type CronPayload struct {
	Kind    string         `json:"kind"`
	Message string         `json:"message,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
}

type CronJob struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	Schedule      CronSchedule `json:"schedule"`
	Payload       CronPayload  `json:"payload"`
	AgentID       string       `json:"agent_id,omitempty"`
	SessionTarget string       `json:"session_target,omitempty"`
	Delivery      string       `json:"delivery,omitempty"`
	Enabled       bool         `json:"enabled"`
	CreatedAt     time.Time    `json:"created_at"`
	LastRunAt     *time.Time   `json:"last_run_at,omitempty"`
	NextRunAt     *time.Time   `json:"next_run_at,omitempty"`
	RunCount      int          `json:"run_count"`
}

type CronRunRecord struct {
	JobID     string    `json:"job_id"`
	RunID     string    `json:"run_id"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
	Status    string    `json:"status"`
	Output    string    `json:"output,omitempty"`
	Error     string    `json:"error,omitempty"`
}

type CronListResponse struct {
	Jobs []CronJob `json:"jobs"`
}
type CronAddRequest struct {
	Name     string       `json:"name"`
	Schedule CronSchedule `json:"schedule"`
	Payload  CronPayload  `json:"payload"`
}
type CronAddResponse struct {
	Job CronJob `json:"job"`
}
type CronRemoveResponse struct {
	Deleted string `json:"deleted"`
}
type CronRunResponse struct {
	Run CronRunRecord `json:"run"`
}

func (c *cronSystemNamespace) List(ctx context.Context) (CronListResponse, error) {
	var out CronListResponse
	if err := apiCallInto(ctx, http.MethodGet, "/v1/cron/list", nil, &out); err != nil {
		return CronListResponse{}, err
	}
	if out.Jobs == nil {
		out.Jobs = []CronJob{}
	}
	return out, nil
}

func (c *cronSystemNamespace) Add(ctx context.Context, req CronAddRequest) (CronAddResponse, error) {
	var out CronAddResponse
	if err := apiCallInto(ctx, http.MethodPost, "/v1/cron/add", req, &out); err != nil {
		return CronAddResponse{}, err
	}
	return out, nil
}

func (c *cronSystemNamespace) Remove(ctx context.Context, id string) (CronRemoveResponse, error) {
	var out CronRemoveResponse
	if err := apiCallInto(ctx, http.MethodPost, "/v1/cron/remove?id="+url.QueryEscape(id), nil, &out); err != nil {
		return CronRemoveResponse{}, err
	}
	return out, nil
}

func (c *cronSystemNamespace) Run(ctx context.Context, id string) (CronRunResponse, error) {
	var out CronRunResponse
	if err := apiCallInto(ctx, http.MethodPost, "/v1/cron/run?id="+url.QueryEscape(id), nil, &out); err != nil {
		return CronRunResponse{}, err
	}
	return out, nil
}

// ── Memory Kernel ──

// MemoryCore provides focused access to the host /v1/memory/* recall layer.
// It is separate from Memory, which targets plugin-private /v1/plugin-api/memory/* KV.
var MemoryCore = &memoryCoreNamespace{}

type memoryCoreNamespace struct{}

type MemoryItem struct {
	Key     string         `json:"key,omitempty"`
	Value   string         `json:"value,omitempty"`
	Content string         `json:"content,omitempty"`
	Source  string         `json:"source,omitempty"`
	Layer   string         `json:"layer,omitempty"`
	Score   float64        `json:"score,omitempty"`
	Tags    []string       `json:"tags,omitempty"`
	Extra   map[string]any `json:"-"`
}

type MemoryStatsResponse map[string]any

type MemorySearchRequest struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
	Layer string `json:"layer,omitempty"`
}

type MemorySearchResponse struct {
	Results []MemoryItem `json:"results"`
	Count   int          `json:"count"`
}

type MemoryAddRequest struct {
	Key     string   `json:"key,omitempty"`
	Value   string   `json:"value,omitempty"`
	Content string   `json:"content,omitempty"`
	Layer   string   `json:"layer,omitempty"`
	Source  string   `json:"source,omitempty"`
	Tags    []string `json:"tags,omitempty"`
}

type MemoryAddResponse struct {
	Status string `json:"status"`
}

type MemoryCompactRequest struct {
	TargetCount int `json:"target_count,omitempty"`
	DecayDays   int `json:"decay_days,omitempty"`
}

type MemoryCompactResponse map[string]any

func (m *memoryCoreNamespace) Stats(ctx context.Context) (MemoryStatsResponse, error) {
	var out MemoryStatsResponse
	if err := apiCallInto(ctx, http.MethodGet, "/v1/memory/stats", nil, &out); err != nil {
		return nil, err
	}
	if out == nil {
		return MemoryStatsResponse{}, nil
	}
	return out, nil
}

func (m *memoryCoreNamespace) Search(ctx context.Context, req MemorySearchRequest) (MemorySearchResponse, error) {
	var out MemorySearchResponse
	if err := apiCallInto(ctx, http.MethodPost, "/v1/memory/search", req, &out); err != nil {
		return MemorySearchResponse{}, err
	}
	if out.Results == nil {
		out.Results = []MemoryItem{}
	}
	return out, nil
}

func (m *memoryCoreNamespace) Add(ctx context.Context, req MemoryAddRequest) (MemoryAddResponse, error) {
	if req.Value == "" {
		req.Value = req.Content
	}
	var out MemoryAddResponse
	if err := apiCallInto(ctx, http.MethodPost, "/v1/memory/add", req, &out); err != nil {
		return MemoryAddResponse{}, err
	}
	return out, nil
}

func (m *memoryCoreNamespace) Compact(ctx context.Context, req MemoryCompactRequest) (MemoryCompactResponse, error) {
	var out MemoryCompactResponse
	if err := apiCallInto(ctx, http.MethodPost, "/v1/memory/compact", req, &out); err != nil {
		return nil, err
	}
	if out == nil {
		return MemoryCompactResponse{}, nil
	}
	return out, nil
}

// ── Prompt Scheduler ──

// Scheduler provides focused access to prompt-based recurring jobs.
var Scheduler = &schedulerNamespace{}

type schedulerNamespace struct{}

// SchedulerJob is a prompt job managed by /v1/scheduler/*.
type SchedulerJob struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	TenantID string `json:"tenant_id,omitempty"`
	Interval any    `json:"interval,omitempty"`
	Prompt   string `json:"prompt,omitempty"`
}

// SchedulerJobsResponse is returned by /v1/scheduler/jobs.
type SchedulerJobsResponse struct {
	Jobs  []SchedulerJob `json:"jobs"`
	Count int            `json:"count"`
}

// SchedulerRemoveResponse is returned by /v1/scheduler/remove.
type SchedulerRemoveResponse struct {
	Status string `json:"status"`
}

// Jobs lists prompt scheduler jobs.
func (s *schedulerNamespace) Jobs(ctx context.Context) (SchedulerJobsResponse, error) {
	var out SchedulerJobsResponse
	if err := apiCallInto(ctx, http.MethodGet, "/v1/scheduler/jobs", nil, &out); err != nil {
		return SchedulerJobsResponse{}, err
	}
	if out.Jobs == nil {
		out.Jobs = []SchedulerJob{}
	}
	return out, nil
}

// Add creates a recurring prompt scheduler job. Interval uses Go duration strings such as "1h".
func (s *schedulerNamespace) Add(ctx context.Context, name, prompt, interval string) (SchedulerJob, error) {
	var out SchedulerJob
	if err := apiCallInto(ctx, http.MethodPost, "/v1/scheduler/add", map[string]any{
		"name": name, "prompt": prompt, "interval": interval,
	}, &out); err != nil {
		return SchedulerJob{}, err
	}
	return out, nil
}

// Remove deletes a prompt scheduler job by id.
func (s *schedulerNamespace) Remove(ctx context.Context, id string) (SchedulerRemoveResponse, error) {
	var out SchedulerRemoveResponse
	if err := apiCallInto(ctx, http.MethodPost, "/v1/scheduler/remove", map[string]any{"id": id}, &out); err != nil {
		return SchedulerRemoveResponse{}, err
	}
	return out, nil
}

// NewAgentKit returns a lightweight bundle of state, reflection, mission parse,
// scheduler, cron, triggers, and plugin runtime helpers.
func NewAgentKit() AgentKit {
	return AgentKit{
		State:       State,
		Reflect:     Reflect,
		Missions:    Missions,
		Scheduler:   Scheduler,
		CronSystem:  CronSystem,
		Triggers:    Triggers,
		MemoryCore:  MemoryCore,
		Plugin:      Plugin,
		Memory:      Memory,
		AgentMemory: AgentMemory,
		Knowledge:   Knowledge,
		Cron:        Cron,
	}
}

func (p *pluginRuntimeNamespace) LLM(ctx context.Context, systemPrompt, userInput string) (string, error) {
	return LLM(ctx, systemPrompt, userInput)
}

func (p *pluginRuntimeNamespace) Chat(ctx context.Context, messages []Message, temperature float64) (string, error) {
	return Chat(ctx, messages, temperature)
}

func (p *pluginRuntimeNamespace) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	return Search(ctx, query, limit)
}

func (p *pluginRuntimeNamespace) Send(ctx context.Context, channelType, target, content string) error {
	return Send(ctx, channelType, target, content)
}

func (p *pluginRuntimeNamespace) RegisterProvider(ctx context.Context, id, baseURL, model string, opts ...ProviderOpt) error {
	return RegisterProvider(ctx, id, baseURL, model, opts...)
}

func (p *pluginRuntimeNamespace) RegisterChannel(ctx context.Context, name, webhookURL, sendEndpoint string) error {
	return RegisterChannel(ctx, name, webhookURL, sendEndpoint)
}

func (p *pluginRuntimeNamespace) RegisterSearchEngine(ctx context.Context, name, baseURL, apiKey string) error {
	return RegisterSearchEngine(ctx, name, baseURL, apiKey)
}

func (p *pluginRuntimeNamespace) RegisterGuardrail(ctx context.Context, name, description, phase string, keywords []string) error {
	return RegisterGuardrail(ctx, name, description, phase, keywords)
}

func (p *pluginRuntimeNamespace) RegisterEmbedding(ctx context.Context, name, baseURL, model string, dimensions int) error {
	return RegisterEmbedding(ctx, name, baseURL, model, dimensions)
}

func (p *pluginRuntimeNamespace) RegisterSpeech(ctx context.Context, name, speechType, baseURL, model string) error {
	return RegisterSpeech(ctx, name, speechType, baseURL, model)
}

// ── State Kernel ──

// State provides typed access to the lightweight State Kernel snapshot API.
//
// These helpers are intentionally small so plugins, CLIs, and sidecar services can
// consume the agent's current goals/resources/focus/capabilities without importing
// the full platform surface.
var State = &stateNamespace{}

type stateNamespace struct{}

// StateGoal is a goal tracked by the State Kernel.
type StateGoal struct {
	ID          string    `json:"id,omitempty"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Priority    int       `json:"priority,omitempty"`
	Status      string    `json:"status,omitempty"`
	Progress    float64   `json:"progress,omitempty"`
	ParentGoal  string    `json:"parent_goal,omitempty"`
	SubGoals    []string  `json:"sub_goals,omitempty"`
	TaskIDs     []string  `json:"task_ids,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

// StateResource is a resource currently tracked by the State Kernel.
type StateResource struct {
	ID        string            `json:"id,omitempty"`
	Type      string            `json:"type,omitempty"`
	Path      string            `json:"path"`
	Status    string            `json:"status,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	TrackedAt time.Time         `json:"tracked_at,omitempty"`
}

// StateActionRecord is a recent action recorded by the State Kernel.
type StateActionRecord struct {
	Timestamp time.Time `json:"timestamp,omitempty"`
	Action    string    `json:"action"`
	Result    string    `json:"result,omitempty"`
	Success   bool      `json:"success"`
}

// StateCapabilities summarizes currently available and missing capabilities.
type StateCapabilities struct {
	TotalSkills    int      `json:"total_skills,omitempty"`
	DynamicSkills  []string `json:"dynamic_skills,omitempty"`
	UnresolvedGaps int      `json:"unresolved_gaps,omitempty"`
	RecentGaps     []string `json:"recent_gaps,omitempty"`
}

// StateSnapshot is the full structured State Kernel snapshot.
type StateSnapshot struct {
	Goals         []StateGoal         `json:"goals"`
	Resources     []StateResource     `json:"resources"`
	Focus         string              `json:"focus,omitempty"`
	Topics        []string            `json:"topics,omitempty"`
	RecentActions []StateActionRecord `json:"recent_actions,omitempty"`
	Capabilities  StateCapabilities   `json:"capabilities,omitempty"`
	UpdatedAt     time.Time           `json:"updated_at,omitempty"`
}

// StateGoalMutationResponse is returned by goal create/update/delete operations.
type StateGoalMutationResponse struct {
	ID     string `json:"id,omitempty"`
	Status string `json:"status"`
}

// StateFocusResponse is returned by /v1/state/focus.
type StateFocusResponse struct {
	Focus string `json:"focus"`
}

// StateResourceMutationResponse is returned by resource track/release operations.
type StateResourceMutationResponse struct {
	Status string `json:"status"`
}

// Snapshot returns the full State Kernel snapshot from /v1/state.
func (s *stateNamespace) Snapshot(ctx context.Context) (StateSnapshot, error) {
	resp, err := apiCall(ctx, "GET", "/v1/state", nil)
	if err != nil {
		return StateSnapshot{}, err
	}
	var out StateSnapshot
	if err := decodeMapResponse(resp, &out); err != nil {
		return StateSnapshot{}, err
	}
	return out, nil
}

// Actions returns recent State Kernel action records from the snapshot.
func (s *stateNamespace) Actions(ctx context.Context) ([]StateActionRecord, error) {
	snap, err := s.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	if snap.RecentActions == nil {
		return []StateActionRecord{}, nil
	}
	return snap.RecentActions, nil
}

// Capabilities returns the State Kernel capabilities section from the snapshot.
func (s *stateNamespace) Capabilities(ctx context.Context) (StateCapabilities, error) {
	snap, err := s.Snapshot(ctx)
	if err != nil {
		return StateCapabilities{}, err
	}
	return snap.Capabilities, nil
}

// Goals lists goals tracked by the State Kernel.
func (s *stateNamespace) Goals(ctx context.Context) ([]StateGoal, error) {
	var out []StateGoal
	if err := apiCallInto(ctx, "GET", "/v1/state/goals", nil, &out); err != nil {
		return nil, err
	}
	if out == nil {
		return []StateGoal{}, nil
	}
	return out, nil
}

// SaveGoal creates or updates a State Kernel goal.
func (s *stateNamespace) SaveGoal(ctx context.Context, goal StateGoal) (StateGoalMutationResponse, error) {
	resp, err := apiCall(ctx, "POST", "/v1/state/goals", goal)
	if err != nil {
		return StateGoalMutationResponse{}, err
	}
	var out StateGoalMutationResponse
	if err := decodeMapResponse(resp, &out); err != nil {
		return StateGoalMutationResponse{}, err
	}
	return out, nil
}

// Focus returns the current State Kernel focus string.
func (s *stateNamespace) Focus(ctx context.Context) (string, error) {
	resp, err := apiCall(ctx, "GET", "/v1/state/focus", nil)
	if err != nil {
		return "", err
	}
	var out StateFocusResponse
	if err := decodeMapResponse(resp, &out); err != nil {
		return "", err
	}
	return out.Focus, nil
}

// Resources lists active resources tracked by the State Kernel.
func (s *stateNamespace) Resources(ctx context.Context) ([]StateResource, error) {
	var out []StateResource
	if err := apiCallInto(ctx, "GET", "/v1/state/resources", nil, &out); err != nil {
		return nil, err
	}
	if out == nil {
		return []StateResource{}, nil
	}
	return out, nil
}

// ── LLM ──

// LLM calls the agent's language model with a system prompt and user input.
func LLM(ctx context.Context, systemPrompt, userInput string) (string, error) {
	resp, err := apiCall(ctx, "POST", "/v1/plugin-api/llm", map[string]any{
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userInput},
		},
		"temperature": 0.7,
	})
	if err != nil {
		return "", err
	}
	reply, _ := resp["reply"].(string)
	return reply, nil
}

// Chat sends a multi-turn conversation to the LLM.
func Chat(ctx context.Context, messages []Message, temperature float64) (string, error) {
	msgList := make([]map[string]string, len(messages))
	for i, m := range messages {
		msgList[i] = map[string]string{"role": m.Role, "content": m.Content}
	}
	resp, err := apiCall(ctx, "POST", "/v1/plugin-api/llm", map[string]any{
		"messages":    msgList,
		"temperature": temperature,
	})
	if err != nil {
		return "", err
	}
	reply, _ := resp["reply"].(string)
	return reply, nil
}

// Message is a chat message.
type Message struct {
	Role    string
	Content string
}

// ── Web Search ──

// SearchResult is a single search result.
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// Search queries the web using the agent's search providers.
func Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	resp, err := apiCall(ctx, "POST", "/v1/plugin-api/search", map[string]any{
		"query": query, "limit": limit,
	})
	if err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(resp["results"])
	var results []SearchResult
	json.Unmarshal(raw, &results)
	return results, nil
}

// ── Channel Messaging ──

// Send sends a message through a channel (Telegram, Feishu, Discord, etc.).
func Send(ctx context.Context, channelType, target, content string) error {
	_, err := apiCall(ctx, "POST", "/v1/plugin-api/send", map[string]any{
		"channel": channelType,
		"target":  target,
		"content": content,
		"format":  "markdown",
	})
	return err
}

// ── HTTP Request ──

// HTTP makes an arbitrary HTTP request (requires "network" permission).
func HTTP(ctx context.Context, method, url string, body any, headers map[string]string) ([]byte, int, error) {
	var reqBody io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reqBody = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 50<<20))
	return data, resp.StatusCode, nil
}

// ── Plugin Memory (private namespace) ──

// Memory provides access to the plugin's private key-value store.
var Memory = &memoryNamespace{}

type memoryNamespace struct{}

func (m *memoryNamespace) Get(ctx context.Context, key string) (string, error) {
	resp, err := apiCall(ctx, "POST", "/v1/plugin-api/memory/get", map[string]any{"key": key})
	if err != nil {
		return "", err
	}
	v, _ := resp["value"].(string)
	return v, nil
}

func (m *memoryNamespace) Set(ctx context.Context, key, value string) error {
	_, err := apiCall(ctx, "POST", "/v1/plugin-api/memory/set", map[string]any{
		"key": key, "value": value,
	})
	return err
}

func (m *memoryNamespace) Delete(ctx context.Context, key string) error {
	_, err := apiCall(ctx, "POST", "/v1/plugin-api/memory/delete", map[string]any{"key": key})
	return err
}

func (m *memoryNamespace) List(ctx context.Context, prefix string) (map[string]string, error) {
	resp, err := apiCall(ctx, "POST", "/v1/plugin-api/memory/list", map[string]any{"prefix": prefix})
	if err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(resp["entries"])
	var entries map[string]string
	json.Unmarshal(raw, &entries)
	return entries, nil
}

func (m *memoryNamespace) Search(ctx context.Context, query string, limit int) ([]string, error) {
	resp, err := apiCall(ctx, "POST", "/v1/plugin-api/memory/search", map[string]any{
		"query": query, "limit": limit,
	})
	if err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(resp["results"])
	var results []string
	json.Unmarshal(raw, &results)
	return results, nil
}

// ── Agent Memory (shared) ──

// AgentMemory provides access to the agent's shared memory system.
var AgentMemory = &agentMemoryNamespace{}

type agentMemoryNamespace struct{}

// Search queries the agent's combined memory (short+mid+long+graph+editable).
func (m *agentMemoryNamespace) Search(ctx context.Context, query string) (string, error) {
	resp, err := apiCall(ctx, "POST", "/v1/plugin-api/agent-memory/search", map[string]any{
		"query": query, "top_k": 5,
	})
	if err != nil {
		return "", err
	}
	context, _ := resp["context"].(string)
	return context, nil
}

// Add writes a fact to the agent's mid-term memory.
func (m *agentMemoryNamespace) Add(ctx context.Context, fact string) error {
	_, err := apiCall(ctx, "POST", "/v1/plugin-api/agent-memory/add", map[string]any{
		"fact": fact, "source": pluginName,
	})
	return err
}

// ── Knowledge Base ──

// Knowledge provides access to the agent's RAG knowledge base.
var Knowledge = &knowledgeNamespace{}

type knowledgeNamespace struct{}

func (k *knowledgeNamespace) Search(ctx context.Context, query string, limit int) ([]map[string]any, error) {
	resp, err := apiCall(ctx, "POST", "/v1/plugin-api/knowledge/search", map[string]any{
		"query": query, "limit": limit,
	})
	if err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(resp["results"])
	var results []map[string]any
	json.Unmarshal(raw, &results)
	return results, nil
}

func (k *knowledgeNamespace) Ingest(ctx context.Context, content, filename string) error {
	_, err := apiCall(ctx, "POST", "/v1/plugin-api/knowledge/ingest", map[string]any{
		"content": content, "source": pluginName, "filename": filename,
	})
	return err
}

// ── Cron ──

// Cron provides access to the scheduled task system.
var Cron = &cronNamespace{}

type cronNamespace struct{}

func (c *cronNamespace) Add(ctx context.Context, expr, name, message string) (string, error) {
	resp, err := apiCall(ctx, "POST", "/v1/plugin-api/cron/add", map[string]any{
		"expression": expr,
		"name":       pluginName + ":" + name,
		"message":    message,
	})
	if err != nil {
		return "", err
	}
	id, _ := resp["id"].(string)
	return id, nil
}

func (c *cronNamespace) Remove(ctx context.Context, id string) error {
	_, err := apiCall(ctx, "POST", "/v1/plugin-api/cron/remove", map[string]any{"id": id})
	return err
}

// ── Skill Registration ──

// SkillHandler is the function signature for plugin-provided skills.
type SkillHandler func(ctx context.Context, args map[string]any) (string, error)

// HookHandler is the function signature for lifecycle hooks.
type HookHandler func(ctx context.Context, data map[string]any)

var (
	registeredSkills = make(map[string]registeredSkill)
	registeredHooks  = make(map[string][]HookHandler)
	hookMu           sync.RWMutex
)

type registeredSkill struct {
	Name        string
	Description string
	Handler     SkillHandler
}

// RegisterSkill registers a new skill that the Planner can call.
// The skill will be available as a function call tool in the LLM.
func RegisterSkill(name, description string, handler SkillHandler) {
	registeredSkills[name] = registeredSkill{
		Name:        name,
		Description: description,
		Handler:     handler,
	}
}

// OnChatBefore registers a hook called before each chat message is processed.
func OnChatBefore(handler HookHandler) {
	hookMu.Lock()
	registeredHooks["chat.before"] = append(registeredHooks["chat.before"], handler)
	hookMu.Unlock()
}

// OnChatAfter registers a hook called after each chat reply is generated.
func OnChatAfter(handler HookHandler) {
	hookMu.Lock()
	registeredHooks["chat.after"] = append(registeredHooks["chat.after"], handler)
	hookMu.Unlock()
}

// OnMemoryExtract registers a hook called when facts are extracted from conversations.
func OnMemoryExtract(handler HookHandler) {
	hookMu.Lock()
	registeredHooks["memory.extract"] = append(registeredHooks["memory.extract"], handler)
	hookMu.Unlock()
}

// ── System Extension Registration ──

// RegisterProvider adds a new LLM provider to the agent.
// The provider must serve an OpenAI-compatible API.
func RegisterProvider(ctx context.Context, id, baseURL, model string, opts ...ProviderOpt) error {
	cfg := map[string]any{
		"id": id, "base_url": baseURL, "model": model, "type": "chat",
	}
	for _, opt := range opts {
		opt(cfg)
	}
	_, err := apiCall(ctx, "POST", "/v1/plugin-api/register/provider", cfg)
	return err
}

// ProviderOpt configures optional provider fields.
type ProviderOpt func(map[string]any)

func WithAPIKeys(keys ...string) ProviderOpt {
	return func(m map[string]any) { m["api_keys"] = keys }
}

func WithTier(tier string) ProviderOpt {
	return func(m map[string]any) { m["tier"] = tier }
}

func WithProviderType(t string) ProviderOpt {
	return func(m map[string]any) { m["type"] = t }
}

// RegisterChannel adds a new messaging channel adapter.
func RegisterChannel(ctx context.Context, name, webhookURL, sendEndpoint string) error {
	_, err := apiCall(ctx, "POST", "/v1/plugin-api/register/channel", map[string]any{
		"name": name, "webhook_url": webhookURL, "send_endpoint": sendEndpoint,
	})
	return err
}

// RegisterSearchEngine adds a new web search provider.
func RegisterSearchEngine(ctx context.Context, name, baseURL, apiKey string) error {
	_, err := apiCall(ctx, "POST", "/v1/plugin-api/register/search", map[string]any{
		"name": name, "base_url": baseURL, "api_key": apiKey,
	})
	return err
}

// RegisterGuardrail adds a new safety rule.
func RegisterGuardrail(ctx context.Context, name, description, phase string, keywords []string) error {
	_, err := apiCall(ctx, "POST", "/v1/plugin-api/register/guardrail", map[string]any{
		"name": name, "description": description, "phase": phase, "keywords": keywords,
	})
	return err
}

// RegisterEmbedding adds a new vector embedding model.
func RegisterEmbedding(ctx context.Context, name, baseURL, model string, dimensions int) error {
	_, err := apiCall(ctx, "POST", "/v1/plugin-api/register/embedding", map[string]any{
		"name": name, "base_url": baseURL, "model": model, "dimensions": dimensions,
	})
	return err
}

// RegisterSpeech adds a new TTS or STT engine.
func RegisterSpeech(ctx context.Context, name, speechType, baseURL, model string) error {
	_, err := apiCall(ctx, "POST", "/v1/plugin-api/register/speech", map[string]any{
		"name": name, "type": speechType, "base_url": baseURL, "model": model,
	})
	return err
}

// ── Plugin Lifecycle ──

// Run starts the plugin and blocks until the agent shuts it down.
// It starts an HTTP server for the agent to call registered skills and hooks,
// then registers itself with the agent via the plugin API.
func Run() {
	port := envOr("YUNQUE_PLUGIN_PORT", "0")

	mux := http.NewServeMux()

	// Skill execution endpoint
	mux.HandleFunc("/skill/", func(w http.ResponseWriter, r *http.Request) {
		skillName := strings.TrimPrefix(r.URL.Path, "/skill/")
		skill, ok := registeredSkills[skillName]
		if !ok {
			http.Error(w, `{"error":"skill not found"}`, http.StatusNotFound)
			return
		}
		var args map[string]any
		json.NewDecoder(r.Body).Decode(&args)

		result, err := skill.Handler(r.Context(), args)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"result": result})
	})

	// Hook callback endpoint
	mux.HandleFunc("/hook/", func(w http.ResponseWriter, r *http.Request) {
		hookName := strings.TrimPrefix(r.URL.Path, "/hook/")
		hookMu.RLock()
		handlers := registeredHooks[hookName]
		hookMu.RUnlock()

		var data map[string]any
		json.NewDecoder(r.Body).Decode(&data)

		for _, h := range handlers {
			h(r.Context(), data)
		}
		w.WriteHeader(http.StatusOK)
	})

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Start server
	srv := &http.Server{Addr: ":" + port, Handler: mux}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("plugin server failed", "err", err)
		}
	}()

	slog.Info("yunque plugin started",
		"name", pluginName,
		"skills", len(registeredSkills),
		"hooks", len(registeredHooks),
	)

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	slog.Info("yunque plugin stopped", "name", pluginName)
}
