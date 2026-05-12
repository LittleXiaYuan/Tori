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
	State        *stateNamespace
	Reflect      *reflectNamespace
	Missions     *missionsNamespace
	Scheduler    *schedulerNamespace
	CronSystem   *cronSystemNamespace
	Triggers     *triggersNamespace
	MemoryCore   *memoryCoreNamespace
	Graph        *graphNamespace
	KnowledgeKB  *knowledgeKBNamespace
	LoRA         *loRANamespace
	Workflows    *workflowsNamespace
	Connectors   *connectorsNamespace
	Notify       *notifyNamespace
	Projects     *projectsNamespace
	Market       *skillMarketNamespace
	Dispatch     *dispatchNamespace
	Orchestrator *orchestratorNamespace
	Plugin       *pluginRuntimeNamespace
	Memory       *memoryNamespace
	AgentMemory  *agentMemoryNamespace
	Knowledge    *knowledgeNamespace
	Cron         *cronNamespace
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

// ── Knowledge Graph ──

// Graph provides focused access to the host /v1/graph/* knowledge graph API.
var Graph = &graphNamespace{}

type graphNamespace struct{}

type GraphEntity struct {
	ID         string            `json:"id,omitempty"`
	Name       string            `json:"name"`
	Type       string            `json:"type,omitempty"`
	Properties map[string]string `json:"properties,omitempty"`
	CreatedAt  string            `json:"created_at,omitempty"`
	UpdatedAt  string            `json:"updated_at,omitempty"`
	Mentions   int               `json:"mentions,omitempty"`
	Extra      map[string]any    `json:"-"`
}

type GraphRelation struct {
	ID        string         `json:"id,omitempty"`
	FromID    string         `json:"from_id"`
	ToID      string         `json:"to_id"`
	Type      string         `json:"type"`
	Weight    float64        `json:"weight,omitempty"`
	Context   string         `json:"context,omitempty"`
	CreatedAt string         `json:"created_at,omitempty"`
	Extra     map[string]any `json:"-"`
}

type GraphEntitiesResponse struct {
	Entities []GraphEntity `json:"entities"`
}

type GraphRelationsResponse struct {
	Relations []GraphRelation `json:"relations"`
}

type GraphDeleteEntityResponse struct {
	OK bool `json:"ok"`
}

type GraphContextResponse struct {
	Context   string           `json:"context"`
	Neighbors []map[string]any `json:"neighbors,omitempty"`
}

type GraphStatsResponse struct {
	Entities  int `json:"entities"`
	Relations int `json:"relations"`
}

func (g *graphNamespace) Entities(ctx context.Context, query string) (GraphEntitiesResponse, error) {
	path := "/v1/graph/entities"
	if query != "" {
		path += "?q=" + url.QueryEscape(query)
	}
	var out GraphEntitiesResponse
	if err := apiCallInto(ctx, http.MethodGet, path, nil, &out); err != nil {
		return GraphEntitiesResponse{}, err
	}
	if out.Entities == nil {
		out.Entities = []GraphEntity{}
	}
	return out, nil
}

func (g *graphNamespace) PutEntity(ctx context.Context, entity GraphEntity) (GraphEntity, error) {
	var out GraphEntity
	if err := apiCallInto(ctx, http.MethodPost, "/v1/graph/entities", entity, &out); err != nil {
		return GraphEntity{}, err
	}
	return out, nil
}

func (g *graphNamespace) DeleteEntity(ctx context.Context, id string) (GraphDeleteEntityResponse, error) {
	var out GraphDeleteEntityResponse
	if err := apiCallInto(ctx, http.MethodDelete, "/v1/graph/entities?id="+url.QueryEscape(id), nil, &out); err != nil {
		return GraphDeleteEntityResponse{}, err
	}
	return out, nil
}

func (g *graphNamespace) Relations(ctx context.Context, entityID string) (GraphRelationsResponse, error) {
	path := "/v1/graph/relations"
	if entityID != "" {
		path += "?entity_id=" + url.QueryEscape(entityID)
	}
	var out GraphRelationsResponse
	if err := apiCallInto(ctx, http.MethodGet, path, nil, &out); err != nil {
		return GraphRelationsResponse{}, err
	}
	if out.Relations == nil {
		out.Relations = []GraphRelation{}
	}
	return out, nil
}

func (g *graphNamespace) PutRelation(ctx context.Context, relation GraphRelation) (GraphRelation, error) {
	var out GraphRelation
	if err := apiCallInto(ctx, http.MethodPost, "/v1/graph/relations", relation, &out); err != nil {
		return GraphRelation{}, err
	}
	return out, nil
}

func (g *graphNamespace) ContextByEntityID(ctx context.Context, entityID string) (GraphContextResponse, error) {
	var out GraphContextResponse
	if err := apiCallInto(ctx, http.MethodGet, "/v1/graph/context?entity_id="+url.QueryEscape(entityID), nil, &out); err != nil {
		return GraphContextResponse{}, err
	}
	return out, nil
}

func (g *graphNamespace) ContextByName(ctx context.Context, name string) (GraphContextResponse, error) {
	var out GraphContextResponse
	if err := apiCallInto(ctx, http.MethodGet, "/v1/graph/context?name="+url.QueryEscape(name), nil, &out); err != nil {
		return GraphContextResponse{}, err
	}
	return out, nil
}

func (g *graphNamespace) Stats(ctx context.Context) (GraphStatsResponse, error) {
	var out GraphStatsResponse
	if err := apiCallInto(ctx, http.MethodGet, "/v1/graph/stats", nil, &out); err != nil {
		return GraphStatsResponse{}, err
	}
	return out, nil
}

// ── Knowledge Base (host) ──

// KnowledgeKB provides focused access to the host /v1/knowledge/* RAG API.
// It is separate from Knowledge, which targets plugin-owned /v1/plugin-api/knowledge/* helpers.
var KnowledgeKB = &knowledgeKBNamespace{}

type knowledgeKBNamespace struct{}

type KnowledgeChunk struct {
	ID       string         `json:"id,omitempty"`
	SourceID string         `json:"source_id,omitempty"`
	Source   string         `json:"source,omitempty"`
	File     string         `json:"file,omitempty"`
	Path     string         `json:"path,omitempty"`
	Lang     string         `json:"lang,omitempty"`
	Content  string         `json:"content,omitempty"`
	Text     string         `json:"text,omitempty"`
	Score    float64        `json:"score,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
	Extra    map[string]any `json:"-"`
}

type KnowledgeSource struct {
	ID        string         `json:"id"`
	Name      string         `json:"name,omitempty"`
	Type      string         `json:"type,omitempty"`
	Path      string         `json:"path,omitempty"`
	Trigger   string         `json:"trigger,omitempty"`
	Chunks    int            `json:"chunks,omitempty"`
	Size      int64          `json:"size,omitempty"`
	CreatedAt string         `json:"created_at,omitempty"`
	UpdatedAt string         `json:"updated_at,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	Extra     map[string]any `json:"-"`
}

type KnowledgeStatsResponse map[string]any

type KnowledgeSearchOptions struct {
	Query string
	Limit int
	File  string
	Lang  string
}

type KnowledgeSearchResponse struct {
	Chunks []KnowledgeChunk `json:"chunks"`
	Count  int              `json:"count"`
}

type KnowledgeSourcesResponse struct {
	Sources []KnowledgeSource `json:"sources"`
}

type KnowledgeIngestRequest struct {
	Name    string `json:"name,omitempty"`
	Trigger string `json:"trigger,omitempty"`
	Content string `json:"content"`
}

type KnowledgeUpdateSourceRequest struct {
	ID      string `json:"id"`
	Name    string `json:"name,omitempty"`
	Trigger string `json:"trigger,omitempty"`
	Content string `json:"content,omitempty"`
}

type KnowledgeImportURLRequest struct {
	URL           string `json:"url"`
	Name          string `json:"name,omitempty"`
	CrawlChildren bool   `json:"crawl_children,omitempty"`
	MaxPages      int    `json:"max_pages,omitempty"`
}

type KnowledgeImportRepoRequest struct {
	Path     string `json:"path"`
	MaxFiles int    `json:"max_files,omitempty"`
}

type KnowledgeMutationResponse struct {
	Source  *KnowledgeSource       `json:"source,omitempty"`
	Sources []KnowledgeSource      `json:"sources,omitempty"`
	Stats   KnowledgeStatsResponse `json:"stats,omitempty"`
	Extra   map[string]any         `json:"-"`
}

type KnowledgeDeleteResponse struct {
	Deleted string                 `json:"deleted,omitempty"`
	Stats   KnowledgeStatsResponse `json:"stats,omitempty"`
}

func (k *knowledgeKBNamespace) Stats(ctx context.Context) (KnowledgeStatsResponse, error) {
	var out KnowledgeStatsResponse
	if err := apiCallInto(ctx, http.MethodGet, "/v1/knowledge/stats", nil, &out); err != nil {
		return nil, err
	}
	if out == nil {
		return KnowledgeStatsResponse{}, nil
	}
	return out, nil
}

func (k *knowledgeKBNamespace) Sources(ctx context.Context) (KnowledgeSourcesResponse, error) {
	var out KnowledgeSourcesResponse
	if err := apiCallInto(ctx, http.MethodGet, "/v1/knowledge/sources", nil, &out); err != nil {
		return KnowledgeSourcesResponse{}, err
	}
	if out.Sources == nil {
		out.Sources = []KnowledgeSource{}
	}
	return out, nil
}

func (k *knowledgeKBNamespace) Search(ctx context.Context, opts KnowledgeSearchOptions) (KnowledgeSearchResponse, error) {
	q := url.Values{}
	q.Set("q", opts.Query)
	if opts.Limit > 0 {
		q.Set("n", strconv.Itoa(opts.Limit))
	}
	if opts.File != "" {
		q.Set("file", opts.File)
	}
	if opts.Lang != "" {
		q.Set("lang", opts.Lang)
	}
	var out KnowledgeSearchResponse
	if err := apiCallInto(ctx, http.MethodGet, "/v1/knowledge/search?"+q.Encode(), nil, &out); err != nil {
		return KnowledgeSearchResponse{}, err
	}
	if out.Chunks == nil {
		out.Chunks = []KnowledgeChunk{}
	}
	return out, nil
}

func (k *knowledgeKBNamespace) Ingest(ctx context.Context, req KnowledgeIngestRequest) (KnowledgeMutationResponse, error) {
	var out KnowledgeMutationResponse
	if err := apiCallInto(ctx, http.MethodPost, "/v1/knowledge/ingest", req, &out); err != nil {
		return KnowledgeMutationResponse{}, err
	}
	return out, nil
}

func (k *knowledgeKBNamespace) UpdateSource(ctx context.Context, req KnowledgeUpdateSourceRequest) (KnowledgeMutationResponse, error) {
	var out KnowledgeMutationResponse
	if err := apiCallInto(ctx, http.MethodPost, "/v1/knowledge/source/update", req, &out); err != nil {
		return KnowledgeMutationResponse{}, err
	}
	return out, nil
}

func (k *knowledgeKBNamespace) DeleteSource(ctx context.Context, id string) (KnowledgeDeleteResponse, error) {
	var out KnowledgeDeleteResponse
	if err := apiCallInto(ctx, http.MethodDelete, "/v1/knowledge/source?id="+url.QueryEscape(id), nil, &out); err != nil {
		return KnowledgeDeleteResponse{}, err
	}
	return out, nil
}

func (k *knowledgeKBNamespace) ImportURL(ctx context.Context, req KnowledgeImportURLRequest) (KnowledgeMutationResponse, error) {
	var out KnowledgeMutationResponse
	if err := apiCallInto(ctx, http.MethodPost, "/v1/knowledge/import-url", req, &out); err != nil {
		return KnowledgeMutationResponse{}, err
	}
	return out, nil
}

func (k *knowledgeKBNamespace) ImportRepo(ctx context.Context, req KnowledgeImportRepoRequest) (KnowledgeMutationResponse, error) {
	var out KnowledgeMutationResponse
	if err := apiCallInto(ctx, http.MethodPost, "/v1/knowledge/import-repo", req, &out); err != nil {
		return KnowledgeMutationResponse{}, err
	}
	return out, nil
}

// ── LoRA Training and Evolution (host) ──

// LoRA provides focused access to host /v1/lora/* local-brain training lifecycle APIs.
var LoRA = &loRANamespace{}

type loRANamespace struct{}

type LoRAStatusResponse map[string]any
type LoRAHistoryResponse map[string]any
type LoRASummaryResponse map[string]any
type LoRAEvolutionResponse map[string]any
type LoRAConfigResponse map[string]any
type LoRARollbackResponse map[string]any
type TriggerLoRAResponse map[string]any

type LoRAPreviewOptions struct {
	TenantID string
}

type TriggerLoRARequest struct {
	TenantID string `json:"tenant_id,omitempty"`
}

type LoRAConfig map[string]any

func (l *loRANamespace) Status(ctx context.Context) (LoRAStatusResponse, error) {
	var out LoRAStatusResponse
	if err := apiCallInto(ctx, http.MethodGet, "/v1/lora/status", nil, &out); err != nil {
		return nil, err
	}
	return nonNilMap(out), nil
}

func (l *loRANamespace) History(ctx context.Context) (LoRAHistoryResponse, error) {
	var out LoRAHistoryResponse
	if err := apiCallInto(ctx, http.MethodGet, "/v1/lora/history", nil, &out); err != nil {
		return nil, err
	}
	return nonNilMap(out), nil
}

func (l *loRANamespace) Summary(ctx context.Context) (LoRASummaryResponse, error) {
	var out LoRASummaryResponse
	if err := apiCallInto(ctx, http.MethodGet, "/v1/lora/summary", nil, &out); err != nil {
		return nil, err
	}
	return nonNilMap(out), nil
}

func (l *loRANamespace) Preview(ctx context.Context, opts LoRAPreviewOptions) (map[string]any, error) {
	path := "/v1/lora/preview"
	if opts.TenantID != "" {
		path += "?tenant_id=" + url.QueryEscape(opts.TenantID)
	}
	var out map[string]any
	if err := apiCallInto(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return nonNilMap(out), nil
}

func (l *loRANamespace) Trigger(ctx context.Context, req TriggerLoRARequest) (TriggerLoRAResponse, error) {
	var out TriggerLoRAResponse
	if err := apiCallInto(ctx, http.MethodPost, "/v1/lora/trigger", req, &out); err != nil {
		return nil, err
	}
	return nonNilMap(out), nil
}

func (l *loRANamespace) Rollback(ctx context.Context) (LoRARollbackResponse, error) {
	var out LoRARollbackResponse
	if err := apiCallInto(ctx, http.MethodPost, "/v1/lora/rollback", map[string]any{}, &out); err != nil {
		return nil, err
	}
	return nonNilMap(out), nil
}

func (l *loRANamespace) Evolution(ctx context.Context) (LoRAEvolutionResponse, error) {
	var out LoRAEvolutionResponse
	if err := apiCallInto(ctx, http.MethodGet, "/v1/lora/evolution", nil, &out); err != nil {
		return nil, err
	}
	return nonNilMap(out), nil
}

func (l *loRANamespace) Config(ctx context.Context) (LoRAConfigResponse, error) {
	var out LoRAConfigResponse
	if err := apiCallInto(ctx, http.MethodGet, "/v1/lora/config", nil, &out); err != nil {
		return nil, err
	}
	return nonNilMap(out), nil
}

func (l *loRANamespace) UpdateConfig(ctx context.Context, config LoRAConfig) (LoRAConfigResponse, error) {
	var out LoRAConfigResponse
	if err := apiCallInto(ctx, http.MethodPut, "/v1/lora/config", config, &out); err != nil {
		return nil, err
	}
	return nonNilMap(out), nil
}

func nonNilMap[T ~map[string]any](value T) T {
	if value == nil {
		return T{}
	}
	return value
}

// ── Workflow Orchestration (host) ──

// Workflows provides focused access to host /v1/workflows* DAG orchestration APIs.
var Workflows = &workflowsNamespace{}

type workflowsNamespace struct{}

type WorkflowDefinition struct {
	ID          string           `json:"id,omitempty"`
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Version     int              `json:"version,omitempty"`
	Nodes       []map[string]any `json:"nodes,omitempty"`
	Edges       []map[string]any `json:"edges,omitempty"`
	Variables   []map[string]any `json:"variables,omitempty"`
	TenantID    string           `json:"tenant_id,omitempty"`
	CreatedAt   string           `json:"created_at,omitempty"`
	UpdatedAt   string           `json:"updated_at,omitempty"`
	Extra       map[string]any   `json:"-"`
}

type WorkflowInstance struct {
	ID           string         `json:"id"`
	DefinitionID string         `json:"definition_id"`
	Version      int            `json:"version,omitempty"`
	Status       string         `json:"status"`
	Variables    map[string]any `json:"variables,omitempty"`
	NodeStates   map[string]any `json:"node_states,omitempty"`
	Error        string         `json:"error,omitempty"`
	TenantID     string         `json:"tenant_id,omitempty"`
	CreatedAt    string         `json:"created_at,omitempty"`
	UpdatedAt    string         `json:"updated_at,omitempty"`
	StartedAt    string         `json:"started_at,omitempty"`
	FinishedAt   string         `json:"finished_at,omitempty"`
	Extra        map[string]any `json:"-"`
}

type WorkflowListResponse struct {
	Workflows []WorkflowDefinition `json:"workflows"`
	Total     int                  `json:"total"`
}

type WorkflowInstancesResponse struct {
	Instances []WorkflowInstance `json:"instances"`
	Total     int                `json:"total"`
}

type WorkflowRunRequest struct {
	DefinitionID string         `json:"definition_id"`
	Variables    map[string]any `json:"variables,omitempty"`
}

type WorkflowRunResponse struct {
	Status     string           `json:"status"`
	InstanceID string           `json:"instance_id"`
	Instance   WorkflowInstance `json:"instance"`
}

type WorkflowCancelRequest struct {
	InstanceID string `json:"instance_id"`
}

type WorkflowCancelResponse struct {
	Status     string `json:"status,omitempty"`
	InstanceID string `json:"instance_id,omitempty"`
}

type WorkflowDeleteResponse struct {
	Deleted string `json:"deleted,omitempty"`
}

func (w *workflowsNamespace) List(ctx context.Context) (WorkflowListResponse, error) {
	var out WorkflowListResponse
	if err := apiCallInto(ctx, http.MethodGet, "/v1/workflows", nil, &out); err != nil {
		return WorkflowListResponse{}, err
	}
	if out.Workflows == nil {
		out.Workflows = []WorkflowDefinition{}
	}
	return out, nil
}

func (w *workflowsNamespace) Get(ctx context.Context, id string) (WorkflowDefinition, error) {
	var out WorkflowDefinition
	if err := apiCallInto(ctx, http.MethodGet, "/v1/workflows?id="+url.QueryEscape(id), nil, &out); err != nil {
		return WorkflowDefinition{}, err
	}
	return out, nil
}

func (w *workflowsNamespace) Save(ctx context.Context, def WorkflowDefinition) (WorkflowDefinition, error) {
	var out WorkflowDefinition
	if err := apiCallInto(ctx, http.MethodPost, "/v1/workflows", def, &out); err != nil {
		return WorkflowDefinition{}, err
	}
	return out, nil
}

func (w *workflowsNamespace) Delete(ctx context.Context, id string) (WorkflowDeleteResponse, error) {
	var out WorkflowDeleteResponse
	if err := apiCallInto(ctx, http.MethodDelete, "/v1/workflows?id="+url.QueryEscape(id), nil, &out); err != nil {
		return WorkflowDeleteResponse{}, err
	}
	return out, nil
}

func (w *workflowsNamespace) Run(ctx context.Context, req WorkflowRunRequest) (WorkflowRunResponse, error) {
	var out WorkflowRunResponse
	if err := apiCallInto(ctx, http.MethodPost, "/v1/workflows/run", req, &out); err != nil {
		return WorkflowRunResponse{}, err
	}
	return out, nil
}

func (w *workflowsNamespace) Instances(ctx context.Context) (WorkflowInstancesResponse, error) {
	var out WorkflowInstancesResponse
	if err := apiCallInto(ctx, http.MethodGet, "/v1/workflows/instances", nil, &out); err != nil {
		return WorkflowInstancesResponse{}, err
	}
	if out.Instances == nil {
		out.Instances = []WorkflowInstance{}
	}
	return out, nil
}

func (w *workflowsNamespace) GetInstance(ctx context.Context, id string) (WorkflowInstance, error) {
	var out WorkflowInstance
	if err := apiCallInto(ctx, http.MethodGet, "/v1/workflows/instances?id="+url.QueryEscape(id), nil, &out); err != nil {
		return WorkflowInstance{}, err
	}
	return out, nil
}

func (w *workflowsNamespace) Cancel(ctx context.Context, req WorkflowCancelRequest) (WorkflowCancelResponse, error) {
	var out WorkflowCancelResponse
	if err := apiCallInto(ctx, http.MethodPost, "/v1/workflows/cancel", req, &out); err != nil {
		return WorkflowCancelResponse{}, err
	}
	return out, nil
}

// Connectors provides focused access to connector catalog, auth, and action execution APIs.
var Connectors = &connectorsNamespace{}

type connectorsNamespace struct{}

type ConnectorStatus string

type ConnectorView struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Icon        string          `json:"icon,omitempty"`
	Category    string          `json:"category,omitempty"`
	AuthType    string          `json:"auth_type,omitempty"`
	Beta        bool            `json:"beta,omitempty"`
	Supported   bool            `json:"supported"`
	Status      ConnectorStatus `json:"status"`
	UserInfo    string          `json:"user_info,omitempty"`
	Error       string          `json:"error,omitempty"`
	ActionCount int             `json:"action_count,omitempty"`
}

type ConnectorAction struct {
	ID          string         `json:"id"`
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Params      map[string]any `json:"params,omitempty"`
}

type ConnectorDefinition struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Icon        string            `json:"icon,omitempty"`
	Category    string            `json:"category,omitempty"`
	AuthType    string            `json:"auth_type,omitempty"`
	Beta        bool              `json:"beta,omitempty"`
	Actions     []ConnectorAction `json:"actions,omitempty"`
}

type ConnectorListResponse struct {
	Connectors []ConnectorView `json:"connectors"`
	Error      string          `json:"error,omitempty"`
}

type ConnectorDetailResponse struct {
	Connector ConnectorDefinition `json:"connector"`
	Supported bool                `json:"supported"`
	Status    ConnectorStatus     `json:"status"`
	UserInfo  string              `json:"user_info,omitempty"`
	Error     string              `json:"error,omitempty"`
}

type ConnectorConnectRequest struct {
	ConnectorID string `json:"connector_id"`
	Token       string `json:"token,omitempty"`
	APIKey      string `json:"api_key,omitempty"`
}

type ConnectorConnectResponse struct {
	OK       bool            `json:"ok"`
	Status   ConnectorStatus `json:"status"`
	UserInfo string          `json:"user_info,omitempty"`
}

type ConnectorOKResponse struct {
	OK bool `json:"ok"`
}

type ConnectorExecuteRequest struct {
	ConnectorID string         `json:"connector_id"`
	ActionID    string         `json:"action_id"`
	Params      map[string]any `json:"params,omitempty"`
}

type ConnectorExecuteResponse struct {
	OK     bool `json:"ok"`
	Result any  `json:"result,omitempty"`
}

func (c *connectorsNamespace) List(ctx context.Context) (ConnectorListResponse, error) {
	var out ConnectorListResponse
	if err := apiCallInto(ctx, http.MethodGet, "/api/connectors", nil, &out); err != nil {
		return ConnectorListResponse{}, err
	}
	if out.Connectors == nil {
		out.Connectors = []ConnectorView{}
	}
	return out, nil
}

func (c *connectorsNamespace) Detail(ctx context.Context, id string) (ConnectorDetailResponse, error) {
	var out ConnectorDetailResponse
	if err := apiCallInto(ctx, http.MethodGet, "/api/connectors/detail?id="+url.QueryEscape(id), nil, &out); err != nil {
		return ConnectorDetailResponse{}, err
	}
	return out, nil
}

func (c *connectorsNamespace) Connect(ctx context.Context, req ConnectorConnectRequest) (ConnectorConnectResponse, error) {
	var out ConnectorConnectResponse
	if err := apiCallInto(ctx, http.MethodPost, "/api/connectors/connect", req, &out); err != nil {
		return ConnectorConnectResponse{}, err
	}
	return out, nil
}

func (c *connectorsNamespace) Disconnect(ctx context.Context, id string) (ConnectorOKResponse, error) {
	var out ConnectorOKResponse
	if err := apiCallInto(ctx, http.MethodPost, "/api/connectors/disconnect", map[string]string{"connector_id": id}, &out); err != nil {
		return ConnectorOKResponse{}, err
	}
	return out, nil
}

func (c *connectorsNamespace) Execute(ctx context.Context, req ConnectorExecuteRequest) (ConnectorExecuteResponse, error) {
	var out ConnectorExecuteResponse
	if req.Params == nil {
		req.Params = map[string]any{}
	}
	if err := apiCallInto(ctx, http.MethodPost, "/api/connectors/execute", req, &out); err != nil {
		return ConnectorExecuteResponse{}, err
	}
	return out, nil
}

// Notify provides focused access to notification channel management and share dispatch APIs.
var Notify = &notifyNamespace{}

type notifyNamespace struct{}

type NotifyChannel struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	URL     string `json:"url,omitempty"`
	Secret  string `json:"secret,omitempty"`
	Enabled bool   `json:"enabled,omitempty"`
}

type NotifyChannelsResponse struct {
	Channels []NotifyChannel `json:"channels"`
}

type NotifyOKResponse struct {
	OK bool `json:"ok"`
}

type NotifyToggleRequest struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
}

type NotifyShareFile struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Size int64  `json:"size,omitempty"`
}

type NotifyShareRequest struct {
	ChannelID string            `json:"channel_id"`
	Title     string            `json:"title,omitempty"`
	Message   string            `json:"message,omitempty"`
	SessionID string            `json:"session_id,omitempty"`
	TaskID    string            `json:"task_id,omitempty"`
	URL       string            `json:"url,omitempty"`
	Files     []NotifyShareFile `json:"files,omitempty"`
}

type NotifyShareResponse struct {
	OK      bool           `json:"ok"`
	SentAt  string         `json:"sent_at,omitempty"`
	Share   map[string]any `json:"share,omitempty"`
	Channel map[string]any `json:"channel,omitempty"`
}

func (n *notifyNamespace) Channels(ctx context.Context) (NotifyChannelsResponse, error) {
	var out NotifyChannelsResponse
	if err := apiCallInto(ctx, http.MethodGet, "/api/notify/channels", nil, &out); err != nil {
		return NotifyChannelsResponse{}, err
	}
	if out.Channels == nil {
		out.Channels = []NotifyChannel{}
	}
	return out, nil
}

func (n *notifyNamespace) AddChannel(ctx context.Context, channel NotifyChannel) (NotifyOKResponse, error) {
	var out NotifyOKResponse
	if err := apiCallInto(ctx, http.MethodPost, "/api/notify/add", channel, &out); err != nil {
		return NotifyOKResponse{}, err
	}
	return out, nil
}

func (n *notifyNamespace) RemoveChannel(ctx context.Context, id string) (NotifyOKResponse, error) {
	var out NotifyOKResponse
	if err := apiCallInto(ctx, http.MethodPost, "/api/notify/remove?id="+url.QueryEscape(id), nil, &out); err != nil {
		return NotifyOKResponse{}, err
	}
	return out, nil
}

func (n *notifyNamespace) ToggleChannel(ctx context.Context, req NotifyToggleRequest) (NotifyOKResponse, error) {
	var out NotifyOKResponse
	if err := apiCallInto(ctx, http.MethodPost, "/api/notify/toggle", req, &out); err != nil {
		return NotifyOKResponse{}, err
	}
	return out, nil
}

func (n *notifyNamespace) TestChannel(ctx context.Context, id string) (NotifyOKResponse, error) {
	var out NotifyOKResponse
	if err := apiCallInto(ctx, http.MethodPost, "/api/notify/test", map[string]string{"id": id}, &out); err != nil {
		return NotifyOKResponse{}, err
	}
	return out, nil
}

func (n *notifyNamespace) Share(ctx context.Context, req NotifyShareRequest) (NotifyShareResponse, error) {
	var out NotifyShareResponse
	if err := apiCallInto(ctx, http.MethodPost, "/api/notify/share", req, &out); err != nil {
		return NotifyShareResponse{}, err
	}
	return out, nil
}

// ── MCP Dispatch ──

// Dispatch provides focused access to external worker registry and dispatch queue APIs.
var Dispatch = &dispatchNamespace{}

type dispatchNamespace struct{}

type DispatchWorker struct {
	ID           string         `json:"id,omitempty"`
	Name         string         `json:"name,omitempty"`
	Type         string         `json:"type,omitempty"`
	Capabilities []string       `json:"capabilities,omitempty"`
	Status       string         `json:"status,omitempty"`
	LastSeen     string         `json:"last_seen,omitempty"`
	Extra        map[string]any `json:"-"`
}

type DispatchWorkersResponse struct {
	Workers []DispatchWorker `json:"workers"`
	Count   int              `json:"count"`
}

type DispatchQueueResponse map[string]any

type DispatchEnqueueRequest struct {
	TaskID       string   `json:"task_id"`
	Capabilities []string `json:"capabilities,omitempty"`
	Priority     int      `json:"priority,omitempty"`
}

type DispatchEnqueueResponse struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
}

type DispatchWorkerConfigResponse struct {
	Type         string `json:"type"`
	MCPConfig    string `json:"mcp_config"`
	Instructions string `json:"instructions"`
	ServerURL    string `json:"server_url"`
}

type DispatchStatusResponse struct {
	Status string `json:"status"`
}

func (d *dispatchNamespace) Workers(ctx context.Context) (DispatchWorkersResponse, error) {
	var out DispatchWorkersResponse
	if err := apiCallInto(ctx, http.MethodGet, "/v1/workers", nil, &out); err != nil {
		return DispatchWorkersResponse{}, err
	}
	if out.Workers == nil {
		out.Workers = []DispatchWorker{}
	}
	return out, nil
}

func (d *dispatchNamespace) Worker(ctx context.Context, id string) (DispatchWorker, error) {
	var out DispatchWorker
	if err := apiCallInto(ctx, http.MethodGet, "/v1/workers/detail?id="+url.QueryEscape(id), nil, &out); err != nil {
		return DispatchWorker{}, err
	}
	return out, nil
}

func (d *dispatchNamespace) RemoveWorker(ctx context.Context, id string) (DispatchStatusResponse, error) {
	var out DispatchStatusResponse
	if err := apiCallInto(ctx, http.MethodPost, "/v1/workers/remove", map[string]string{"id": id}, &out); err != nil {
		return DispatchStatusResponse{}, err
	}
	return out, nil
}

func (d *dispatchNamespace) Queue(ctx context.Context) (DispatchQueueResponse, error) {
	var out DispatchQueueResponse
	if err := apiCallInto(ctx, http.MethodGet, "/v1/dispatch/queue", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (d *dispatchNamespace) Enqueue(ctx context.Context, req DispatchEnqueueRequest) (DispatchEnqueueResponse, error) {
	var out DispatchEnqueueResponse
	if err := apiCallInto(ctx, http.MethodPost, "/v1/dispatch/enqueue", req, &out); err != nil {
		return DispatchEnqueueResponse{}, err
	}
	return out, nil
}

func (d *dispatchNamespace) WorkerConfig(ctx context.Context, workerType string) (DispatchWorkerConfigResponse, error) {
	path := "/v1/workers/config"
	if workerType != "" {
		path += "?type=" + url.QueryEscape(workerType)
	}
	var out DispatchWorkerConfigResponse
	if err := apiCallInto(ctx, http.MethodGet, path, nil, &out); err != nil {
		return DispatchWorkerConfigResponse{}, err
	}
	return out, nil
}

// ── IDE Worker Orchestrator ──

// Orchestrator provides focused access to IDE worker daemon status, sessions, events, and policy APIs.
var Orchestrator = &orchestratorNamespace{}

type orchestratorNamespace struct{}

type OrchestratorPolicy map[string]any

type OrchestratorStatusResponse struct {
	Running        bool               `json:"running"`
	Adapters       []string           `json:"adapters"`
	ActiveSessions int                `json:"active_sessions"`
	Policy         OrchestratorPolicy `json:"policy,omitempty"`
	EventCount     int                `json:"event_count,omitempty"`
}

type OrchestratorToggleResponse struct {
	Status string `json:"status"`
}

type OrchestratorSession struct {
	SessionID string         `json:"session_id"`
	Adapter   string         `json:"adapter"`
	TaskID    string         `json:"task_id"`
	StartedAt string         `json:"started_at,omitempty"`
	Extra     map[string]any `json:"-"`
}

type OrchestratorSessionsResponse struct {
	Sessions []OrchestratorSession `json:"sessions"`
}

type OrchestratorIDE struct {
	Name      string         `json:"name,omitempty"`
	Path      string         `json:"path,omitempty"`
	Available bool           `json:"available,omitempty"`
	Version   string         `json:"version,omitempty"`
	Extra     map[string]any `json:"-"`
}

type OrchestratorDetectResponse struct {
	IDEs []OrchestratorIDE `json:"ides"`
}

type OrchestratorEvent struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	TaskID    string         `json:"task_id,omitempty"`
	WorkerID  string         `json:"worker_id,omitempty"`
	SessionID string         `json:"session_id,omitempty"`
	Message   string         `json:"message,omitempty"`
	Meta      map[string]any `json:"meta,omitempty"`
	Timestamp string         `json:"timestamp,omitempty"`
	Extra     map[string]any `json:"-"`
}

type OrchestratorEventsResponse struct {
	Events []OrchestratorEvent `json:"events"`
	Total  int                 `json:"total,omitempty"`
}

type OrchestratorTaskTimelineResponse struct {
	TaskID string              `json:"task_id"`
	Events []OrchestratorEvent `json:"events"`
}

type OrchestratorPolicyUpdateResponse struct {
	Status string             `json:"status"`
	Policy OrchestratorPolicy `json:"policy"`
}

type OrchestratorAdapterConfig struct {
	AdapterName   string `json:"adapter_name"`
	Binary        string `json:"binary"`
	LaunchArgs    string `json:"launch_args,omitempty"`
	MCPConfigPath string `json:"mcp_config_path"`
	RulesFilePath string `json:"rules_file_path,omitempty"`
	Lifecycle     string `json:"lifecycle,omitempty"`
}

type OrchestratorAdapterResponse struct {
	Status    string `json:"status"`
	Name      string `json:"name"`
	Available bool   `json:"available"`
}

func (o *orchestratorNamespace) Status(ctx context.Context) (OrchestratorStatusResponse, error) {
	var out OrchestratorStatusResponse
	if err := apiCallInto(ctx, http.MethodGet, "/v1/orchestrator/status", nil, &out); err != nil {
		return OrchestratorStatusResponse{}, err
	}
	if out.Adapters == nil {
		out.Adapters = []string{}
	}
	return out, nil
}

func (o *orchestratorNamespace) Toggle(ctx context.Context, action string) (OrchestratorToggleResponse, error) {
	var out OrchestratorToggleResponse
	if err := apiCallInto(ctx, http.MethodPost, "/v1/orchestrator/toggle", map[string]string{"action": action}, &out); err != nil {
		return OrchestratorToggleResponse{}, err
	}
	return out, nil
}

func (o *orchestratorNamespace) Sessions(ctx context.Context) (OrchestratorSessionsResponse, error) {
	var out OrchestratorSessionsResponse
	if err := apiCallInto(ctx, http.MethodGet, "/v1/orchestrator/sessions", nil, &out); err != nil {
		return OrchestratorSessionsResponse{}, err
	}
	if out.Sessions == nil {
		out.Sessions = []OrchestratorSession{}
	}
	return out, nil
}

func (o *orchestratorNamespace) DetectIDEs(ctx context.Context) (OrchestratorDetectResponse, error) {
	var out OrchestratorDetectResponse
	if err := apiCallInto(ctx, http.MethodGet, "/v1/orchestrator/detect", nil, &out); err != nil {
		return OrchestratorDetectResponse{}, err
	}
	if out.IDEs == nil {
		out.IDEs = []OrchestratorIDE{}
	}
	return out, nil
}

func (o *orchestratorNamespace) Events(ctx context.Context, limit int) (OrchestratorEventsResponse, error) {
	path := "/v1/orchestrator/events"
	if limit > 0 {
		path += "?limit=" + strconv.Itoa(limit)
	}
	var out OrchestratorEventsResponse
	if err := apiCallInto(ctx, http.MethodGet, path, nil, &out); err != nil {
		return OrchestratorEventsResponse{}, err
	}
	if out.Events == nil {
		out.Events = []OrchestratorEvent{}
	}
	return out, nil
}

func (o *orchestratorNamespace) TaskTimeline(ctx context.Context, taskID string) (OrchestratorTaskTimelineResponse, error) {
	var out OrchestratorTaskTimelineResponse
	if err := apiCallInto(ctx, http.MethodGet, "/v1/orchestrator/events/task?task_id="+url.QueryEscape(taskID), nil, &out); err != nil {
		return OrchestratorTaskTimelineResponse{}, err
	}
	if out.Events == nil {
		out.Events = []OrchestratorEvent{}
	}
	return out, nil
}

func (o *orchestratorNamespace) Policy(ctx context.Context) (OrchestratorPolicy, error) {
	var out OrchestratorPolicy
	if err := apiCallInto(ctx, http.MethodGet, "/v1/orchestrator/policy", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (o *orchestratorNamespace) UpdatePolicy(ctx context.Context, policy OrchestratorPolicy) (OrchestratorPolicyUpdateResponse, error) {
	var out OrchestratorPolicyUpdateResponse
	if err := apiCallInto(ctx, http.MethodPut, "/v1/orchestrator/policy", policy, &out); err != nil {
		return OrchestratorPolicyUpdateResponse{}, err
	}
	return out, nil
}

func (o *orchestratorNamespace) AddAdapter(ctx context.Context, cfg OrchestratorAdapterConfig) (OrchestratorAdapterResponse, error) {
	var out OrchestratorAdapterResponse
	if err := apiCallInto(ctx, http.MethodPost, "/v1/orchestrator/adapters/add", cfg, &out); err != nil {
		return OrchestratorAdapterResponse{}, err
	}
	return out, nil
}

// ── Skill Market ──

// SkillMarket provides focused access to skill marketplace search, ranking, and stats APIs.
var SkillMarket = &skillMarketNamespace{}

type skillMarketNamespace struct{}

type SkillMarketSkill struct {
	Name         string         `json:"name"`
	Version      string         `json:"version"`
	Description  string         `json:"description,omitempty"`
	Author       string         `json:"author,omitempty"`
	Category     string         `json:"category,omitempty"`
	Tags         []string       `json:"tags,omitempty"`
	License      string         `json:"license,omitempty"`
	Homepage     string         `json:"homepage,omitempty"`
	Deprecated   bool           `json:"deprecated,omitempty"`
	Installs     int            `json:"installs,omitempty"`
	Rating       float64        `json:"rating,omitempty"`
	RatingCount  int            `json:"rating_count,omitempty"`
	CreatedAt    string         `json:"created_at,omitempty"`
	UpdatedAt    string         `json:"updated_at,omitempty"`
	MinVersion   string         `json:"min_version,omitempty"`
	Dependencies []string       `json:"dependencies,omitempty"`
	Extra        map[string]any `json:"-"`
}

type SkillMarketSearchResponse struct {
	Skills []SkillMarketSkill `json:"skills"`
	Count  int                `json:"count,omitempty"`
}

type SkillMarketTopOptions struct {
	N  int
	By string
}

type SkillMarketStatsResponse map[string]any

func (m *skillMarketNamespace) Search(ctx context.Context, query string) (SkillMarketSearchResponse, error) {
	path := "/v1/market/search"
	if query != "" {
		path += "?q=" + url.QueryEscape(query)
	}
	var out SkillMarketSearchResponse
	if err := apiCallInto(ctx, http.MethodGet, path, nil, &out); err != nil {
		return SkillMarketSearchResponse{}, err
	}
	if out.Skills == nil {
		out.Skills = []SkillMarketSkill{}
	}
	return out, nil
}

func (m *skillMarketNamespace) Top(ctx context.Context, opts SkillMarketTopOptions) (SkillMarketSearchResponse, error) {
	q := url.Values{}
	if opts.N > 0 {
		q.Set("n", strconv.Itoa(opts.N))
	}
	if opts.By != "" {
		q.Set("by", opts.By)
	}
	path := "/v1/market/top"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	var out SkillMarketSearchResponse
	if err := apiCallInto(ctx, http.MethodGet, path, nil, &out); err != nil {
		return SkillMarketSearchResponse{}, err
	}
	if out.Skills == nil {
		out.Skills = []SkillMarketSkill{}
	}
	return out, nil
}

func (m *skillMarketNamespace) Stats(ctx context.Context) (SkillMarketStatsResponse, error) {
	var out SkillMarketStatsResponse
	if err := apiCallInto(ctx, http.MethodGet, "/v1/market/stats", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ── Projects ──

// Projects provides focused access to project workspace CRUD APIs.
var Projects = &projectsNamespace{}

type projectsNamespace struct{}

type Project struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	RepoPath    string            `json:"repo_path"`
	RepoURL     string            `json:"repo_url,omitempty"`
	Description string            `json:"description,omitempty"`
	DefaultCaps []string          `json:"default_caps,omitempty"`
	Meta        map[string]string `json:"meta,omitempty"`
	CreatedAt   string            `json:"created_at,omitempty"`
	UpdatedAt   string            `json:"updated_at,omitempty"`
}

type ProjectsListResponse struct {
	Projects []Project `json:"projects"`
}

type CreateProjectRequest struct {
	Name        string            `json:"name"`
	RepoPath    string            `json:"repo_path"`
	RepoURL     string            `json:"repo_url,omitempty"`
	Description string            `json:"description,omitempty"`
	DefaultCaps []string          `json:"default_caps,omitempty"`
	Meta        map[string]string `json:"meta,omitempty"`
}

type UpdateProjectRequest struct {
	Name        string            `json:"name,omitempty"`
	RepoPath    string            `json:"repo_path,omitempty"`
	RepoURL     string            `json:"repo_url,omitempty"`
	Description string            `json:"description,omitempty"`
	DefaultCaps []string          `json:"default_caps,omitempty"`
	Meta        map[string]string `json:"meta,omitempty"`
}

type DeleteProjectResponse struct {
	Status string `json:"status"`
}

func (p *projectsNamespace) List(ctx context.Context) (ProjectsListResponse, error) {
	var out ProjectsListResponse
	if err := apiCallInto(ctx, http.MethodGet, "/v1/projects", nil, &out); err != nil {
		return ProjectsListResponse{}, err
	}
	if out.Projects == nil {
		out.Projects = []Project{}
	}
	return out, nil
}

func (p *projectsNamespace) Create(ctx context.Context, req CreateProjectRequest) (Project, error) {
	var out Project
	if err := apiCallInto(ctx, http.MethodPost, "/v1/projects", req, &out); err != nil {
		return Project{}, err
	}
	return out, nil
}

func (p *projectsNamespace) Detail(ctx context.Context, id string) (Project, error) {
	var out Project
	if err := apiCallInto(ctx, http.MethodGet, "/v1/projects/detail?id="+url.QueryEscape(id), nil, &out); err != nil {
		return Project{}, err
	}
	return out, nil
}

func (p *projectsNamespace) Update(ctx context.Context, id string, req UpdateProjectRequest) (Project, error) {
	var out Project
	if err := apiCallInto(ctx, http.MethodPut, "/v1/projects/detail?id="+url.QueryEscape(id), req, &out); err != nil {
		return Project{}, err
	}
	return out, nil
}

func (p *projectsNamespace) Remove(ctx context.Context, id string) (DeleteProjectResponse, error) {
	var out DeleteProjectResponse
	if err := apiCallInto(ctx, http.MethodPost, "/v1/projects/remove", map[string]string{"id": id}, &out); err != nil {
		return DeleteProjectResponse{}, err
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
		State:        State,
		Reflect:      Reflect,
		Missions:     Missions,
		Scheduler:    Scheduler,
		CronSystem:   CronSystem,
		Triggers:     Triggers,
		MemoryCore:   MemoryCore,
		Graph:        Graph,
		KnowledgeKB:  KnowledgeKB,
		LoRA:         LoRA,
		Workflows:    Workflows,
		Connectors:   Connectors,
		Notify:       Notify,
		Projects:     Projects,
		Market:       SkillMarket,
		Dispatch:     Dispatch,
		Orchestrator: Orchestrator,
		Plugin:       Plugin,
		Memory:       Memory,
		AgentMemory:  AgentMemory,
		Knowledge:    Knowledge,
		Cron:         Cron,
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
