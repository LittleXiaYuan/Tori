//! # yunque-client
//!
//! Auto-generated Rust client for the [Yunque (云雀)](https://github.com/LittleXiaYuan/yunque-agent)
//! Agent HTTP API.
//!
//! - Source spec: [`docs/openapi.yaml`](../../docs/openapi.yaml)
//! - Generator: [`progenitor`](https://crates.io/crates/progenitor) (build-time)
//! - Runtime: [`reqwest`](https://crates.io/crates/reqwest) with `rustls-tls`
//!
//! ## Quick start
//!
//! ```no_run
//! # async fn run() -> Result<(), Box<dyn std::error::Error>> {
//! let client = yunque_client::Client::new("http://localhost:9090");
//! // Inspect the generated module to see all available methods.
//! # Ok(()) }
//! ```
//!
//! Re-run `cargo build` to regenerate the client whenever
//! `docs/openapi.yaml` changes (controlled by `build.rs`).

include!(concat!(env!("OUT_DIR"), "/yunque_client.rs"));

use chrono::{DateTime, Utc};
use reqwest::header::{HeaderMap, HeaderValue, AUTHORIZATION, CONTENT_TYPE};
use serde::{Deserialize, Serialize};

/// Goal tracked by the Yunque State Kernel.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct StateGoal {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub id: String,
    pub title: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub description: String,
    #[serde(default, skip_serializing_if = "is_default")]
    pub priority: i32,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub status: String,
    #[serde(default, skip_serializing_if = "is_default")]
    pub progress: f64,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub parent_goal: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub sub_goals: Vec<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub task_ids: Vec<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub created_at: Option<DateTime<Utc>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub updated_at: Option<DateTime<Utc>>,
}

/// Resource currently tracked by the Yunque State Kernel.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct StateResource {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub r#type: String,
    pub path: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub status: String,
    #[serde(default, skip_serializing_if = "std::collections::BTreeMap::is_empty")]
    pub metadata: std::collections::BTreeMap<String, String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub tracked_at: Option<DateTime<Utc>>,
}

/// Recent action recorded by the Yunque State Kernel.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct StateActionRecord {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub timestamp: Option<DateTime<Utc>>,
    pub action: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub result: String,
    #[serde(default)]
    pub success: bool,
}

/// Capability summary included in a State Kernel snapshot.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct StateCapabilities {
    #[serde(default)]
    pub total_skills: i32,
    #[serde(default)]
    pub dynamic_skills: Vec<String>,
    #[serde(default)]
    pub unresolved_gaps: i32,
    #[serde(default)]
    pub recent_gaps: Vec<String>,
}

/// Full State Kernel snapshot.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct StateSnapshot {
    #[serde(default)]
    pub goals: Vec<StateGoal>,
    #[serde(default)]
    pub resources: Vec<StateResource>,
    #[serde(default)]
    pub focus: String,
    #[serde(default)]
    pub topics: Vec<String>,
    #[serde(default)]
    pub recent_actions: Vec<StateActionRecord>,
    #[serde(default)]
    pub capabilities: StateCapabilities,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub updated_at: Option<DateTime<Utc>>,
}

/// Response returned by State Kernel goal mutations.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct StateGoalMutationResponse {
    #[serde(default)]
    pub id: String,
    pub status: String,
}

/// Filters for the reflection experience and strategy APIs.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ReflectOptions {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub q: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub source: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub category: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub outcome: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub tag: String,
    #[serde(default, skip_serializing_if = "is_default")]
    pub limit: i32,
}

/// Structured lesson captured by the Yunque reflection layer.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ReflectExperience {
    #[serde(default)]
    pub id: String,
    #[serde(default)]
    pub source: String,
    #[serde(default)]
    pub source_id: String,
    #[serde(default)]
    pub category: String,
    #[serde(default)]
    pub outcome: String,
    #[serde(default)]
    pub lesson: String,
    #[serde(default)]
    pub context: String,
    #[serde(default)]
    pub tags: Vec<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub created_at: Option<DateTime<Utc>>,
}

/// Response returned by `/v1/reflect/experiences`.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ReflectExperiencesResponse {
    #[serde(default)]
    pub experiences: Vec<ReflectExperience>,
    #[serde(default)]
    pub total: i32,
}

/// Counters returned by `/v1/reflect/experiences?stats=true`.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ReflectExperienceStats {
    #[serde(default)]
    pub total: i32,
    #[serde(default)]
    pub by_source: std::collections::BTreeMap<String, i32>,
    #[serde(default)]
    pub by_category: std::collections::BTreeMap<String, i32>,
    #[serde(default)]
    pub by_outcome: std::collections::BTreeMap<String, i32>,
    #[serde(default)]
    pub recent_7d: i32,
}

/// Response returned by `/v1/reflect/strategies`.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ReflectStrategiesResponse {
    #[serde(default)]
    pub strategies: String,
}

/// Structured task/workflow/cron/trigger draft returned by `/v1/missions/parse`.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct MissionParseResult {
    #[serde(default)]
    pub r#type: String,
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub description: String,
    #[serde(default)]
    pub config: serde_json::Value,
    #[serde(default)]
    pub confidence: f64,
    #[serde(default)]
    pub explanation: String,
}

/// Prompt scheduler job returned by `/v1/scheduler/*`.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct SchedulerJob {
    #[serde(default)]
    pub id: String,
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub tenant_id: String,
    #[serde(default)]
    pub interval: serde_json::Value,
    #[serde(default)]
    pub prompt: String,
}

/// Response returned by `/v1/scheduler/jobs`.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct SchedulerJobsResponse {
    #[serde(default)]
    pub jobs: Vec<SchedulerJob>,
    #[serde(default)]
    pub count: i32,
}

/// Request body for `/v1/scheduler/add`.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct SchedulerAddRequest {
    pub name: String,
    pub prompt: String,
    pub interval: String,
}

/// Response returned by `/v1/scheduler/remove`.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct SchedulerRemoveResponse {
    #[serde(default)]
    pub status: String,
}

/// Host cron schedule accepted by `/v1/cron/add`.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct CronSchedule {
    pub r#type: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub at: String,
    #[serde(default, skip_serializing_if = "is_default")]
    pub every_ms: i64,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub cron_expr: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub timezone: String,
}

/// Host cron payload accepted by `/v1/cron/add`.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct CronPayload {
    pub kind: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub message: String,
    #[serde(default, skip_serializing_if = "serde_json::Map::is_empty")]
    pub data: serde_json::Map<String, serde_json::Value>,
}

/// Host cron job returned by `/v1/cron/*`.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct CronJob {
    #[serde(default)]
    pub id: String,
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub schedule: CronSchedule,
    #[serde(default)]
    pub payload: CronPayload,
    #[serde(default)]
    pub agent_id: String,
    #[serde(default)]
    pub session_target: String,
    #[serde(default)]
    pub delivery: String,
    #[serde(default)]
    pub enabled: bool,
    #[serde(default)]
    pub created_at: String,
    #[serde(default)]
    pub last_run_at: String,
    #[serde(default)]
    pub next_run_at: String,
    #[serde(default)]
    pub run_count: i32,
}

/// Host cron run record returned by `/v1/cron/run`.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct CronRunRecord {
    #[serde(default)]
    pub job_id: String,
    #[serde(default)]
    pub run_id: String,
    #[serde(default)]
    pub started_at: String,
    #[serde(default)]
    pub ended_at: String,
    #[serde(default)]
    pub status: String,
    #[serde(default)]
    pub output: String,
    #[serde(default)]
    pub error: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct CronListResponse {
    #[serde(default)]
    pub jobs: Vec<CronJob>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct CronAddRequest {
    pub name: String,
    pub schedule: CronSchedule,
    pub payload: CronPayload,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct CronAddResponse {
    #[serde(default)]
    pub job: CronJob,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct CronRemoveResponse {
    #[serde(default)]
    pub deleted: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct CronRunResponse {
    #[serde(default)]
    pub run: CronRunRecord,
}

/// Host recall memory item returned by `/v1/memory/search`.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct MemoryItem {
    #[serde(default)]
    pub key: String,
    #[serde(default)]
    pub value: String,
    #[serde(default)]
    pub content: String,
    #[serde(default)]
    pub source: String,
    #[serde(default)]
    pub layer: String,
    #[serde(default)]
    pub score: f64,
    #[serde(default)]
    pub tags: Vec<String>,
    #[serde(flatten)]
    pub extra: serde_json::Map<String, serde_json::Value>,
}

pub type MemoryStatsResponse = serde_json::Map<String, serde_json::Value>;

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct MemorySearchRequest {
    pub query: String,
    #[serde(default, skip_serializing_if = "is_default")]
    pub limit: i32,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub layer: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct MemorySearchResponse {
    #[serde(default)]
    pub results: Vec<MemoryItem>,
    #[serde(default)]
    pub count: i32,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct MemoryAddRequest {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub key: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub value: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub content: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub layer: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub source: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub tags: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct MemoryAddResponse {
    #[serde(default)]
    pub status: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct MemoryCompactRequest {
    #[serde(default, skip_serializing_if = "is_default")]
    pub target_count: i32,
    #[serde(default, skip_serializing_if = "is_default")]
    pub decay_days: i32,
}

pub type MemoryCompactResponse = serde_json::Map<String, serde_json::Value>;

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct GraphEntity {
    #[serde(default)]
    pub id: String,
    pub name: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub r#type: String,
    #[serde(default, skip_serializing_if = "serde_json::Map::is_empty")]
    pub properties: serde_json::Map<String, serde_json::Value>,
    #[serde(default)]
    pub created_at: String,
    #[serde(default)]
    pub updated_at: String,
    #[serde(default)]
    pub mentions: i32,
    #[serde(flatten)]
    pub extra: serde_json::Map<String, serde_json::Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct GraphRelation {
    #[serde(default)]
    pub id: String,
    pub from_id: String,
    pub to_id: String,
    pub r#type: String,
    #[serde(default, skip_serializing_if = "is_default")]
    pub weight: f64,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub context: String,
    #[serde(default)]
    pub created_at: String,
    #[serde(flatten)]
    pub extra: serde_json::Map<String, serde_json::Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct GraphEntitiesResponse {
    #[serde(default)]
    pub entities: Vec<GraphEntity>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct GraphRelationsResponse {
    #[serde(default)]
    pub relations: Vec<GraphRelation>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct GraphDeleteEntityResponse {
    #[serde(default)]
    pub ok: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct GraphContextResponse {
    #[serde(default)]
    pub context: String,
    #[serde(default)]
    pub neighbors: Vec<serde_json::Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct GraphStatsResponse {
    #[serde(default)]
    pub entities: i32,
    #[serde(default)]
    pub relations: i32,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct KnowledgeChunk {
    #[serde(default)]
    pub id: String,
    #[serde(default)]
    pub source_id: String,
    #[serde(default)]
    pub source: String,
    #[serde(default)]
    pub file: String,
    #[serde(default)]
    pub path: String,
    #[serde(default)]
    pub lang: String,
    #[serde(default)]
    pub content: String,
    #[serde(default)]
    pub text: String,
    #[serde(default)]
    pub score: f64,
    #[serde(default)]
    pub metadata: serde_json::Map<String, serde_json::Value>,
    #[serde(flatten)]
    pub extra: serde_json::Map<String, serde_json::Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct KnowledgeSource {
    #[serde(default)]
    pub id: String,
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub r#type: String,
    #[serde(default)]
    pub path: String,
    #[serde(default)]
    pub trigger: String,
    #[serde(default)]
    pub chunks: i32,
    #[serde(default)]
    pub size: i64,
    #[serde(default)]
    pub created_at: String,
    #[serde(default)]
    pub updated_at: String,
    #[serde(default)]
    pub metadata: serde_json::Map<String, serde_json::Value>,
    #[serde(flatten)]
    pub extra: serde_json::Map<String, serde_json::Value>,
}

pub type KnowledgeStatsResponse = serde_json::Map<String, serde_json::Value>;

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct KnowledgeSearchOptions {
    pub query: String,
    #[serde(default, skip_serializing_if = "is_default")]
    pub limit: i32,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub file: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub lang: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct KnowledgeSearchResponse {
    #[serde(default)]
    pub chunks: Vec<KnowledgeChunk>,
    #[serde(default)]
    pub count: i32,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct KnowledgeSourcesResponse {
    #[serde(default)]
    pub sources: Vec<KnowledgeSource>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct KnowledgeIngestRequest {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub name: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub trigger: String,
    pub content: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct KnowledgeUpdateSourceRequest {
    pub id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub name: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub trigger: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub content: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct KnowledgeImportUrlRequest {
    pub url: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub name: String,
    #[serde(default, skip_serializing_if = "is_default")]
    pub crawl_children: bool,
    #[serde(default, skip_serializing_if = "is_default")]
    pub max_pages: i32,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct KnowledgeImportRepoRequest {
    pub path: String,
    #[serde(default, skip_serializing_if = "is_default")]
    pub max_files: i32,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct KnowledgeMutationResponse {
    #[serde(default)]
    pub source: Option<KnowledgeSource>,
    #[serde(default)]
    pub sources: Vec<KnowledgeSource>,
    #[serde(default)]
    pub stats: KnowledgeStatsResponse,
    #[serde(flatten)]
    pub extra: serde_json::Map<String, serde_json::Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct KnowledgeDeleteResponse {
    #[serde(default)]
    pub deleted: String,
    #[serde(default)]
    pub stats: KnowledgeStatsResponse,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct WorkflowDefinition {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub id: String,
    pub name: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub description: String,
    #[serde(default, skip_serializing_if = "is_default")]
    pub version: i32,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub nodes: Vec<serde_json::Value>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub edges: Vec<serde_json::Value>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub variables: Vec<serde_json::Value>,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub tenant_id: String,
    #[serde(flatten)]
    pub extra: serde_json::Map<String, serde_json::Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct WorkflowInstance {
    #[serde(default)]
    pub id: String,
    #[serde(default)]
    pub definition_id: String,
    #[serde(default)]
    pub status: String,
    #[serde(default)]
    pub variables: serde_json::Map<String, serde_json::Value>,
    #[serde(default)]
    pub tenant_id: String,
    #[serde(flatten)]
    pub extra: serde_json::Map<String, serde_json::Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct WorkflowListResponse {
    #[serde(default)]
    pub workflows: Vec<WorkflowDefinition>,
    #[serde(default)]
    pub total: i32,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct WorkflowInstancesResponse {
    #[serde(default)]
    pub instances: Vec<WorkflowInstance>,
    #[serde(default)]
    pub total: i32,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct WorkflowRunRequest {
    pub definition_id: String,
    #[serde(default, skip_serializing_if = "serde_json::Map::is_empty")]
    pub variables: serde_json::Map<String, serde_json::Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct WorkflowRunResponse {
    #[serde(default)]
    pub status: String,
    #[serde(default)]
    pub instance_id: String,
    #[serde(default)]
    pub instance: WorkflowInstance,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct WorkflowCancelRequest {
    pub instance_id: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct WorkflowCancelResponse {
    #[serde(default)]
    pub status: String,
    #[serde(default)]
    pub instance_id: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct WorkflowDeleteResponse {
    #[serde(default)]
    pub deleted: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ConnectorView {
    pub id: String,
    pub name: String,
    #[serde(default)]
    pub description: String,
    #[serde(default)]
    pub icon: String,
    #[serde(default)]
    pub category: String,
    #[serde(default)]
    pub auth_type: String,
    #[serde(default)]
    pub beta: bool,
    #[serde(default)]
    pub supported: bool,
    #[serde(default)]
    pub status: String,
    #[serde(default)]
    pub user_info: String,
    #[serde(default)]
    pub error: String,
    #[serde(default)]
    pub action_count: i32,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ConnectorAction {
    pub id: String,
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub description: String,
    #[serde(default)]
    pub params: serde_json::Value,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ConnectorDefinition {
    pub id: String,
    pub name: String,
    #[serde(default)]
    pub description: String,
    #[serde(default)]
    pub icon: String,
    #[serde(default)]
    pub category: String,
    #[serde(default)]
    pub auth_type: String,
    #[serde(default)]
    pub beta: bool,
    #[serde(default)]
    pub actions: Vec<ConnectorAction>,
    #[serde(flatten)]
    pub extra: serde_json::Map<String, serde_json::Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ConnectorListResponse {
    #[serde(default)]
    pub connectors: Vec<ConnectorView>,
    #[serde(default)]
    pub error: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ConnectorDetailResponse {
    pub connector: ConnectorDefinition,
    #[serde(default)]
    pub supported: bool,
    #[serde(default)]
    pub status: String,
    #[serde(default)]
    pub user_info: String,
    #[serde(default)]
    pub error: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ConnectorConnectRequest {
    pub connector_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub token: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub api_key: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ConnectorConnectResponse {
    #[serde(default)]
    pub ok: bool,
    #[serde(default)]
    pub status: String,
    #[serde(default)]
    pub user_info: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ConnectorOkResponse {
    #[serde(default)]
    pub ok: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ConnectorExecuteRequest {
    pub connector_id: String,
    pub action_id: String,
    #[serde(default, skip_serializing_if = "serde_json::Map::is_empty")]
    pub params: serde_json::Map<String, serde_json::Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ConnectorExecuteResponse {
    #[serde(default)]
    pub ok: bool,
    #[serde(default)]
    pub result: serde_json::Value,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct NotifyChannel {
    pub id: String,
    pub r#type: String,
    pub name: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub url: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub secret: String,
    #[serde(default)]
    pub enabled: bool,
    #[serde(flatten)]
    pub extra: serde_json::Map<String, serde_json::Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct NotifyChannelsResponse {
    #[serde(default)]
    pub channels: Vec<NotifyChannel>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct NotifyOkResponse {
    #[serde(default)]
    pub ok: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct NotifyToggleRequest {
    pub id: String,
    pub enabled: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct NotifyShareFile {
    pub name: String,
    pub path: String,
    #[serde(default, skip_serializing_if = "is_default")]
    pub size: i64,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct NotifyShareRequest {
    pub channel_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub title: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub message: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub session_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub task_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub url: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub files: Vec<NotifyShareFile>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct NotifyShareResponse {
    #[serde(default)]
    pub ok: bool,
    #[serde(default)]
    pub sent_at: String,
    #[serde(default)]
    pub share: serde_json::Map<String, serde_json::Value>,
    #[serde(default)]
    pub channel: serde_json::Map<String, serde_json::Value>,
}

pub type LoRAStatusResponse = serde_json::Map<String, serde_json::Value>;
pub type LoRAHistoryResponse = serde_json::Map<String, serde_json::Value>;
pub type LoRASummaryResponse = serde_json::Map<String, serde_json::Value>;
pub type LoRAEvolutionResponse = serde_json::Map<String, serde_json::Value>;
pub type LoRAConfigResponse = serde_json::Map<String, serde_json::Value>;
pub type LoRARollbackResponse = serde_json::Map<String, serde_json::Value>;
pub type TriggerLoRAResponse = serde_json::Map<String, serde_json::Value>;
pub type LoRAConfig = serde_json::Map<String, serde_json::Value>;

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct LoRAPreviewOptions {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub tenant_id: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct TriggerLoRARequest {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub tenant_id: String,
}

/// Triggers v2 automation definition returned by `/v1/triggers/v2`.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct TriggerDef {
    #[serde(default)]
    pub id: String,
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub tenant_id: String,
    #[serde(default)]
    pub r#type: String,
    #[serde(default)]
    pub status: String,
    #[serde(default)]
    pub actions: Vec<serde_json::Value>,
    #[serde(flatten)]
    pub extra: serde_json::Map<String, serde_json::Value>,
}

/// Filters for Triggers v2 definitions.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct TriggerListOptions {
    #[serde(default)]
    pub tenant_id: String,
    #[serde(default)]
    pub r#type: String,
    #[serde(default)]
    pub status: String,
}

/// Response returned by `/v1/triggers/v2`.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct TriggerListResponse {
    #[serde(default)]
    pub triggers: Vec<TriggerDef>,
    #[serde(default)]
    pub total: i32,
}

/// Event payload accepted by `/v1/triggers/v2/emit`.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct TriggerPayload {
    pub event: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub text: String,
    #[serde(default, skip_serializing_if = "serde_json::Map::is_empty")]
    pub data: serde_json::Map<String, serde_json::Value>,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub timestamp: String,
}

/// Response returned by `/v1/triggers/v2/emit`.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct TriggerEmitResponse {
    #[serde(default)]
    pub status: String,
    #[serde(default)]
    pub event: String,
}

/// Response returned by `DELETE /v1/triggers/v2?id=...`.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct TriggerDeleteResponse {
    #[serde(default)]
    pub deleted: String,
}

/// Filters for Triggers v2 runs and events.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct TriggerHistoryOptions {
    #[serde(default)]
    pub trigger_id: String,
    #[serde(default)]
    pub limit: i32,
}

/// Response returned by `/v1/triggers/v2/runs`.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct TriggerRunsResponse {
    #[serde(default)]
    pub runs: Vec<serde_json::Value>,
    #[serde(default)]
    pub total: i32,
}

/// Response returned by `/v1/triggers/v2/events`.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct TriggerEventsResponse {
    #[serde(default)]
    pub events: Vec<serde_json::Value>,
    #[serde(default)]
    pub total: i32,
}

/// Message accepted by the Plugin API LLM endpoint.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct PluginLLMMessage {
    pub role: String,
    pub content: String,
}

/// Request body for `/v1/plugin-api/llm`.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct PluginLLMRequest {
    pub messages: Vec<PluginLLMMessage>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub temperature: Option<f64>,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub model: String,
}

/// Response returned by `/v1/plugin-api/llm`.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct PluginLLMResponse {
    #[serde(default)]
    pub reply: String,
}

/// Search result returned by `/v1/plugin-api/search`.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct PluginSearchResult {
    #[serde(default)]
    pub title: String,
    #[serde(default)]
    pub url: String,
    #[serde(default)]
    pub snippet: String,
}

/// Response returned by `/v1/plugin-api/search`.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct PluginSearchResponse {
    #[serde(default)]
    pub results: Vec<PluginSearchResult>,
}

/// Generic `{ ok: bool }` response used by Plugin API mutation helpers.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct PluginOkResponse {
    #[serde(default)]
    pub ok: bool,
}

/// Response returned by `/v1/plugin-api/send`.
pub type PluginSendResponse = PluginOkResponse;

/// Response returned by `/v1/plugin-api/memory/get`.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct PluginMemoryValueResponse {
    #[serde(default)]
    pub value: String,
}

/// Response returned by `/v1/plugin-api/memory/list`.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct PluginMemoryListResponse {
    #[serde(default)]
    pub entries: serde_json::Value,
}

/// Response returned by `/v1/plugin-api/memory/search`.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct PluginMemorySearchResponse {
    #[serde(default)]
    pub results: Vec<String>,
}

/// Response returned by `/v1/plugin-api/agent-memory/search`.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct PluginAgentMemorySearchResponse {
    #[serde(default)]
    pub context: String,
}

/// Response returned by `/v1/plugin-api/knowledge/search`.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct PluginKnowledgeSearchResponse {
    #[serde(default)]
    pub results: Vec<serde_json::Value>,
}

/// Response returned by `/v1/plugin-api/cron/add`.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct PluginCronAddResponse {
    #[serde(default)]
    pub id: String,
    #[serde(default)]
    pub status: String,
}

/// Response returned by `/v1/plugin-api/cron/list`.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct PluginCronListResponse {
    #[serde(default)]
    pub jobs: Vec<serde_json::Value>,
}

/// Generic Plugin API system extension registration response.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct PluginExtensionRegisterResponse {
    #[serde(default)]
    pub ok: bool,
    #[serde(flatten)]
    pub extra: serde_json::Map<String, serde_json::Value>,
}

/// Response returned by `/v1/plugin-api/extensions`.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct PluginExtensionsResponse {
    #[serde(default)]
    pub extensions: Vec<serde_json::Value>,
}

/// Small Rust helper over `/v1/state` and focused State Kernel routes.
///
/// Use this when a sidecar, CLI, or plugin wants state-layer access without
/// depending on the large generated OpenAPI surface.
#[derive(Debug, Clone)]
pub struct StateClient {
    base_url: String,
    http: reqwest::Client,
}

impl StateClient {
    /// Create a StateClient using a bearer token.
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let value = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&value) {
                headers.insert(AUTHORIZATION, value);
            }
        }
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    /// Create a StateClient with a caller-provided reqwest client.
    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: trim_base_url(base_url.into()),
            http,
        }
    }

    /// Return the full State Kernel snapshot.
    pub async fn snapshot(&self) -> Result<StateSnapshot, reqwest::Error> {
        self.http
            .get(self.url("/v1/state"))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    /// Return recent State Kernel action records.
    pub async fn actions(&self) -> Result<Vec<StateActionRecord>, reqwest::Error> {
        Ok(self.snapshot().await?.recent_actions)
    }

    /// Return State Kernel capability summary.
    pub async fn capabilities(&self) -> Result<StateCapabilities, reqwest::Error> {
        Ok(self.snapshot().await?.capabilities)
    }

    /// List goals tracked by the State Kernel.
    pub async fn goals(&self) -> Result<Vec<StateGoal>, reqwest::Error> {
        self.http
            .get(self.url("/v1/state/goals"))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    /// Create or update a State Kernel goal.
    pub async fn save_goal(
        &self,
        goal: &StateGoal,
    ) -> Result<StateGoalMutationResponse, reqwest::Error> {
        self.http
            .post(self.url("/v1/state/goals"))
            .json(goal)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    /// Return the current focus string.
    pub async fn focus(&self) -> Result<String, reqwest::Error> {
        #[derive(Deserialize)]
        struct FocusResponse {
            #[serde(default)]
            focus: String,
        }
        let response: FocusResponse = self
            .http
            .get(self.url("/v1/state/focus"))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await?;
        Ok(response.focus)
    }

    /// List active resources tracked by the State Kernel.
    pub async fn resources(&self) -> Result<Vec<StateResource>, reqwest::Error> {
        self.http
            .get(self.url("/v1/state/resources"))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }
}

/// Small Rust helper over `/v1/reflect/experiences` and `/v1/reflect/strategies`.
///
/// Use this when a CLI, sidecar, plugin, or evaluation script wants to reuse
/// agent lessons and strategy hints without coupling to the full generated
/// OpenAPI surface.
#[derive(Debug, Clone)]
pub struct ReflectClient {
    base_url: String,
    http: reqwest::Client,
}

impl ReflectClient {
    /// Create a ReflectClient using a bearer token.
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let value = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&value) {
                headers.insert(AUTHORIZATION, value);
            }
        }
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    /// Create a ReflectClient with a caller-provided reqwest client.
    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: trim_base_url(base_url.into()),
            http,
        }
    }

    /// List captured reflection experiences with optional filters.
    pub async fn experiences(
        &self,
        options: &ReflectOptions,
    ) -> Result<ReflectExperiencesResponse, reqwest::Error> {
        self.http
            .get(self.url(&format!(
                "/v1/reflect/experiences{}",
                reflect_query(options, false)
            )))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    /// Return reflection counters for the same filter set.
    pub async fn stats(
        &self,
        options: &ReflectOptions,
    ) -> Result<ReflectExperienceStats, reqwest::Error> {
        self.http
            .get(self.url(&format!(
                "/v1/reflect/experiences{}",
                reflect_query(options, true)
            )))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    /// Return compiled strategy hints derived from reflection experiences.
    pub async fn strategies(&self, options: &ReflectOptions) -> Result<String, reqwest::Error> {
        let response: ReflectStrategiesResponse = self
            .http
            .get(self.url(&format!(
                "/v1/reflect/strategies{}",
                reflect_query(options, false)
            )))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await?;
        Ok(response.strategies)
    }

    fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }
}

/// Lightweight bundle of common SDK-first Rust clients.
///
/// Use this when a Rust CLI, sidecar, plugin runner, or automation binary wants
/// State Kernel, Reflection Experience, and Plugin API Runtime access from one
/// small entrypoint without coupling to the generated OpenAPI client surface.
#[derive(Debug, Clone)]
pub struct AgentKit {
    pub state: StateClient,
    pub reflect: ReflectClient,
    pub missions: MissionsClient,
    pub scheduler: SchedulerClient,
    pub cron: CronClient,
    pub triggers: TriggersClient,
    pub memory: MemoryClient,
    pub graph: GraphClient,
    pub knowledge: KnowledgeClient,
    pub lora: LoRAClient,
    pub workflows: WorkflowClient,
    pub connectors: ConnectorsClient,
    pub notify: NotifyClient,
    pub projects: ProjectsClient,
    pub market: SkillMarketClient,
    pub dispatch: DispatchClient,
    pub orchestrator: OrchestratorClient,
    pub fork: ForkClient,
    pub cost: CostClient,
    pub providers: ProvidersClient,
    pub cognis: CognisClient,
    pub trace: TraceClient,
    pub heartbeat: HeartbeatClient,
    pub events: EventsClient,
    pub runtime: RuntimeClient,
    pub subagents: SubagentsClient,
    pub tools: ToolsClient,
    pub audit: AuditClient,
    pub trust: TrustClient,
    pub iterate: IterateClient,
    pub persona: PersonaClient,
    pub emotion: EmotionClient,
    pub instructions: InstructionsClient,
    pub reactions: ReactionsClient,
    pub permissions: PermissionsClient,
    pub tasks: TasksClient,
    pub reverie: ReverieClient,
    pub realtime: RealtimeClient,
    pub chat: ChatClient,
    pub conversations: ConversationsClient,
    pub approvals: ApprovalsClient,
    pub rbac: RBACClient,
    pub files: FilesClient,
    pub browser: BrowserClient,
    pub plugin: PluginApiClient,
}

impl AgentKit {
    /// Create an AgentKit where the same bearer token is used for state,
    /// reflection, and plugin runtime calls.
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let base_url = base_url.into();
        let token = token.as_ref().to_string();
        Self::new_with_plugin_token(base_url, token.clone(), token)
    }

    /// Create an AgentKit with separate API and plugin runtime bearer tokens.
    pub fn new_with_plugin_token(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
        plugin_token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let base_url = base_url.into();
        Ok(Self {
            state: StateClient::new(base_url.clone(), token.as_ref())?,
            reflect: ReflectClient::new(base_url.clone(), token.as_ref())?,
            missions: MissionsClient::new(base_url.clone(), token.as_ref())?,
            scheduler: SchedulerClient::new(base_url.clone(), token.as_ref())?,
            cron: CronClient::new(base_url.clone(), token.as_ref())?,
            triggers: TriggersClient::new(base_url.clone(), token.as_ref())?,
            memory: MemoryClient::new(base_url.clone(), token.as_ref())?,
            graph: GraphClient::new(base_url.clone(), token.as_ref())?,
            knowledge: KnowledgeClient::new(base_url.clone(), token.as_ref())?,
            lora: LoRAClient::new(base_url.clone(), token.as_ref())?,
            workflows: WorkflowClient::new(base_url.clone(), token.as_ref())?,
            connectors: ConnectorsClient::new(base_url.clone(), token.as_ref())?,
            notify: NotifyClient::new(base_url.clone(), token.as_ref())?,
            projects: ProjectsClient::new(base_url.clone(), token.as_ref())?,
            market: SkillMarketClient::new(base_url.clone(), token.as_ref())?,
            dispatch: DispatchClient::new(base_url.clone(), token.as_ref())?,
            orchestrator: OrchestratorClient::new(base_url.clone(), token.as_ref())?,
            fork: ForkClient::new(base_url.clone(), token.as_ref())?,
            cost: CostClient::new(base_url.clone(), token.as_ref())?,
            providers: ProvidersClient::new(base_url.clone(), token.as_ref())?,
            cognis: CognisClient::new(base_url.clone(), token.as_ref())?,
            trace: TraceClient::new(base_url.clone(), token.as_ref())?,
            heartbeat: HeartbeatClient::new(base_url.clone(), token.as_ref())?,
            events: EventsClient::new(base_url.clone(), token.as_ref())?,
            runtime: RuntimeClient::new(base_url.clone(), token.as_ref())?,
            subagents: SubagentsClient::new(base_url.clone(), token.as_ref())?,
            tools: ToolsClient::new(base_url.clone(), token.as_ref())?,
            audit: AuditClient::new(base_url.clone(), token.as_ref())?,
            trust: TrustClient::new(base_url.clone(), token.as_ref())?,
            iterate: IterateClient::new(base_url.clone(), token.as_ref())?,
            persona: PersonaClient::new(base_url.clone(), token.as_ref())?,
            emotion: EmotionClient::new(base_url.clone(), token.as_ref())?,
            instructions: InstructionsClient::new(base_url.clone(), token.as_ref())?,
            reactions: ReactionsClient::new(base_url.clone(), token.as_ref())?,
            permissions: PermissionsClient::new(base_url.clone(), token.as_ref())?,
            tasks: TasksClient::new(base_url.clone(), token.as_ref())?,
            reverie: ReverieClient::new(base_url.clone(), token.as_ref())?,
            realtime: RealtimeClient::new(base_url.clone(), token.as_ref())?,
            chat: ChatClient::new(base_url.clone(), token.as_ref())?,
            conversations: ConversationsClient::new(base_url.clone(), token.as_ref())?,
            approvals: ApprovalsClient::new(base_url.clone(), token.as_ref())?,
            rbac: RBACClient::new(base_url.clone(), token.as_ref())?,
            files: FilesClient::new(base_url.clone(), token.as_ref())?,
            browser: BrowserClient::new(base_url.clone(), token.as_ref())?,
            plugin: PluginApiClient::new(base_url, plugin_token.as_ref())?,
        })
    }

    /// Create an AgentKit with caller-provided reqwest clients.
    pub fn new_with_clients(
        base_url: impl Into<String>,
        state_http: reqwest::Client,
        reflect_http: reqwest::Client,
        plugin_http: reqwest::Client,
    ) -> Self {
        let base_url = base_url.into();
        Self {
            state: StateClient::new_with_client(base_url.clone(), state_http),
            reflect: ReflectClient::new_with_client(base_url.clone(), reflect_http.clone()),
            missions: MissionsClient::new_with_client(base_url.clone(), reflect_http),
            scheduler: SchedulerClient::new_with_client(base_url.clone(), plugin_http.clone()),
            cron: CronClient::new_with_client(base_url.clone(), plugin_http.clone()),
            triggers: TriggersClient::new_with_client(base_url.clone(), plugin_http.clone()),
            memory: MemoryClient::new_with_client(base_url.clone(), plugin_http.clone()),
            graph: GraphClient::new_with_client(base_url.clone(), plugin_http.clone()),
            knowledge: KnowledgeClient::new_with_client(base_url.clone(), plugin_http.clone()),
            lora: LoRAClient::new_with_client(base_url.clone(), plugin_http.clone()),
            workflows: WorkflowClient::new_with_client(base_url.clone(), plugin_http.clone()),
            connectors: ConnectorsClient::new_with_client(base_url.clone(), plugin_http.clone()),
            notify: NotifyClient::new_with_client(base_url.clone(), plugin_http.clone()),
            projects: ProjectsClient::new_with_client(base_url.clone(), plugin_http.clone()),
            market: SkillMarketClient::new_with_client(base_url.clone(), plugin_http.clone()),
            dispatch: DispatchClient::new_with_client(base_url.clone(), plugin_http.clone()),
            orchestrator: OrchestratorClient::new_with_client(
                base_url.clone(),
                plugin_http.clone(),
            ),
            fork: ForkClient::new_with_client(base_url.clone(), plugin_http.clone()),
            cost: CostClient::new_with_client(base_url.clone(), plugin_http.clone()),
            providers: ProvidersClient::new_with_client(base_url.clone(), plugin_http.clone()),
            cognis: CognisClient::new_with_client(base_url.clone(), plugin_http.clone()),
            trace: TraceClient::new_with_client(base_url.clone(), plugin_http.clone()),
            heartbeat: HeartbeatClient::new_with_client(base_url.clone(), plugin_http.clone()),
            events: EventsClient::new_with_client(base_url.clone(), plugin_http.clone()),
            runtime: RuntimeClient::new_with_client(base_url.clone(), plugin_http.clone()),
            subagents: SubagentsClient::new_with_client(base_url.clone(), plugin_http.clone()),
            tools: ToolsClient::new_with_client(base_url.clone(), plugin_http.clone()),
            audit: AuditClient::new_with_client(base_url.clone(), plugin_http.clone()),
            trust: TrustClient::new_with_client(base_url.clone(), plugin_http.clone()),
            iterate: IterateClient::new_with_client(base_url.clone(), plugin_http.clone()),
            persona: PersonaClient::new_with_client(base_url.clone(), plugin_http.clone()),
            emotion: EmotionClient::new_with_client(base_url.clone(), plugin_http.clone()),
            instructions: InstructionsClient::new_with_client(
                base_url.clone(),
                plugin_http.clone(),
            ),
            reactions: ReactionsClient::new_with_client(base_url.clone(), plugin_http.clone()),
            permissions: PermissionsClient::new_with_client(base_url.clone(), plugin_http.clone()),
            tasks: TasksClient::new_with_client(base_url.clone(), plugin_http.clone()),
            reverie: ReverieClient::new_with_client(base_url.clone(), plugin_http.clone()),
            realtime: RealtimeClient::new_with_client(base_url.clone(), plugin_http.clone()),
            chat: ChatClient::new_with_client(base_url.clone(), plugin_http.clone()),
            conversations: ConversationsClient::new_with_client(
                base_url.clone(),
                plugin_http.clone(),
            ),
            approvals: ApprovalsClient::new_with_client(base_url.clone(), plugin_http.clone()),
            rbac: RBACClient::new_with_client(base_url.clone(), plugin_http.clone()),
            files: FilesClient::new_with_client(base_url.clone(), plugin_http.clone()),
            browser: BrowserClient::new_with_client(base_url.clone(), plugin_http.clone()),
            plugin: PluginApiClient::new_with_client(base_url, plugin_http),
        }
    }
}

/// Small Rust helper over host `/v1/cron/*` scheduled task endpoints.
#[derive(Debug, Clone)]
pub struct CronClient {
    base_url: String,
    http: reqwest::Client,
}

impl CronClient {
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let value = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&value) {
                headers.insert(AUTHORIZATION, value);
            }
        }
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: trim_base_url(base_url.into()),
            http,
        }
    }

    pub async fn list(&self) -> Result<CronListResponse, reqwest::Error> {
        self.http
            .get(self.url("/v1/cron/list"))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn add(&self, request: &CronAddRequest) -> Result<CronAddResponse, reqwest::Error> {
        self.http
            .post(self.url("/v1/cron/add"))
            .json(request)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn remove(&self, id: impl AsRef<str>) -> Result<CronRemoveResponse, reqwest::Error> {
        self.http
            .post(self.url(&format!("/v1/cron/remove?id={}", id.as_ref())))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn run(&self, id: impl AsRef<str>) -> Result<CronRunResponse, reqwest::Error> {
        self.http
            .post(self.url(&format!("/v1/cron/run?id={}", id.as_ref())))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }
}

/// Small Rust helper over host `/v1/memory/*` recall memory endpoints.
#[derive(Debug, Clone)]
pub struct MemoryClient {
    base_url: String,
    http: reqwest::Client,
}

impl MemoryClient {
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let value = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&value) {
                headers.insert(AUTHORIZATION, value);
            }
        }
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: trim_base_url(base_url.into()),
            http,
        }
    }

    pub async fn stats(&self) -> Result<MemoryStatsResponse, reqwest::Error> {
        self.http
            .get(self.url("/v1/memory/stats"))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn search(
        &self,
        request: &MemorySearchRequest,
    ) -> Result<MemorySearchResponse, reqwest::Error> {
        self.http
            .post(self.url("/v1/memory/search"))
            .json(request)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn add(
        &self,
        request: &MemoryAddRequest,
    ) -> Result<MemoryAddResponse, reqwest::Error> {
        let mut request = request.clone();
        if request.value.is_empty() {
            request.value = request.content.clone();
        }
        self.http
            .post(self.url("/v1/memory/add"))
            .json(&request)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn compact(
        &self,
        request: &MemoryCompactRequest,
    ) -> Result<MemoryCompactResponse, reqwest::Error> {
        self.http
            .post(self.url("/v1/memory/compact"))
            .json(request)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }
}

/// Small Rust helper over host `/v1/graph/*` knowledge graph endpoints.
#[derive(Debug, Clone)]
pub struct GraphClient {
    base_url: String,
    http: reqwest::Client,
}

impl GraphClient {
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let value = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&value) {
                headers.insert(AUTHORIZATION, value);
            }
        }
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: trim_base_url(base_url.into()),
            http,
        }
    }

    pub async fn entities(
        &self,
        query: impl AsRef<str>,
    ) -> Result<GraphEntitiesResponse, reqwest::Error> {
        let query = query.as_ref();
        let path = if query.is_empty() {
            "/v1/graph/entities".to_string()
        } else {
            format!("/v1/graph/entities?q={}", url_encode_query_component(query))
        };
        self.http
            .get(self.url(&path))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn put_entity(&self, entity: &GraphEntity) -> Result<GraphEntity, reqwest::Error> {
        self.http
            .post(self.url("/v1/graph/entities"))
            .json(entity)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn delete_entity(
        &self,
        id: impl AsRef<str>,
    ) -> Result<GraphDeleteEntityResponse, reqwest::Error> {
        self.http
            .delete(self.url(&format!(
                "/v1/graph/entities?id={}",
                url_encode_query_component(id.as_ref())
            )))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn relations(
        &self,
        entity_id: impl AsRef<str>,
    ) -> Result<GraphRelationsResponse, reqwest::Error> {
        let entity_id = entity_id.as_ref();
        let path = if entity_id.is_empty() {
            "/v1/graph/relations".to_string()
        } else {
            format!(
                "/v1/graph/relations?entity_id={}",
                url_encode_query_component(entity_id)
            )
        };
        self.http
            .get(self.url(&path))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn put_relation(
        &self,
        relation: &GraphRelation,
    ) -> Result<GraphRelation, reqwest::Error> {
        self.http
            .post(self.url("/v1/graph/relations"))
            .json(relation)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn context_by_entity_id(
        &self,
        entity_id: impl AsRef<str>,
    ) -> Result<GraphContextResponse, reqwest::Error> {
        self.http
            .get(self.url(&format!(
                "/v1/graph/context?entity_id={}",
                url_encode_query_component(entity_id.as_ref())
            )))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn context_by_name(
        &self,
        name: impl AsRef<str>,
    ) -> Result<GraphContextResponse, reqwest::Error> {
        self.http
            .get(self.url(&format!(
                "/v1/graph/context?name={}",
                url_encode_query_component(name.as_ref())
            )))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn stats(&self) -> Result<GraphStatsResponse, reqwest::Error> {
        self.http
            .get(self.url("/v1/graph/stats"))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }
}

/// Small Rust helper over host `/v1/knowledge/*` RAG endpoints.
#[derive(Debug, Clone)]
pub struct KnowledgeClient {
    base_url: String,
    http: reqwest::Client,
}

impl KnowledgeClient {
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let value = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&value) {
                headers.insert(AUTHORIZATION, value);
            }
        }
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: trim_base_url(base_url.into()),
            http,
        }
    }

    pub async fn stats(&self) -> Result<KnowledgeStatsResponse, reqwest::Error> {
        self.http
            .get(self.url("/v1/knowledge/stats"))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn sources(&self) -> Result<KnowledgeSourcesResponse, reqwest::Error> {
        self.http
            .get(self.url("/v1/knowledge/sources"))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn search(
        &self,
        options: &KnowledgeSearchOptions,
    ) -> Result<KnowledgeSearchResponse, reqwest::Error> {
        self.http
            .get(self.url(&knowledge_search_query(options)))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn ingest(
        &self,
        request: &KnowledgeIngestRequest,
    ) -> Result<KnowledgeMutationResponse, reqwest::Error> {
        self.http
            .post(self.url("/v1/knowledge/ingest"))
            .json(request)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn update_source(
        &self,
        request: &KnowledgeUpdateSourceRequest,
    ) -> Result<KnowledgeMutationResponse, reqwest::Error> {
        self.http
            .post(self.url("/v1/knowledge/source/update"))
            .json(request)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn delete_source(
        &self,
        id: impl AsRef<str>,
    ) -> Result<KnowledgeDeleteResponse, reqwest::Error> {
        self.http
            .delete(self.url(&format!(
                "/v1/knowledge/source?id={}",
                url_encode_query_component(id.as_ref())
            )))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn import_url(
        &self,
        request: &KnowledgeImportUrlRequest,
    ) -> Result<KnowledgeMutationResponse, reqwest::Error> {
        self.http
            .post(self.url("/v1/knowledge/import-url"))
            .json(request)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn import_repo(
        &self,
        request: &KnowledgeImportRepoRequest,
    ) -> Result<KnowledgeMutationResponse, reqwest::Error> {
        self.http
            .post(self.url("/v1/knowledge/import-repo"))
            .json(request)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }
}

/// Small Rust helper over host `/v1/workflows*` DAG orchestration endpoints.
#[derive(Debug, Clone)]
pub struct WorkflowClient {
    base_url: String,
    http: reqwest::Client,
}

impl WorkflowClient {
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let value = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&value) {
                headers.insert(AUTHORIZATION, value);
            }
        }
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: trim_base_url(base_url.into()),
            http,
        }
    }

    pub async fn list(&self) -> Result<WorkflowListResponse, reqwest::Error> {
        self.http
            .get(self.url("/v1/workflows"))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn get(&self, id: impl AsRef<str>) -> Result<WorkflowDefinition, reqwest::Error> {
        self.http
            .get(self.url(&format!(
                "/v1/workflows?id={}",
                url_encode_query_component(id.as_ref())
            )))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn save(
        &self,
        definition: &WorkflowDefinition,
    ) -> Result<WorkflowDefinition, reqwest::Error> {
        self.http
            .post(self.url("/v1/workflows"))
            .json(definition)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn delete(
        &self,
        id: impl AsRef<str>,
    ) -> Result<WorkflowDeleteResponse, reqwest::Error> {
        self.http
            .delete(self.url(&format!(
                "/v1/workflows?id={}",
                url_encode_query_component(id.as_ref())
            )))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn run(
        &self,
        request: &WorkflowRunRequest,
    ) -> Result<WorkflowRunResponse, reqwest::Error> {
        self.http
            .post(self.url("/v1/workflows/run"))
            .json(request)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn instances(&self) -> Result<WorkflowInstancesResponse, reqwest::Error> {
        self.http
            .get(self.url("/v1/workflows/instances"))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn get_instance(
        &self,
        id: impl AsRef<str>,
    ) -> Result<WorkflowInstance, reqwest::Error> {
        self.http
            .get(self.url(&format!(
                "/v1/workflows/instances?id={}",
                url_encode_query_component(id.as_ref())
            )))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn cancel(
        &self,
        request: &WorkflowCancelRequest,
    ) -> Result<WorkflowCancelResponse, reqwest::Error> {
        self.http
            .post(self.url("/v1/workflows/cancel"))
            .json(request)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }
}

/// External MCP worker record returned by `/v1/workers*`.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct DispatchWorker {
    #[serde(default)]
    pub id: String,
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub r#type: String,
    #[serde(default)]
    pub capabilities: Vec<String>,
    #[serde(default)]
    pub status: String,
    #[serde(default)]
    pub last_seen: String,
    #[serde(flatten)]
    pub extra: serde_json::Map<String, serde_json::Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct DispatchWorkersResponse {
    #[serde(default)]
    pub workers: Vec<DispatchWorker>,
    #[serde(default)]
    pub count: i32,
}

pub type DispatchQueueResponse = serde_json::Map<String, serde_json::Value>;

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct DispatchEnqueueRequest {
    pub task_id: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub capabilities: Vec<String>,
    #[serde(default, skip_serializing_if = "is_default")]
    pub priority: i32,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct DispatchEnqueueResponse {
    pub task_id: String,
    pub status: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct DispatchWorkerConfigResponse {
    pub r#type: String,
    pub mcp_config: String,
    pub instructions: String,
    pub server_url: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct DispatchStatusResponse {
    pub status: String,
}

/// Small Rust helper over host `/v1/workers*` and `/v1/dispatch/*` endpoints.
#[derive(Debug, Clone)]
pub struct DispatchClient {
    base_url: String,
    http: reqwest::Client,
}

impl DispatchClient {
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let value = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&value) {
                headers.insert(AUTHORIZATION, value);
            }
        }
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: trim_base_url(base_url.into()),
            http,
        }
    }

    pub fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }

    pub async fn workers(&self) -> Result<DispatchWorkersResponse, reqwest::Error> {
        self.http
            .get(self.url("/v1/workers"))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn worker(&self, id: &str) -> Result<DispatchWorker, reqwest::Error> {
        self.http
            .get(self.url(&format!(
                "/v1/workers/detail?id={}",
                url_encode_query_component(id)
            )))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn remove_worker(&self, id: &str) -> Result<DispatchStatusResponse, reqwest::Error> {
        self.http
            .post(self.url("/v1/workers/remove"))
            .json(&serde_json::json!({ "id": id }))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn queue(&self) -> Result<DispatchQueueResponse, reqwest::Error> {
        self.http
            .get(self.url("/v1/dispatch/queue"))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn enqueue(
        &self,
        request: &DispatchEnqueueRequest,
    ) -> Result<DispatchEnqueueResponse, reqwest::Error> {
        self.http
            .post(self.url("/v1/dispatch/enqueue"))
            .json(request)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn worker_config(
        &self,
        worker_type: &str,
    ) -> Result<DispatchWorkerConfigResponse, reqwest::Error> {
        let path = if worker_type.is_empty() {
            "/v1/workers/config".to_string()
        } else {
            format!(
                "/v1/workers/config?type={}",
                url_encode_query_component(worker_type)
            )
        };
        self.http
            .get(self.url(&path))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }
}

/// Policy map returned by `/v1/orchestrator/policy`.
pub type OrchestratorPolicy = serde_json::Map<String, serde_json::Value>;

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct OrchestratorStatusResponse {
    #[serde(default)]
    pub running: bool,
    #[serde(default)]
    pub adapters: Vec<String>,
    #[serde(default)]
    pub active_sessions: i32,
    #[serde(default)]
    pub policy: OrchestratorPolicy,
    #[serde(default)]
    pub event_count: i32,
    #[serde(flatten)]
    pub extra: serde_json::Map<String, serde_json::Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct OrchestratorToggleResponse {
    pub status: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct OrchestratorSession {
    pub session_id: String,
    pub adapter: String,
    pub task_id: String,
    #[serde(default)]
    pub started_at: String,
    #[serde(flatten)]
    pub extra: serde_json::Map<String, serde_json::Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct OrchestratorSessionsResponse {
    #[serde(default)]
    pub sessions: Vec<OrchestratorSession>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct OrchestratorIDE {
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub path: String,
    #[serde(default)]
    pub available: bool,
    #[serde(default)]
    pub version: String,
    #[serde(flatten)]
    pub extra: serde_json::Map<String, serde_json::Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct OrchestratorDetectResponse {
    #[serde(default)]
    pub ides: Vec<OrchestratorIDE>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct OrchestratorEvent {
    pub id: String,
    pub r#type: String,
    #[serde(default)]
    pub task_id: String,
    #[serde(default)]
    pub worker_id: String,
    #[serde(default)]
    pub session_id: String,
    #[serde(default)]
    pub message: String,
    #[serde(default)]
    pub meta: serde_json::Map<String, serde_json::Value>,
    #[serde(default)]
    pub timestamp: String,
    #[serde(flatten)]
    pub extra: serde_json::Map<String, serde_json::Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct OrchestratorEventsResponse {
    #[serde(default)]
    pub events: Vec<OrchestratorEvent>,
    #[serde(default)]
    pub total: i32,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct OrchestratorTaskTimelineResponse {
    pub task_id: String,
    #[serde(default)]
    pub events: Vec<OrchestratorEvent>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct OrchestratorPolicyUpdateResponse {
    pub status: String,
    #[serde(default)]
    pub policy: OrchestratorPolicy,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct OrchestratorAdapterConfig {
    pub adapter_name: String,
    pub binary: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub launch_args: String,
    pub mcp_config_path: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub rules_file_path: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub lifecycle: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct OrchestratorAdapterResponse {
    pub status: String,
    pub name: String,
    #[serde(default)]
    pub available: bool,
}

/// Small Rust helper over host `/v1/orchestrator/*` IDE worker daemon endpoints.
#[derive(Debug, Clone)]
pub struct OrchestratorClient {
    base_url: String,
    http: reqwest::Client,
}

impl OrchestratorClient {
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let value = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&value) {
                headers.insert(AUTHORIZATION, value);
            }
        }
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: trim_base_url(base_url.into()),
            http,
        }
    }

    pub fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }

    pub async fn status(&self) -> Result<OrchestratorStatusResponse, reqwest::Error> {
        self.http
            .get(self.url("/v1/orchestrator/status"))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn toggle(&self, action: &str) -> Result<OrchestratorToggleResponse, reqwest::Error> {
        self.http
            .post(self.url("/v1/orchestrator/toggle"))
            .json(&serde_json::json!({ "action": action }))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn sessions(&self) -> Result<OrchestratorSessionsResponse, reqwest::Error> {
        self.http
            .get(self.url("/v1/orchestrator/sessions"))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn detect_ides(&self) -> Result<OrchestratorDetectResponse, reqwest::Error> {
        self.http
            .get(self.url("/v1/orchestrator/detect"))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn events(&self, limit: i32) -> Result<OrchestratorEventsResponse, reqwest::Error> {
        let path = if limit > 0 {
            format!("/v1/orchestrator/events?limit={limit}")
        } else {
            "/v1/orchestrator/events".to_string()
        };
        self.http
            .get(self.url(&path))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn task_timeline(
        &self,
        task_id: &str,
    ) -> Result<OrchestratorTaskTimelineResponse, reqwest::Error> {
        self.http
            .get(self.url(&format!(
                "/v1/orchestrator/events/task?task_id={}",
                url_encode_query_component(task_id)
            )))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn policy(&self) -> Result<OrchestratorPolicy, reqwest::Error> {
        self.http
            .get(self.url("/v1/orchestrator/policy"))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn update_policy(
        &self,
        policy: &OrchestratorPolicy,
    ) -> Result<OrchestratorPolicyUpdateResponse, reqwest::Error> {
        self.http
            .put(self.url("/v1/orchestrator/policy"))
            .json(policy)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn add_adapter(
        &self,
        config: &OrchestratorAdapterConfig,
    ) -> Result<OrchestratorAdapterResponse, reqwest::Error> {
        self.http
            .post(self.url("/v1/orchestrator/adapters/add"))
            .json(config)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ForkMessage {
    pub role: String,
    pub content: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub timestamp: String,
    #[serde(flatten)]
    pub extra: serde_json::Map<String, serde_json::Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ConversationFork {
    pub id: String,
    #[serde(default)]
    pub parent_id: String,
    pub session_id: String,
    #[serde(default)]
    pub label: String,
    #[serde(default)]
    pub messages: Vec<ForkMessage>,
    pub created_at: String,
    #[serde(default)]
    pub children: Vec<String>,
    #[serde(flatten)]
    pub extra: serde_json::Map<String, serde_json::Value>,
}

pub type ForkRootResponse = serde_json::Map<String, serde_json::Value>;

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ForkCreateRequest {
    pub session_id: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub messages: Vec<ForkMessage>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ForkBranchRequest {
    pub fork_id: String,
    pub at_index: i32,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub label: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ForkDeleteResponse {
    pub deleted: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ForkListResponse {
    #[serde(default)]
    pub forks: Vec<ConversationFork>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ModelsResponse {
    #[serde(default)]
    pub models: Vec<serde_json::Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ProvidersResponse {
    #[serde(default)]
    pub providers: Vec<serde_json::Value>,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub mode: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub warning: String,
}

pub type ProviderConfig = serde_json::Value;
pub type ModelEntry = serde_json::Value;
pub type ProviderActionResponse = serde_json::Value;
pub type ProviderTestResponse = serde_json::Value;
pub type ProviderModeResponse = serde_json::Value;
pub type ProviderPresetsResponse = serde_json::Value;
pub type ExecProviderResponse = serde_json::Value;
pub type ToriDiscoverResponse = serde_json::Value;

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ProviderSessionOverrideRequest {
    pub session_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub provider_id: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct LocalDiscoverRequest {
    pub base_url: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct LocalRegisterRequest {
    pub base_url: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub model: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub tier: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub backend: String,
}

/// Small Rust helper over host `/api/providers*`, `/v1/models`, and provider breaker endpoints.
#[derive(Debug, Clone)]
pub struct ProvidersClient {
    base_url: String,
    http: reqwest::Client,
}

impl ProvidersClient {
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let value = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&value) {
                headers.insert(AUTHORIZATION, value);
            }
        }
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: trim_base_url(base_url.into()),
            http,
        }
    }

    pub fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }

    pub async fn models(&self) -> Result<ModelsResponse, reqwest::Error> {
        self.get_json("/v1/models").await
    }
    pub async fn add_model(&self, model: &ModelEntry) -> Result<ModelEntry, reqwest::Error> {
        self.post_json("/v1/models", model).await
    }
    pub async fn delete_model(&self, id: &str) -> Result<ProviderActionResponse, reqwest::Error> {
        self.delete_json(&format!("/v1/models?id={}", url_encode_query_component(id)))
            .await
    }
    pub async fn list(&self) -> Result<ProvidersResponse, reqwest::Error> {
        self.get_json("/api/providers").await
    }
    pub async fn test(&self, id: &str) -> Result<ProviderTestResponse, reqwest::Error> {
        self.post_json("/api/providers/test", &serde_json::json!({"id": id}))
            .await
    }
    pub async fn enable(&self, id: &str) -> Result<ProviderActionResponse, reqwest::Error> {
        self.post_json("/api/providers/enable", &serde_json::json!({"id": id}))
            .await
    }
    pub async fn disable(&self, id: &str) -> Result<ProviderActionResponse, reqwest::Error> {
        self.post_json("/api/providers/disable", &serde_json::json!({"id": id}))
            .await
    }
    pub async fn switch_model(
        &self,
        id: &str,
        model: &str,
    ) -> Result<ProviderActionResponse, reqwest::Error> {
        self.post_json(
            "/api/providers/switch-model",
            &serde_json::json!({"id": id, "model": model}),
        )
        .await
    }
    pub async fn set_session(
        &self,
        request: &ProviderSessionOverrideRequest,
    ) -> Result<ProviderActionResponse, reqwest::Error> {
        self.post_json("/api/providers/session", request).await
    }
    pub async fn mode(&self) -> Result<ProviderModeResponse, reqwest::Error> {
        self.get_json("/api/providers/mode").await
    }
    pub async fn set_mode(&self, mode: &str) -> Result<ProviderModeResponse, reqwest::Error> {
        self.post_json("/api/providers/mode", &serde_json::json!({"mode": mode}))
            .await
    }
    pub async fn presets(&self) -> Result<ProviderPresetsResponse, reqwest::Error> {
        self.get_json("/api/providers/presets").await
    }
    pub async fn register(
        &self,
        config: &ProviderConfig,
    ) -> Result<ProviderActionResponse, reqwest::Error> {
        self.post_json("/api/providers/register", config).await
    }
    pub async fn delete(&self, id: &str) -> Result<ProviderActionResponse, reqwest::Error> {
        self.post_json("/api/providers/delete", &serde_json::json!({"id": id}))
            .await
    }
    pub async fn discover_local(
        &self,
        request: &LocalDiscoverRequest,
    ) -> Result<ProviderActionResponse, reqwest::Error> {
        self.post_json("/api/providers/local/discover", request)
            .await
    }
    pub async fn register_local(
        &self,
        request: &LocalRegisterRequest,
    ) -> Result<ProviderActionResponse, reqwest::Error> {
        self.post_json("/api/providers/local/register", request)
            .await
    }
    pub async fn discover_tori(
        &self,
        auto_register: bool,
    ) -> Result<ToriDiscoverResponse, reqwest::Error> {
        let suffix = if auto_register {
            "?auto_register=true"
        } else {
            ""
        };
        self.post_json(
            &format!("/api/providers/tori/discover{}", suffix),
            &serde_json::json!({}),
        )
        .await
    }
    pub async fn exec(&self) -> Result<ExecProviderResponse, reqwest::Error> {
        self.get_json("/api/providers/exec").await
    }
    pub async fn set_exec(
        &self,
        provider_id: &str,
    ) -> Result<ExecProviderResponse, reqwest::Error> {
        self.post_json(
            "/api/providers/exec",
            &serde_json::json!({"provider_id": provider_id}),
        )
        .await
    }
    pub async fn reset_breakers(&self) -> Result<ProviderActionResponse, reqwest::Error> {
        self.post_json("/api/breaker/reset", &serde_json::json!({}))
            .await
    }

    async fn get_json<T>(&self, path: &str) -> Result<T, reqwest::Error>
    where
        T: for<'de> Deserialize<'de>,
    {
        self.http
            .get(self.url(path))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }
    async fn post_json<B, T>(&self, path: &str, body: &B) -> Result<T, reqwest::Error>
    where
        B: Serialize + ?Sized,
        T: for<'de> Deserialize<'de>,
    {
        self.http
            .post(self.url(path))
            .json(body)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }
    async fn delete_json<T>(&self, path: &str) -> Result<T, reqwest::Error>
    where
        T: for<'de> Deserialize<'de>,
    {
        self.http
            .delete(self.url(path))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }
}

pub type CogniDeclaration = serde_json::Map<String, serde_json::Value>;
pub type CogniListResponse = serde_json::Value;
pub type CogniMutationResponse = serde_json::Value;
pub type CogniTraceResponse = serde_json::Value;
pub type CogniStatsResponse = serde_json::Value;
pub type CogniHealthResponse = serde_json::Value;
pub type CogniAlertsResponse = serde_json::Value;
pub type CogniVerifyResponse = serde_json::Value;
pub type CogniExperienceResponse = serde_json::Value;
pub type CogniWorkflowRunRequest = serde_json::Map<String, serde_json::Value>;
pub type CogniExperienceRecordRequest = serde_json::Map<String, serde_json::Value>;

/// Small Rust helper over Cogni registry, trace, experience, evolution, and federation endpoints.
#[derive(Debug, Clone)]
pub struct CognisClient {
    base_url: String,
    http: reqwest::Client,
}

impl CognisClient {
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let value = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&value) {
                headers.insert(AUTHORIZATION, value);
            }
        }
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: trim_base_url(base_url.into()),
            http,
        }
    }

    pub fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }

    pub async fn list(&self) -> Result<CogniListResponse, reqwest::Error> {
        self.get_json("/v1/cognis").await
    }
    pub async fn create(
        &self,
        declaration: &CogniDeclaration,
    ) -> Result<CogniDeclaration, reqwest::Error> {
        self.post_json("/v1/cognis", declaration).await
    }
    pub async fn get(&self, id: &str) -> Result<CogniDeclaration, reqwest::Error> {
        self.get_json(&format!("/v1/cognis/{}", url_encode_query_component(id)))
            .await
    }
    pub async fn remove(&self, id: &str) -> Result<CogniMutationResponse, reqwest::Error> {
        self.delete_json(&format!("/v1/cognis/{}", url_encode_query_component(id)))
            .await
    }
    pub async fn enable(&self, id: &str) -> Result<CogniMutationResponse, reqwest::Error> {
        self.post_id(id, "enable", &serde_json::json!({})).await
    }
    pub async fn disable(&self, id: &str) -> Result<CogniMutationResponse, reqwest::Error> {
        self.post_id(id, "disable", &serde_json::json!({})).await
    }
    pub async fn reload(&self) -> Result<CogniMutationResponse, reqwest::Error> {
        self.post_json("/v1/cognis/reload", &serde_json::json!({}))
            .await
    }
    pub async fn traces(&self, limit: i32) -> Result<CogniTraceResponse, reqwest::Error> {
        let suffix = if limit > 0 {
            format!("?limit={limit}")
        } else {
            String::new()
        };
        self.get_json(&format!("/v1/cognis/traces{}", suffix)).await
    }
    pub async fn trace(&self, id: &str, limit: i32) -> Result<CogniTraceResponse, reqwest::Error> {
        let suffix = if limit > 0 {
            format!("?limit={limit}")
        } else {
            String::new()
        };
        self.get_json(&format!(
            "/v1/cognis/{}/trace{}",
            url_encode_query_component(id),
            suffix
        ))
        .await
    }
    pub async fn stats(&self) -> Result<CogniStatsResponse, reqwest::Error> {
        self.get_json("/v1/cognis/stats").await
    }
    pub async fn health(&self, id: Option<&str>) -> Result<CogniHealthResponse, reqwest::Error> {
        match id {
            Some(id) if !id.is_empty() => {
                self.get_json(&format!(
                    "/v1/cognis/{}/health",
                    url_encode_query_component(id)
                ))
                .await
            }
            _ => self.get_json("/v1/cognis/health").await,
        }
    }
    pub async fn verify(&self, id: Option<&str>) -> Result<CogniVerifyResponse, reqwest::Error> {
        match id {
            Some(id) if !id.is_empty() => {
                self.get_json(&format!(
                    "/v1/cognis/{}/verify",
                    url_encode_query_component(id)
                ))
                .await
            }
            _ => self.get_json("/v1/cognis/verify").await,
        }
    }
    pub async fn alerts(&self) -> Result<CogniAlertsResponse, reqwest::Error> {
        self.get_json("/v1/cognis/alerts").await
    }
    pub async fn scan_alerts(&self) -> Result<CogniAlertsResponse, reqwest::Error> {
        self.post_json("/v1/cognis/alerts/scan", &serde_json::json!({}))
            .await
    }
    pub async fn generate(
        &self,
        request: &serde_json::Value,
    ) -> Result<CogniMutationResponse, reqwest::Error> {
        self.post_json("/v1/cognis/generate", request).await
    }
    pub async fn export_bundle(&self) -> Result<serde_json::Value, reqwest::Error> {
        self.get_json("/v1/cognis/export").await
    }
    pub async fn import_bundle(
        &self,
        bundle: &serde_json::Value,
    ) -> Result<CogniMutationResponse, reqwest::Error> {
        self.post_json("/v1/cognis/import", bundle).await
    }
    pub async fn workflows(&self, id: &str) -> Result<serde_json::Value, reqwest::Error> {
        self.get_json(&format!(
            "/v1/cognis/{}/workflows",
            url_encode_query_component(id)
        ))
        .await
    }
    pub async fn run_workflow(
        &self,
        id: &str,
        workflow: &str,
        request: &CogniWorkflowRunRequest,
    ) -> Result<serde_json::Value, reqwest::Error> {
        self.post_json(
            &format!(
                "/v1/cognis/{}/workflow/{}",
                url_encode_query_component(id),
                url_encode_query_component(workflow)
            ),
            request,
        )
        .await
    }
    pub async fn experience(&self, id: &str) -> Result<CogniExperienceResponse, reqwest::Error> {
        self.get_json(&format!(
            "/v1/cognis/{}/experience",
            url_encode_query_component(id)
        ))
        .await
    }
    pub async fn record_experience(
        &self,
        id: &str,
        request: &CogniExperienceRecordRequest,
    ) -> Result<CogniMutationResponse, reqwest::Error> {
        self.post_json(
            &format!(
                "/v1/cognis/{}/experience/record",
                url_encode_query_component(id)
            ),
            request,
        )
        .await
    }
    pub async fn confirm_experience_pattern(
        &self,
        id: &str,
        pattern_id: &str,
    ) -> Result<CogniMutationResponse, reqwest::Error> {
        self.post_json(
            &format!(
                "/v1/cognis/{}/experience/patterns/{}/confirm",
                url_encode_query_component(id),
                url_encode_query_component(pattern_id)
            ),
            &serde_json::json!({}),
        )
        .await
    }
    pub async fn evolve(
        &self,
        id: &str,
        request: &serde_json::Value,
    ) -> Result<CogniMutationResponse, reqwest::Error> {
        self.post_id(id, "evolve", request).await
    }
    pub async fn evolution(&self, id: Option<&str>) -> Result<serde_json::Value, reqwest::Error> {
        match id {
            Some(id) if !id.is_empty() => {
                self.get_json(&format!(
                    "/v1/cognis/{}/evolution",
                    url_encode_query_component(id)
                ))
                .await
            }
            _ => self.get_json("/v1/cognis/evolution").await,
        }
    }
    pub async fn federation(&self) -> Result<serde_json::Value, reqwest::Error> {
        self.get_json("/v1/cognis/federation").await
    }
    pub async fn federation_peers(&self) -> Result<serde_json::Value, reqwest::Error> {
        self.get_json("/v1/cognis/federation/peers").await
    }
    pub async fn discover_federation(
        &self,
        request: &serde_json::Value,
    ) -> Result<serde_json::Value, reqwest::Error> {
        self.post_json("/v1/cognis/federation/discover", request)
            .await
    }
    pub async fn expose(&self, id: &str) -> Result<CogniMutationResponse, reqwest::Error> {
        self.post_id(id, "expose", &serde_json::json!({})).await
    }
    pub async fn unexpose(&self, id: &str) -> Result<CogniMutationResponse, reqwest::Error> {
        self.post_id(id, "unexpose", &serde_json::json!({})).await
    }
    pub async fn economics(&self) -> Result<serde_json::Value, reqwest::Error> {
        self.get_json("/v1/cognis/economics").await
    }
    async fn post_id<B, T>(&self, id: &str, action: &str, body: &B) -> Result<T, reqwest::Error>
    where
        B: Serialize + ?Sized,
        T: for<'de> Deserialize<'de>,
    {
        self.post_json(
            &format!("/v1/cognis/{}/{}", url_encode_query_component(id), action),
            body,
        )
        .await
    }
    async fn get_json<T>(&self, path: &str) -> Result<T, reqwest::Error>
    where
        T: for<'de> Deserialize<'de>,
    {
        self.http
            .get(self.url(path))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }
    async fn post_json<B, T>(&self, path: &str, body: &B) -> Result<T, reqwest::Error>
    where
        B: Serialize + ?Sized,
        T: for<'de> Deserialize<'de>,
    {
        self.http
            .post(self.url(path))
            .json(body)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }
    async fn delete_json<T>(&self, path: &str) -> Result<T, reqwest::Error>
    where
        T: for<'de> Deserialize<'de>,
    {
        self.http
            .delete(self.url(path))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }
}

pub type TraceEvent = serde_json::Value;

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct TraceEventsResponse {
    #[serde(default)]
    pub count: i32,
    #[serde(default)]
    pub raw: bool,
    #[serde(default)]
    pub events: Vec<TraceEvent>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct TraceByIdResponse {
    #[serde(default)]
    pub trace_id: String,
    #[serde(default)]
    pub count: i32,
    #[serde(default)]
    pub raw: bool,
    #[serde(default)]
    pub events: Vec<TraceEvent>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct TraceByTaskResponse {
    #[serde(default)]
    pub task_id: String,
    #[serde(default)]
    pub count: i32,
    #[serde(default)]
    pub raw: bool,
    #[serde(default)]
    pub events: Vec<TraceEvent>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct TraceRecentQuery {
    #[serde(default, skip_serializing_if = "is_default")]
    pub limit: i32,
    #[serde(default, skip_serializing_if = "is_default")]
    pub raw: bool,
}

/// Small Rust helper over `/v1/trace/*` execution/audit trace read endpoints.
#[derive(Debug, Clone)]
pub struct TraceClient {
    base_url: String,
    http: reqwest::Client,
}

impl TraceClient {
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let value = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&value) {
                headers.insert(AUTHORIZATION, value);
            }
        }
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: trim_base_url(base_url.into()),
            http,
        }
    }

    pub fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }

    pub async fn recent(
        &self,
        query: &TraceRecentQuery,
    ) -> Result<TraceEventsResponse, reqwest::Error> {
        let mut pairs = Vec::new();
        if query.limit > 0 {
            pairs.push(format!("limit={}", query.limit));
        }
        if query.raw {
            pairs.push("raw=true".to_string());
        }
        let suffix = if pairs.is_empty() {
            String::new()
        } else {
            format!("?{}", pairs.join("&"))
        };
        self.get_json(&format!("/v1/trace/recent{}", suffix)).await
    }

    pub async fn by_trace_id(
        &self,
        trace_id: &str,
        raw: bool,
    ) -> Result<TraceByIdResponse, reqwest::Error> {
        let suffix = if raw { "?raw=true" } else { "" };
        self.get_json(&format!(
            "/v1/trace/{}{}",
            url_encode_query_component(trace_id),
            suffix
        ))
        .await
    }

    pub async fn by_task_id(
        &self,
        task_id: &str,
        raw: bool,
    ) -> Result<TraceByTaskResponse, reqwest::Error> {
        let suffix = if raw { "?raw=true" } else { "" };
        self.get_json(&format!(
            "/v1/trace/task/{}{}",
            url_encode_query_component(task_id),
            suffix
        ))
        .await
    }

    async fn get_json<T>(&self, path: &str) -> Result<T, reqwest::Error>
    where
        T: for<'de> Deserialize<'de>,
    {
        self.http
            .get(self.url(path))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }
}

pub type HeartbeatStatusResponse = serde_json::Value;
pub type HeartbeatUpdateResponse = serde_json::Value;
pub type HeartbeatLogEntry = serde_json::Value;

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct HeartbeatUpdateRequest {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub interval_minutes: Option<i32>,
}

/// Small Rust helper over `/v1/heartbeat*` proactive lifecycle endpoints.
#[derive(Debug, Clone)]
pub struct HeartbeatClient {
    base_url: String,
    http: reqwest::Client,
}

impl HeartbeatClient {
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let value = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&value) {
                headers.insert(AUTHORIZATION, value);
            }
        }
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: trim_base_url(base_url.into()),
            http,
        }
    }

    pub fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }

    pub async fn status(&self) -> Result<HeartbeatStatusResponse, reqwest::Error> {
        self.get_json("/v1/heartbeat").await
    }

    pub async fn update(
        &self,
        request: &HeartbeatUpdateRequest,
    ) -> Result<HeartbeatUpdateResponse, reqwest::Error> {
        self.http
            .put(self.url("/v1/heartbeat"))
            .json(request)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn trigger(&self) -> Result<HeartbeatLogEntry, reqwest::Error> {
        self.http
            .post(self.url("/v1/heartbeat/trigger"))
            .json(&serde_json::json!({}))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn logs(&self, limit: i32) -> Result<Vec<HeartbeatLogEntry>, reqwest::Error> {
        let suffix = if limit > 0 {
            format!("?limit={limit}")
        } else {
            String::new()
        };
        self.get_json(&format!("/v1/heartbeat/logs{}", suffix))
            .await
    }

    async fn get_json<T>(&self, path: &str) -> Result<T, reqwest::Error>
    where
        T: for<'de> Deserialize<'de>,
    {
        self.http
            .get(self.url(path))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct EventStreamMessage {
    pub event: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub data: Option<serde_json::Value>,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub id: String,
    #[serde(default, skip_serializing_if = "is_default")]
    pub retry: i32,
    pub raw: String,
}

/// Small Rust helper over `/v1/events/stream` Server-Sent Events integration.
#[derive(Debug, Clone)]
pub struct EventsClient {
    base_url: String,
    http: reqwest::Client,
}

impl EventsClient {
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let value = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&value) {
                headers.insert(AUTHORIZATION, value);
            }
        }
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: trim_base_url(base_url.into()),
            http,
        }
    }

    pub fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }

    pub fn stream_url(&self) -> String {
        self.url("/v1/events/stream")
    }

    pub async fn stream_text(&self) -> Result<String, reqwest::Error> {
        self.http
            .get(self.stream_url())
            .send()
            .await?
            .error_for_status()?
            .text()
            .await
    }

    pub fn parse(&self, text: &str) -> Vec<EventStreamMessage> {
        parse_sse_events(text)
    }
}

pub fn parse_sse_events(text: &str) -> Vec<EventStreamMessage> {
    text.replace("\r\n", "\n")
        .split("\n\n")
        .filter_map(|raw| {
            if raw.trim().is_empty() {
                return None;
            }
            let mut event = "message".to_string();
            let mut data = Vec::new();
            let mut id = String::new();
            let mut retry = 0;
            for line in raw.lines() {
                if line.is_empty() || line.starts_with(':') {
                    continue;
                }
                let (field, value) = line.split_once(':').unwrap_or((line, ""));
                let value = value.strip_prefix(' ').unwrap_or(value);
                match field {
                    "event" => event = value.to_string(),
                    "data" => data.push(value.to_string()),
                    "id" => id = value.to_string(),
                    "retry" => retry = value.parse().unwrap_or(0),
                    _ => {}
                }
            }
            if event == "message" && data.is_empty() && id.is_empty() && retry == 0 {
                return None;
            }
            let data = if data.is_empty() {
                None
            } else {
                let payload = data.join("\n");
                Some(
                    serde_json::from_str(&payload)
                        .unwrap_or_else(|_| serde_json::Value::String(payload)),
                )
            };
            Some(EventStreamMessage {
                event,
                data,
                id,
                retry,
                raw: raw.to_string(),
            })
        })
        .collect()
}

pub type ConversationSession = serde_json::Map<String, serde_json::Value>;
pub type ConversationMessage = serde_json::Map<String, serde_json::Value>;
pub type ConversationsResponse = serde_json::Value;
pub type ConversationMessagesResponse = serde_json::Value;
pub type ConversationDeleteResponse = serde_json::Value;
pub type ManageConversationResponse = serde_json::Value;
pub type ConversationReplayResponse = serde_json::Value;

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ManageConversationRequest {
    pub session_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub name: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub pinned: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub archive: Option<bool>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ConversationReplayOptions {
    #[serde(default)]
    pub raw: bool,
    #[serde(default, skip_serializing_if = "is_default")]
    pub limit: i32,
    #[serde(default, skip_serializing_if = "is_default")]
    pub offset: i32,
}

pub type BrowserResponse = serde_json::Value;
pub type BrowserAction = serde_json::Map<String, serde_json::Value>;

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ReactRequest {
    pub channel_type: String,
    pub target: String,
    pub message_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub emoji: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct SendStickerRequest {
    pub channel_type: String,
    pub target: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub package_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub sticker_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub file_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub emoji: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub platform: String,
}

pub type ReactionStatusResponse = serde_json::Value;

#[derive(Debug, Clone)]
pub struct ReactionsClient {
    base_url: String,
    http: reqwest::Client,
}

impl ReactionsClient {
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let bearer = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&bearer) {
                headers.insert(AUTHORIZATION, value);
            }
        }
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: base_url.into().trim_end_matches('/').to_string(),
            http,
        }
    }

    pub fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }

    pub async fn react(
        &self,
        request: &ReactRequest,
    ) -> Result<ReactionStatusResponse, reqwest::Error> {
        self.http
            .post(self.url("/v1/react"))
            .json(request)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn send_sticker(
        &self,
        request: &SendStickerRequest,
    ) -> Result<ReactionStatusResponse, reqwest::Error> {
        self.http
            .post(self.url("/v1/sticker/send"))
            .json(request)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }
}

pub type UserInstruction = serde_json::Map<String, serde_json::Value>;
pub type InstructionsResponse = serde_json::Value;
pub type InstructionStatusResponse = serde_json::Value;

#[derive(Debug, Clone)]
pub struct InstructionsClient {
    base_url: String,
    http: reqwest::Client,
}

impl InstructionsClient {
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let bearer = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&bearer) {
                headers.insert(AUTHORIZATION, value);
            }
        }
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: base_url.into().trim_end_matches('/').to_string(),
            http,
        }
    }

    pub fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }

    pub async fn list(&self, category: &str) -> Result<InstructionsResponse, reqwest::Error> {
        self.http
            .get(self.url(&("/v1/instructions".to_string() + &instructions_list_query(category))))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn create(
        &self,
        instruction: &UserInstruction,
    ) -> Result<UserInstruction, reqwest::Error> {
        self.http
            .post(self.url("/v1/instructions"))
            .json(instruction)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn update(
        &self,
        instruction: &UserInstruction,
    ) -> Result<InstructionStatusResponse, reqwest::Error> {
        self.http
            .put(self.url("/v1/instructions"))
            .json(instruction)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn delete(&self, id: &str) -> Result<InstructionStatusResponse, reqwest::Error> {
        self.http
            .delete(self.url(&format!(
                "/v1/instructions?id={}",
                url_encode_query_component(id)
            )))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn reorder(
        &self,
        ids: &[String],
    ) -> Result<InstructionStatusResponse, reqwest::Error> {
        self.http
            .post(self.url("/v1/instructions/reorder"))
            .json(&serde_json::json!({ "ids": ids }))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }
}

fn instructions_list_query(category: &str) -> String {
    if category.is_empty() {
        String::new()
    } else {
        format!("?category={}", url_encode_query_component(category))
    }
}

pub type EmotionHistoryResponse = serde_json::Value;
pub type StickerMapResponse = serde_json::Value;
pub type EmotionStatusResponse = serde_json::Value;

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct EmotionHistoryQuery {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub session_id: String,
    #[serde(default, skip_serializing_if = "is_default")]
    pub limit: i64,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub from: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub to: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct StickerSuggestion {
    pub package_id: String,
    pub sticker_id: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct RegisterStickersRequest {
    pub platform: String,
    pub emotion: String,
    pub stickers: Vec<StickerSuggestion>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ClearStickersRequest {
    pub platform: String,
    pub emotion: String,
}

#[derive(Debug, Clone)]
pub struct EmotionClient {
    base_url: String,
    http: reqwest::Client,
}

impl EmotionClient {
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let bearer = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&bearer) {
                headers.insert(AUTHORIZATION, value);
            }
        }
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: base_url.into().trim_end_matches('/').to_string(),
            http,
        }
    }

    pub fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }

    pub async fn history(
        &self,
        query: &EmotionHistoryQuery,
    ) -> Result<EmotionHistoryResponse, reqwest::Error> {
        self.http
            .get(self.url(&("/v1/emotion/history".to_string() + &emotion_history_query(query))))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn stickers(&self) -> Result<StickerMapResponse, reqwest::Error> {
        self.http
            .get(self.url("/v1/emotion/stickers"))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn register_stickers(
        &self,
        request: &RegisterStickersRequest,
    ) -> Result<EmotionStatusResponse, reqwest::Error> {
        self.http
            .put(self.url("/v1/emotion/stickers"))
            .json(request)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn clear_stickers(
        &self,
        request: &ClearStickersRequest,
    ) -> Result<EmotionStatusResponse, reqwest::Error> {
        self.http
            .delete(self.url("/v1/emotion/stickers"))
            .json(request)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }
}

fn emotion_history_query(query: &EmotionHistoryQuery) -> String {
    let mut pairs = Vec::new();
    if !query.session_id.is_empty() {
        pairs.push(format!(
            "session_id={}",
            url_encode_query_component(&query.session_id)
        ));
    }
    if query.limit > 0 {
        pairs.push(format!("limit={}", query.limit));
    }
    if !query.from.is_empty() {
        pairs.push(format!("from={}", url_encode_query_component(&query.from)));
    }
    if !query.to.is_empty() {
        pairs.push(format!("to={}", url_encode_query_component(&query.to)));
    }
    if pairs.is_empty() {
        String::new()
    } else {
        format!("?{}", pairs.join("&"))
    }
}

pub type PersonaStateResponse = serde_json::Value;
pub type PersonaStatusResponse = serde_json::Value;
pub type PersonaSkillsResponse = serde_json::Value;
pub type PersonaPresetsResponse = serde_json::Value;

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct UpdatePersonaRequest {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub identity: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub soul: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct AddPersonaSkillRequest {
    pub name: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub description: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub content: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct PersonaNameRequest {
    pub name: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct PersonaPresetIdRequest {
    pub id: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct AddCustomPersonaPresetRequest {
    pub id: String,
    pub name: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub description: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub tone: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub style: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub greeting: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub system_note: String,
    #[serde(default, skip_serializing_if = "std::collections::BTreeMap::is_empty")]
    pub features: std::collections::BTreeMap<String, bool>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct UpdatePersonaPresetFeaturesRequest {
    pub id: String,
    pub features: std::collections::BTreeMap<String, bool>,
}

/// Small Rust helper over persona identity, skills, and preset endpoints.
#[derive(Debug, Clone)]
pub struct PersonaClient {
    base_url: String,
    http: reqwest::Client,
}

impl PersonaClient {
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let bearer = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&bearer) {
                headers.insert(AUTHORIZATION, value);
            }
        }
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: base_url.into().trim_end_matches('/').to_string(),
            http,
        }
    }

    pub fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }

    pub async fn get(&self) -> Result<PersonaStateResponse, reqwest::Error> {
        self.http
            .get(self.url("/v1/persona"))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn update(
        &self,
        request: &UpdatePersonaRequest,
    ) -> Result<PersonaStatusResponse, reqwest::Error> {
        self.http
            .put(self.url("/v1/persona"))
            .json(request)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn skills(&self) -> Result<PersonaSkillsResponse, reqwest::Error> {
        self.http
            .get(self.url("/v1/persona/skills"))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn add_skill(
        &self,
        request: &AddPersonaSkillRequest,
    ) -> Result<PersonaStatusResponse, reqwest::Error> {
        self.http
            .post(self.url("/v1/persona/skills"))
            .json(request)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn delete_skill(&self, name: &str) -> Result<PersonaStatusResponse, reqwest::Error> {
        self.http
            .delete(self.url("/v1/persona/skills"))
            .json(&PersonaNameRequest {
                name: name.to_string(),
            })
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn presets(&self) -> Result<PersonaPresetsResponse, reqwest::Error> {
        self.http
            .get(self.url("/v1/persona/presets"))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn switch_preset(&self, id: &str) -> Result<PersonaStatusResponse, reqwest::Error> {
        self.http
            .post(self.url("/v1/persona/presets"))
            .json(&PersonaPresetIdRequest { id: id.to_string() })
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn add_custom_preset(
        &self,
        request: &AddCustomPersonaPresetRequest,
    ) -> Result<PersonaStatusResponse, reqwest::Error> {
        self.http
            .post(self.url("/v1/persona/presets/custom"))
            .json(request)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn delete_custom_preset(
        &self,
        id: &str,
    ) -> Result<PersonaStatusResponse, reqwest::Error> {
        self.http
            .delete(self.url("/v1/persona/presets/custom"))
            .json(&PersonaPresetIdRequest { id: id.to_string() })
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn update_preset_features(
        &self,
        request: &UpdatePersonaPresetFeaturesRequest,
    ) -> Result<PersonaStatusResponse, reqwest::Error> {
        self.http
            .put(self.url("/v1/persona/presets/features"))
            .json(request)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }
}

pub type IterateProposalsResponse = serde_json::Value;
pub type IterateDecisionResponse = serde_json::Value;
pub type IterateTriggerResponse = serde_json::Value;
pub type IterateStatusResponse = serde_json::Value;

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct IterateProposalsQuery {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub status: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct IterateDecisionRequest {
    pub id: String,
}

fn iterate_proposals_query(query: &IterateProposalsQuery) -> String {
    if query.status.is_empty() {
        String::new()
    } else {
        format!("?status={}", url_encode_query_component(&query.status))
    }
}

/// Small Rust helper over self-iteration proposal review and manual cycle endpoints.
#[derive(Debug, Clone)]
pub struct IterateClient {
    base_url: String,
    http: reqwest::Client,
}

impl IterateClient {
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let bearer = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&bearer) {
                headers.insert(AUTHORIZATION, value);
            }
        }
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: base_url.into().trim_end_matches('/').to_string(),
            http,
        }
    }

    pub fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }

    pub async fn proposals(
        &self,
        query: &IterateProposalsQuery,
    ) -> Result<IterateProposalsResponse, reqwest::Error> {
        self.http
            .get(self.url(&format!(
                "/api/iterate/proposals{}",
                iterate_proposals_query(query)
            )))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn pending_proposals(&self) -> Result<IterateProposalsResponse, reqwest::Error> {
        self.proposals(&IterateProposalsQuery {
            status: "pending".to_string(),
        })
        .await
    }

    pub async fn approve(&self, id: &str) -> Result<IterateDecisionResponse, reqwest::Error> {
        self.http
            .post(self.url("/api/iterate/approve"))
            .json(&IterateDecisionRequest { id: id.to_string() })
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn reject(&self, id: &str) -> Result<IterateDecisionResponse, reqwest::Error> {
        self.http
            .post(self.url("/api/iterate/reject"))
            .json(&IterateDecisionRequest { id: id.to_string() })
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn trigger(&self) -> Result<IterateTriggerResponse, reqwest::Error> {
        self.http
            .post(self.url("/api/iterate/trigger"))
            .json(&serde_json::json!({}))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn status(&self) -> Result<IterateStatusResponse, reqwest::Error> {
        self.http
            .get(self.url("/api/iterate/status"))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }
}

pub type TrustScoresResponse = serde_json::Value;

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct TrustSlugRequest {
    pub slug: String,
}

pub type TrustMutationResponse = serde_json::Value;
pub type ReviewStatusResponse = serde_json::Value;
pub type SkillGrowPatternsResponse = serde_json::Value;

/// Small Rust helper over trust, review gate, and skill growth governance endpoints.
#[derive(Debug, Clone)]
pub struct TrustClient {
    base_url: String,
    http: reqwest::Client,
}

impl TrustClient {
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let bearer = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&bearer) {
                headers.insert(AUTHORIZATION, value);
            }
        }
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: base_url.into().trim_end_matches('/').to_string(),
            http,
        }
    }

    pub fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }

    pub async fn scores(&self) -> Result<TrustScoresResponse, reqwest::Error> {
        self.http
            .get(self.url("/api/trust/scores"))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn reset(&self, slug: &str) -> Result<TrustMutationResponse, reqwest::Error> {
        self.http
            .post(self.url("/api/trust/reset"))
            .json(&TrustSlugRequest {
                slug: slug.to_string(),
            })
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn grant(&self, slug: &str) -> Result<TrustMutationResponse, reqwest::Error> {
        self.http
            .post(self.url("/api/trust/grant"))
            .json(&TrustSlugRequest {
                slug: slug.to_string(),
            })
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn grant_all(&self) -> Result<TrustMutationResponse, reqwest::Error> {
        self.grant("*").await
    }

    pub async fn review_status(&self) -> Result<ReviewStatusResponse, reqwest::Error> {
        self.http
            .get(self.url("/api/review/status"))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn skillgrow_patterns(&self) -> Result<SkillGrowPatternsResponse, reqwest::Error> {
        self.http
            .get(self.url("/api/skillgrow/patterns"))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct AuditTailQuery {
    #[serde(default, skip_serializing_if = "is_default")]
    pub n: i32,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub r#type: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub actor: String,
}

pub type AuditRecord = serde_json::Map<String, serde_json::Value>;

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct AuditTailResponse {
    #[serde(default)]
    pub records: Vec<serde_json::Value>,
    #[serde(default)]
    pub count: i32,
    #[serde(default)]
    pub error: String,
}

pub type AuditVerifyResponse = serde_json::Value;
pub type AuditStatsResponse = serde_json::Value;

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct AuditTrailQuery {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub date: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub r#type: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct AuditTrailResponse {
    #[serde(default)]
    pub entries: Vec<serde_json::Value>,
    #[serde(default)]
    pub count: i32,
}

/// Small Rust helper over Merkle audit-chain and task audit-trail read endpoints.
#[derive(Debug, Clone)]
pub struct AuditClient {
    base_url: String,
    http: reqwest::Client,
}

impl AuditClient {
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let bearer = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&bearer) {
                headers.insert(AUTHORIZATION, value);
            }
        }
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: base_url.into().trim_end_matches('/').to_string(),
            http,
        }
    }

    pub fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }

    pub async fn tail(&self, query: &AuditTailQuery) -> Result<AuditTailResponse, reqwest::Error> {
        self.http
            .get(self.url(&format!("/v1/audit/tail{}", audit_tail_query(query))))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn verify(&self) -> Result<AuditVerifyResponse, reqwest::Error> {
        self.http
            .get(self.url("/v1/audit/verify"))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn stats(&self) -> Result<AuditStatsResponse, reqwest::Error> {
        self.http
            .get(self.url("/v1/audit/stats"))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn trail(
        &self,
        query: &AuditTrailQuery,
    ) -> Result<AuditTrailResponse, reqwest::Error> {
        self.http
            .get(self.url(&format!("/api/audit/trail{}", audit_trail_query(query))))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }
}

fn audit_tail_query(query: &AuditTailQuery) -> String {
    let mut pairs = Vec::new();
    if query.n > 0 {
        pairs.push(format!("n={}", query.n));
    }
    if !query.r#type.is_empty() {
        pairs.push(format!(
            "type={}",
            url_encode_query_component(&query.r#type)
        ));
    }
    if !query.actor.is_empty() {
        pairs.push(format!(
            "actor={}",
            url_encode_query_component(&query.actor)
        ));
    }
    if pairs.is_empty() {
        String::new()
    } else {
        format!("?{}", pairs.join("&"))
    }
}

fn audit_trail_query(query: &AuditTrailQuery) -> String {
    let mut pairs = Vec::new();
    if !query.date.is_empty() {
        pairs.push(format!("date={}", url_encode_query_component(&query.date)));
    }
    if !query.r#type.is_empty() {
        pairs.push(format!(
            "type={}",
            url_encode_query_component(&query.r#type)
        ));
    }
    if pairs.is_empty() {
        String::new()
    } else {
        format!("?{}", pairs.join("&"))
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ToolExecOptions {
    #[serde(rename = "Command")]
    pub command: String,
    #[serde(default, rename = "Cwd", skip_serializing_if = "String::is_empty")]
    pub cwd: String,
    #[serde(default, rename = "Background", skip_serializing_if = "is_default")]
    pub background: bool,
    #[serde(default, rename = "TimeoutMs", skip_serializing_if = "is_default")]
    pub timeout_ms: i64,
    #[serde(default, rename = "YieldMs", skip_serializing_if = "is_default")]
    pub yield_ms: i64,
    #[serde(default, rename = "Env", skip_serializing_if = "Vec::is_empty")]
    pub env: Vec<String>,
}

pub type ToolExecResult = serde_json::Value;

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ToolProcessSession {
    #[serde(default)]
    pub id: String,
    #[serde(default)]
    pub command: String,
    #[serde(default)]
    pub state: String,
    #[serde(default)]
    pub exit_code: i32,
    #[serde(default)]
    pub started_at: String,
    #[serde(default)]
    pub ended_at: String,
    #[serde(default)]
    pub cwd: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ToolListResponse {
    #[serde(default)]
    pub sessions: Vec<ToolProcessSession>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ToolPollResponse {
    #[serde(default)]
    pub lines: Vec<String>,
    #[serde(default)]
    pub state: String,
}

pub type ToolKillResponse = serde_json::Value;

/// Small Rust helper over `/v1/tools/*` controlled process execution endpoints.
#[derive(Debug, Clone)]
pub struct ToolsClient {
    base_url: String,
    http: reqwest::Client,
}

impl ToolsClient {
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let bearer = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&bearer) {
                headers.insert(AUTHORIZATION, value);
            }
        }
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: base_url.into().trim_end_matches('/').to_string(),
            http,
        }
    }

    pub fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }

    pub async fn exec(&self, options: &ToolExecOptions) -> Result<ToolExecResult, reqwest::Error> {
        self.http
            .post(self.url("/v1/tools/exec"))
            .json(options)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn list(&self) -> Result<ToolListResponse, reqwest::Error> {
        self.http
            .get(self.url("/v1/tools/list"))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn poll(&self, id: &str) -> Result<ToolPollResponse, reqwest::Error> {
        self.http
            .get(self.url(&format!(
                "/v1/tools/poll?id={}",
                url_encode_query_component(id)
            )))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn kill(&self, id: &str) -> Result<ToolKillResponse, reqwest::Error> {
        self.http
            .post(self.url(&format!(
                "/v1/tools/kill?id={}",
                url_encode_query_component(id)
            )))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct Subagent {
    #[serde(default)]
    pub id: String,
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub description: String,
    #[serde(default)]
    pub parent_id: String,
    #[serde(default)]
    pub messages: Vec<serde_json::Value>,
    #[serde(default)]
    pub skills: Vec<String>,
    #[serde(default)]
    pub metadata: serde_json::Value,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct SubagentsResponse {
    #[serde(default)]
    pub subagents: Vec<Subagent>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct SpawnSubagentRequest {
    #[serde(default)]
    pub parent_id: String,
    pub name: String,
    #[serde(default)]
    pub description: String,
    #[serde(default)]
    pub skills: Vec<String>,
}

pub type SubagentMessage = serde_json::Map<String, serde_json::Value>;
pub type AppendSubagentMessagesResponse = serde_json::Value;
pub type DeleteSubagentResponse = serde_json::Value;

/// Small Rust helper over `/v1/subagent` and `/v1/subagent/message` endpoints.
#[derive(Debug, Clone)]
pub struct SubagentsClient {
    base_url: String,
    http: reqwest::Client,
}

impl SubagentsClient {
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let bearer = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&bearer) {
                headers.insert(AUTHORIZATION, value);
            }
        }
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: base_url.into().trim_end_matches('/').to_string(),
            http,
        }
    }

    pub fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }

    pub async fn list(&self, parent_id: &str) -> Result<SubagentsResponse, reqwest::Error> {
        let path = if parent_id.is_empty() {
            "/v1/subagent".to_string()
        } else {
            format!(
                "/v1/subagent?parent_id={}",
                url_encode_query_component(parent_id)
            )
        };
        self.http
            .get(self.url(&path))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn get(&self, id: &str) -> Result<Subagent, reqwest::Error> {
        self.http
            .get(self.url(&format!(
                "/v1/subagent?id={}",
                url_encode_query_component(id)
            )))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn spawn(&self, request: &SpawnSubagentRequest) -> Result<Subagent, reqwest::Error> {
        self.http
            .post(self.url("/v1/subagent"))
            .json(request)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn destroy(&self, id: &str) -> Result<DeleteSubagentResponse, reqwest::Error> {
        self.http
            .delete(self.url(&format!(
                "/v1/subagent?id={}",
                url_encode_query_component(id)
            )))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn append_messages(
        &self,
        id: &str,
        messages: &[serde_json::Value],
    ) -> Result<AppendSubagentMessagesResponse, reqwest::Error> {
        self.http
            .post(self.url("/v1/subagent/message"))
            .json(&serde_json::json!({"id": id, "messages": messages}))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }
}

pub type RuntimeQueueTask = serde_json::Map<String, serde_json::Value>;
pub type RuntimeQueueOverviewResponse = serde_json::Value;
pub type RuntimeQueueSessionResponse = serde_json::Value;
pub type RuntimeQueueCancelResponse = serde_json::Value;

/// Small Rust helper over session queue operations and `/v1/events/stream` URLs.
#[derive(Debug, Clone)]
pub struct RuntimeClient {
    base_url: String,
    http: reqwest::Client,
}

impl RuntimeClient {
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let bearer = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&bearer) {
                headers.insert(AUTHORIZATION, value);
            }
        }
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: base_url.into().trim_end_matches('/').to_string(),
            http,
        }
    }

    pub fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }

    pub async fn queues(&self) -> Result<RuntimeQueueOverviewResponse, reqwest::Error> {
        self.http
            .get(self.url("/v1/sessions/queue"))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn session_queue(
        &self,
        session_id: &str,
    ) -> Result<RuntimeQueueSessionResponse, reqwest::Error> {
        self.http
            .get(self.url(&format!(
                "/v1/sessions/queue?id={}",
                url_encode_query_component(session_id)
            )))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn cancel_queued_task(
        &self,
        session_id: &str,
        task_id: &str,
    ) -> Result<RuntimeQueueCancelResponse, reqwest::Error> {
        self.http
            .post(self.url("/v1/sessions/queue/cancel"))
            .json(&serde_json::json!({"session_id": session_id, "task_id": task_id}))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub fn events_stream_url(&self) -> String {
        self.url("/v1/events/stream")
    }
}

/// Small Rust helper over `/v1/browser*` and `/api/browser/ext*` browser automation endpoints.
#[derive(Debug, Clone)]
pub struct BrowserClient {
    base_url: String,
    http: reqwest::Client,
}

impl BrowserClient {
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let mut headers = reqwest::header::HeaderMap::new();
        let bearer = format!("Bearer {}", token.as_ref());
        headers.insert(
            reqwest::header::AUTHORIZATION,
            reqwest::header::HeaderValue::from_str(&bearer).unwrap(),
        );
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: trim_base_url(base_url.into()),
            http,
        }
    }

    pub fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }

    pub async fn status(&self) -> Result<BrowserResponse, reqwest::Error> {
        self.get_json("/v1/browser/status").await
    }
    pub async fn config(&self) -> Result<BrowserResponse, reqwest::Error> {
        self.get_json("/v1/browser/config").await
    }
    pub async fn navigate(
        &self,
        url: impl Into<String>,
    ) -> Result<BrowserResponse, reqwest::Error> {
        self.post_json(
            "/v1/browser/navigate",
            &serde_json::json!({ "url": url.into() }),
        )
        .await
    }
    pub async fn screenshot(&self) -> Result<BrowserResponse, reqwest::Error> {
        self.get_json("/v1/browser/screenshot").await
    }
    pub async fn latest_screenshot(&self) -> Result<BrowserResponse, reqwest::Error> {
        self.get_json("/v1/browser/screenshot/latest").await
    }
    pub async fn ocr(&self) -> Result<BrowserResponse, reqwest::Error> {
        self.post_json("/v1/browser/ocr", &serde_json::json!({}))
            .await
    }
    pub async fn opp_pending(&self) -> Result<BrowserResponse, reqwest::Error> {
        self.get_json("/v1/browser/opp/pending").await
    }
    pub async fn opp_decide(
        &self,
        decision: BrowserAction,
    ) -> Result<BrowserResponse, reqwest::Error> {
        self.post_json("/v1/browser/opp/decide", &decision).await
    }
    pub async fn extension_status(&self) -> Result<BrowserResponse, reqwest::Error> {
        self.get_json("/api/browser/ext/status").await
    }
    pub async fn extension_session(&self) -> Result<BrowserResponse, reqwest::Error> {
        self.post_json("/api/browser/ext/session", &serde_json::json!({}))
            .await
    }
    pub async fn extension_action(
        &self,
        action: BrowserAction,
    ) -> Result<BrowserResponse, reqwest::Error> {
        self.post_json("/api/browser/ext/action", &action).await
    }
    pub async fn scenarios(&self) -> Result<BrowserResponse, reqwest::Error> {
        self.get_json("/api/browser/ext/scenarios").await
    }
    pub async fn run_scenario(
        &self,
        scenario_id: impl Into<String>,
    ) -> Result<BrowserResponse, reqwest::Error> {
        self.post_json(
            "/api/browser/ext/scenarios/run",
            &serde_json::json!({ "scenario_id": scenario_id.into() }),
        )
        .await
    }

    async fn get_json<T>(&self, path: &str) -> Result<T, reqwest::Error>
    where
        T: for<'de> Deserialize<'de>,
    {
        self.http
            .get(self.url(path))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    async fn post_json<B, T>(&self, path: &str, body: &B) -> Result<T, reqwest::Error>
    where
        B: Serialize + ?Sized,
        T: for<'de> Deserialize<'de>,
    {
        self.http
            .post(self.url(path))
            .json(body)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct FileEntry {
    pub name: String,
    pub path: String,
    #[serde(default)]
    pub size: i64,
    #[serde(default)]
    pub is_dir: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct FileListResponse {
    #[serde(default)]
    pub files: Vec<FileEntry>,
}

pub type FilePreviewResponse = serde_json::Value;

#[derive(Debug, Clone, PartialEq)]
pub struct FileDownloadResponse {
    pub content: Vec<u8>,
    pub filename: String,
    pub content_type: String,
}

/// Small Rust helper over `/api/files*` agent output file listing, previews, and downloads.
#[derive(Debug, Clone)]
pub struct FilesClient {
    base_url: String,
    http: reqwest::Client,
}

impl FilesClient {
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let mut headers = reqwest::header::HeaderMap::new();
        let bearer = format!("Bearer {}", token.as_ref());
        headers.insert(
            reqwest::header::AUTHORIZATION,
            reqwest::header::HeaderValue::from_str(&bearer).unwrap(),
        );
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: trim_base_url(base_url.into()),
            http,
        }
    }

    pub fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }

    pub async fn list(&self, path: impl AsRef<str>) -> Result<FileListResponse, reqwest::Error> {
        let path = path.as_ref();
        let api_path = if path.is_empty() {
            "/api/files".to_string()
        } else {
            format!("/api/files?path={}", encode_query_component(path))
        };
        self.get_json(&api_path).await
    }

    pub async fn preview(
        &self,
        path: impl AsRef<str>,
    ) -> Result<FilePreviewResponse, reqwest::Error> {
        self.get_json(&format!(
            "/api/files/preview?path={}",
            encode_query_component(path.as_ref())
        ))
        .await
    }

    pub async fn download(
        &self,
        path: impl AsRef<str>,
    ) -> Result<FileDownloadResponse, reqwest::Error> {
        let response = self
            .http
            .get(self.url(&format!(
                "/api/files/download?path={}",
                encode_query_component(path.as_ref())
            )))
            .send()
            .await?
            .error_for_status()?;
        let headers = response.headers().clone();
        let content = response.bytes().await?.to_vec();
        Ok(FileDownloadResponse {
            content,
            filename: filename_from_disposition(
                headers
                    .get(reqwest::header::CONTENT_DISPOSITION)
                    .and_then(|v| v.to_str().ok())
                    .unwrap_or(""),
            ),
            content_type: headers
                .get(reqwest::header::CONTENT_TYPE)
                .and_then(|v| v.to_str().ok())
                .unwrap_or("")
                .to_string(),
        })
    }

    async fn get_json<T>(&self, path: &str) -> Result<T, reqwest::Error>
    where
        T: for<'de> Deserialize<'de>,
    {
        self.http
            .get(self.url(path))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }
}

fn filename_from_disposition(disposition: &str) -> String {
    for part in disposition.split(';') {
        let trimmed = part.trim();
        if let Some(value) = trimmed.strip_prefix("filename=") {
            return value.trim_matches('"').to_string();
        }
    }
    String::new()
}

pub type Task = serde_json::Map<String, serde_json::Value>;
pub type TaskActionResponse = serde_json::Value;
pub type TaskTemplate = serde_json::Value;
pub type TaskTemplatesResponse = serde_json::Value;
pub type DeleteTaskTemplateResponse = serde_json::Value;
pub type TaskGap = serde_json::Value;
pub type TaskGapStats = serde_json::Value;
pub type ResolveTaskGapResponse = serde_json::Value;

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct TaskConstraints {
    #[serde(default, skip_serializing_if = "is_default")]
    pub max_steps: i32,
    #[serde(default, skip_serializing_if = "is_default")]
    pub timeout_sec: i32,
    #[serde(default, skip_serializing_if = "is_default")]
    pub max_cost_usd: f64,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub success_criteria: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub test_command: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub priority: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub risk_level: String,
    #[serde(default, skip_serializing_if = "is_default")]
    pub auto_approve: bool,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub tags: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct CreateTaskRequest {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub title: String,
    pub description: String,
    #[serde(default, skip_serializing_if = "is_default")]
    pub constraints: TaskConstraints,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct TaskTemplateVariable {
    pub name: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub description: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub default: String,
    #[serde(default, skip_serializing_if = "is_default")]
    pub required: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct TaskTemplateStep {
    pub action: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub skill_name: String,
    #[serde(default, skip_serializing_if = "serde_json::Map::is_empty")]
    pub args: serde_json::Map<String, serde_json::Value>,
    #[serde(default, skip_serializing_if = "is_default")]
    pub group: i32,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct CreateTaskTemplateRequest {
    pub id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub name: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub description: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub variables: Vec<TaskTemplateVariable>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub steps: Vec<TaskTemplateStep>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub tags: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct InstantiateTaskTemplateRequest {
    pub template_id: String,
    #[serde(default, skip_serializing_if = "std::collections::BTreeMap::is_empty")]
    pub variables: std::collections::BTreeMap<String, String>,
}

/// Small Rust helper over task CRUD and lifecycle endpoints.
#[derive(Debug, Clone)]
pub struct TasksClient {
    base_url: String,
    http: reqwest::Client,
}

impl TasksClient {
    pub fn new(base_url: impl Into<String>, token: impl AsRef<str>) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let value = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&value) { headers.insert(AUTHORIZATION, value); }
        }
        Ok(Self::new_with_client(base_url, reqwest::Client::builder().default_headers(headers).build()?))
    }

    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self { base_url: trim_base_url(base_url.into()), http }
    }

    pub fn url(&self, path: &str) -> String { format!("{}{}", self.base_url, path) }

    pub async fn list(&self) -> Result<Vec<Task>, reqwest::Error> { self.get_json("/v1/tasks").await }

    pub async fn get(&self, id: impl AsRef<str>) -> Result<Task, reqwest::Error> {
        self.get_json(&format!("/v1/tasks?id={}", url_encode_query_component(id.as_ref()))).await
    }

    pub async fn create(&self, request: &CreateTaskRequest) -> Result<Task, reqwest::Error> {
        self.post_json("/v1/tasks", request).await
    }

    pub async fn run(&self, id: impl AsRef<str>) -> Result<TaskActionResponse, reqwest::Error> { self.action("run", id.as_ref()).await }
    pub async fn pause(&self, id: impl AsRef<str>) -> Result<TaskActionResponse, reqwest::Error> { self.action("pause", id.as_ref()).await }
    pub async fn resume(&self, id: impl AsRef<str>) -> Result<TaskActionResponse, reqwest::Error> { self.action("resume", id.as_ref()).await }
    pub async fn restart(&self, id: impl AsRef<str>) -> Result<TaskActionResponse, reqwest::Error> { self.action("restart", id.as_ref()).await }
    pub async fn cancel(&self, id: impl AsRef<str>) -> Result<TaskActionResponse, reqwest::Error> { self.action("cancel", id.as_ref()).await }

    pub async fn delete(&self, id: impl AsRef<str>) -> Result<TaskActionResponse, reqwest::Error> {
        self.http.delete(self.url(&format!("/v1/tasks?id={}", url_encode_query_component(id.as_ref())))).send().await?.error_for_status()?.json().await
    }

    pub async fn templates(&self) -> Result<TaskTemplatesResponse, reqwest::Error> {
        self.get_json("/v1/tasks/templates").await
    }

    pub async fn template(&self, id: impl AsRef<str>) -> Result<TaskTemplate, reqwest::Error> {
        self.get_json(&format!("/v1/tasks/templates?id={}", url_encode_query_component(id.as_ref()))).await
    }

    pub async fn create_template(&self, request: &CreateTaskTemplateRequest) -> Result<TaskTemplate, reqwest::Error> {
        self.post_json("/v1/tasks/templates", request).await
    }

    pub async fn delete_template(&self, id: impl AsRef<str>) -> Result<DeleteTaskTemplateResponse, reqwest::Error> {
        self.http.delete(self.url(&format!("/v1/tasks/templates?id={}", url_encode_query_component(id.as_ref())))).send().await?.error_for_status()?.json().await
    }

    pub async fn instantiate_template(&self, request: &InstantiateTaskTemplateRequest) -> Result<Task, reqwest::Error> {
        self.post_json("/v1/tasks/templates/instantiate", request).await
    }

    pub async fn gaps(&self, gap_type: impl AsRef<str>) -> Result<Vec<TaskGap>, reqwest::Error> {
        let gap_type = gap_type.as_ref();
        if gap_type.is_empty() {
            self.get_json("/v1/tasks/gaps").await
        } else {
            self.get_json(&format!("/v1/tasks/gaps?type={}", url_encode_query_component(gap_type))).await
        }
    }

    pub async fn gap_stats(&self) -> Result<TaskGapStats, reqwest::Error> {
        self.get_json("/v1/tasks/gaps?stats=true").await
    }

    pub async fn resolve_gap(&self, id: impl AsRef<str>) -> Result<ResolveTaskGapResponse, reqwest::Error> {
        self.post_json("/v1/tasks/gaps/resolve", &serde_json::json!({"id": id.as_ref()})).await
    }

    async fn action(&self, action: &str, id: &str) -> Result<TaskActionResponse, reqwest::Error> {
        self.post_json(&format!("/v1/tasks/{action}"), &serde_json::json!({"id": id})).await
    }

    async fn get_json<T>(&self, path: &str) -> Result<T, reqwest::Error>
    where T: for<'de> Deserialize<'de>,
    { self.http.get(self.url(path)).send().await?.error_for_status()?.json().await }

    async fn post_json<B, T>(&self, path: &str, body: &B) -> Result<T, reqwest::Error>
    where B: Serialize + ?Sized, T: for<'de> Deserialize<'de>,
    { self.http.post(self.url(path)).json(body).send().await?.error_for_status()?.json().await }
}

pub type RBACPermission = serde_json::Map<String, serde_json::Value>;
pub type RBACRole = serde_json::Map<String, serde_json::Value>;
pub type RBACRolesResponse = serde_json::Value;
pub type RBACDeletedResponse = serde_json::Value;
pub type RBACRoleBindingResponse = serde_json::Value;

/// Small Rust facade over RBAC permission checks and current-role reads.
#[derive(Debug, Clone)]
pub struct PermissionsClient {
    rbac: RBACClient,
}

impl PermissionsClient {
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        Ok(Self {
            rbac: RBACClient::new(base_url, token)?,
        })
    }

    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            rbac: RBACClient::new_with_client(base_url, http),
        }
    }

    pub fn url(&self, path: &str) -> String {
        self.rbac.url(path)
    }

    pub async fn check(
        &self,
        request: RBACCheckRequest,
    ) -> Result<RBACCheckResponse, reqwest::Error> {
        self.rbac.check(request).await
    }

    pub async fn my_roles(&self) -> Result<RBACMyRolesResponse, reqwest::Error> {
        self.rbac.my_roles().await
    }
}

pub type RBACCheckResponse = serde_json::Value;
pub type RBACMyRolesResponse = serde_json::Value;

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct RBACRoleBindingRequest {
    pub subject_id: String,
    pub role_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub tenant_id: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct RBACCheckRequest {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub subject_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub tenant_id: String,
    pub resource: String,
    pub action: String,
}

/// Small Rust helper over `/v1/rbac*` roles, bindings, and permission checks.
#[derive(Debug, Clone)]
pub struct RBACClient {
    base_url: String,
    http: reqwest::Client,
}

impl RBACClient {
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let mut headers = reqwest::header::HeaderMap::new();
        let bearer = format!("Bearer {}", token.as_ref());
        headers.insert(
            reqwest::header::AUTHORIZATION,
            reqwest::header::HeaderValue::from_str(&bearer).unwrap(),
        );
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: trim_base_url(base_url.into()),
            http,
        }
    }

    pub fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }

    pub async fn roles(&self) -> Result<RBACRolesResponse, reqwest::Error> {
        self.get_json("/v1/rbac/roles").await
    }

    pub async fn create_role(&self, role: RBACRole) -> Result<RBACRole, reqwest::Error> {
        self.post_json("/v1/rbac/roles", &role).await
    }

    pub async fn delete_role(
        &self,
        id: impl AsRef<str>,
    ) -> Result<RBACDeletedResponse, reqwest::Error> {
        self.delete_json(&format!(
            "/v1/rbac/roles?id={}",
            encode_query_component(id.as_ref())
        ))
        .await
    }

    pub async fn assign_role(
        &self,
        request: RBACRoleBindingRequest,
    ) -> Result<RBACRoleBindingResponse, reqwest::Error> {
        self.post_json("/v1/rbac/assign", &request).await
    }

    pub async fn revoke_role(
        &self,
        request: RBACRoleBindingRequest,
    ) -> Result<RBACRoleBindingResponse, reqwest::Error> {
        self.post_json("/v1/rbac/revoke", &request).await
    }

    pub async fn check(
        &self,
        request: RBACCheckRequest,
    ) -> Result<RBACCheckResponse, reqwest::Error> {
        self.post_json("/v1/rbac/check", &request).await
    }

    pub async fn my_roles(&self) -> Result<RBACMyRolesResponse, reqwest::Error> {
        self.get_json("/v1/rbac/my-roles").await
    }

    async fn get_json<T>(&self, path: &str) -> Result<T, reqwest::Error>
    where
        T: for<'de> Deserialize<'de>,
    {
        self.http
            .get(self.url(path))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    async fn post_json<B, T>(&self, path: &str, body: &B) -> Result<T, reqwest::Error>
    where
        B: Serialize + ?Sized,
        T: for<'de> Deserialize<'de>,
    {
        self.http
            .post(self.url(path))
            .json(body)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    async fn delete_json<T>(&self, path: &str) -> Result<T, reqwest::Error>
    where
        T: for<'de> Deserialize<'de>,
    {
        self.http
            .delete(self.url(path))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }
}

pub type ApprovalRequest = serde_json::Map<String, serde_json::Value>;
pub type ApprovalRule = serde_json::Map<String, serde_json::Value>;
pub type ListApprovalsResponse = serde_json::Value;
pub type ApprovalActionResponse = serde_json::Value;
pub type ApprovalRulesResponse = serde_json::Value;

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ListApprovalsOptions {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub status: String,
    #[serde(default, skip_serializing_if = "is_default")]
    pub history: bool,
}

/// Small Rust helper over `/v1/approvals*` human-in-the-loop queues and rules.
#[derive(Debug, Clone)]
pub struct ApprovalsClient {
    base_url: String,
    http: reqwest::Client,
}

impl ApprovalsClient {
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let mut headers = reqwest::header::HeaderMap::new();
        let bearer = format!("Bearer {}", token.as_ref());
        headers.insert(
            reqwest::header::AUTHORIZATION,
            reqwest::header::HeaderValue::from_str(&bearer).unwrap(),
        );
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: trim_base_url(base_url.into()),
            http,
        }
    }

    pub fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }

    pub async fn list(
        &self,
        opts: ListApprovalsOptions,
    ) -> Result<ListApprovalsResponse, reqwest::Error> {
        let mut query = Vec::new();
        if !opts.status.is_empty() {
            query.push(format!("status={}", encode_query_component(&opts.status)));
        }
        if opts.history {
            query.push("history=true".to_string());
        }
        let path = if query.is_empty() {
            "/v1/approvals".to_string()
        } else {
            format!("/v1/approvals?{}", query.join("&"))
        };
        self.get_json(&path).await
    }

    pub async fn pending(&self) -> Result<ListApprovalsResponse, reqwest::Error> {
        self.list(ListApprovalsOptions {
            status: "pending".to_string(),
            history: false,
        })
        .await
    }

    pub async fn history(
        &self,
        status: impl Into<String>,
    ) -> Result<ListApprovalsResponse, reqwest::Error> {
        self.list(ListApprovalsOptions {
            status: status.into(),
            history: true,
        })
        .await
    }

    pub async fn approve(
        &self,
        id: impl Into<String>,
    ) -> Result<ApprovalActionResponse, reqwest::Error> {
        self.post_json(
            "/v1/approvals/approve",
            &serde_json::json!({ "id": id.into() }),
        )
        .await
    }

    pub async fn deny(
        &self,
        id: impl Into<String>,
        reason: impl Into<String>,
    ) -> Result<ApprovalActionResponse, reqwest::Error> {
        let reason = reason.into();
        let mut body = serde_json::Map::new();
        body.insert("id".to_string(), serde_json::Value::String(id.into()));
        if !reason.is_empty() {
            body.insert("reason".to_string(), serde_json::Value::String(reason));
        }
        self.post_json("/v1/approvals/deny", &body).await
    }

    pub async fn decide(
        &self,
        id: impl Into<String>,
        decision: impl Into<String>,
    ) -> Result<ApprovalActionResponse, reqwest::Error> {
        self.post_json(
            "/v1/approvals/decide",
            &serde_json::json!({ "id": id.into(), "decision": decision.into() }),
        )
        .await
    }

    pub async fn rules(&self) -> Result<ApprovalRulesResponse, reqwest::Error> {
        self.get_json("/v1/approvals/rules").await
    }

    pub async fn add_rule(
        &self,
        rule: ApprovalRule,
    ) -> Result<ApprovalActionResponse, reqwest::Error> {
        self.post_json("/v1/approvals/rules", &rule).await
    }

    pub async fn delete_rule(
        &self,
        id: impl AsRef<str>,
    ) -> Result<ApprovalActionResponse, reqwest::Error> {
        self.delete_json(&format!(
            "/v1/approvals/rules?id={}",
            encode_query_component(id.as_ref())
        ))
        .await
    }

    async fn get_json<T>(&self, path: &str) -> Result<T, reqwest::Error>
    where
        T: for<'de> Deserialize<'de>,
    {
        self.http
            .get(self.url(path))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    async fn post_json<B, T>(&self, path: &str, body: &B) -> Result<T, reqwest::Error>
    where
        B: Serialize + ?Sized,
        T: for<'de> Deserialize<'de>,
    {
        self.http
            .post(self.url(path))
            .json(body)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    async fn delete_json<T>(&self, path: &str) -> Result<T, reqwest::Error>
    where
        T: for<'de> Deserialize<'de>,
    {
        self.http
            .delete(self.url(path))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }
}

/// Small Rust helper over `/v1/conversations*` session, message, metadata, and replay endpoints.
#[derive(Debug, Clone)]
pub struct ConversationsClient {
    base_url: String,
    http: reqwest::Client,
}

impl ConversationsClient {
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let value = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&value) {
                headers.insert(AUTHORIZATION, value);
            }
        }
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: trim_base_url(base_url.into()),
            http,
        }
    }

    pub fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }

    pub async fn list(&self, archived: bool) -> Result<ConversationsResponse, reqwest::Error> {
        let path = if archived {
            "/v1/conversations?archived=true"
        } else {
            "/v1/conversations"
        };
        self.get_json(path).await
    }

    pub async fn messages(
        &self,
        session_id: &str,
    ) -> Result<ConversationMessagesResponse, reqwest::Error> {
        self.get_json(&format!(
            "/v1/conversations/messages?session_id={}",
            url_encode_query_component(session_id)
        ))
        .await
    }

    pub async fn delete_messages(
        &self,
        session_id: &str,
    ) -> Result<ConversationDeleteResponse, reqwest::Error> {
        self.http
            .delete(self.url(&format!(
                "/v1/conversations/messages?session_id={}",
                url_encode_query_component(session_id)
            )))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn manage(
        &self,
        request: &ManageConversationRequest,
    ) -> Result<ManageConversationResponse, reqwest::Error> {
        self.http
            .put(self.url("/v1/conversations/manage"))
            .json(request)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn rename(
        &self,
        session_id: &str,
        name: &str,
    ) -> Result<ManageConversationResponse, reqwest::Error> {
        self.manage(&ManageConversationRequest {
            session_id: session_id.to_string(),
            name: name.to_string(),
            ..Default::default()
        })
        .await
    }

    pub async fn pin(
        &self,
        session_id: &str,
        pinned: bool,
    ) -> Result<ManageConversationResponse, reqwest::Error> {
        self.manage(&ManageConversationRequest {
            session_id: session_id.to_string(),
            pinned: Some(pinned),
            ..Default::default()
        })
        .await
    }

    pub async fn archive(
        &self,
        session_id: &str,
        archive: bool,
    ) -> Result<ManageConversationResponse, reqwest::Error> {
        self.manage(&ManageConversationRequest {
            session_id: session_id.to_string(),
            archive: Some(archive),
            ..Default::default()
        })
        .await
    }

    pub async fn replay(
        &self,
        session_id: &str,
        opts: ConversationReplayOptions,
    ) -> Result<ConversationReplayResponse, reqwest::Error> {
        let mut query = vec![("session_id".to_string(), session_id.to_string())];
        if opts.raw {
            query.push(("raw".to_string(), "true".to_string()));
        }
        if opts.limit > 0 {
            query.push(("limit".to_string(), opts.limit.to_string()));
        }
        if opts.offset > 0 {
            query.push(("offset".to_string(), opts.offset.to_string()));
        }
        let encoded = query
            .into_iter()
            .map(|(k, v)| {
                format!(
                    "{}={}",
                    url_encode_query_component(&k),
                    url_encode_query_component(&v)
                )
            })
            .collect::<Vec<_>>()
            .join("&");
        self.get_json(&format!("/v1/conversations/replay?{encoded}"))
            .await
    }

    async fn get_json<T>(&self, path: &str) -> Result<T, reqwest::Error>
    where
        T: for<'de> Deserialize<'de>,
    {
        self.http
            .get(self.url(path))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ChatMessage {
    pub role: String,
    pub content: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub name: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ChatRequest {
    pub messages: Vec<ChatMessage>,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub session_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub task_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub class_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub teacher_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub student_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub platform: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub thinking_level: String,
    #[serde(default, skip_serializing_if = "is_default")]
    pub stream: bool,
}

pub type ChatResponse = serde_json::Value;

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ChatStreamItem {
    pub kind: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub event: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub content: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub message: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub data: Option<serde_json::Value>,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub raw: String,
}

/// Small Rust helper over `/v1/chat`, `/v1/chat/stream`, and `/v1/chat/agentic`.
#[derive(Debug, Clone)]
pub struct ChatClient {
    base_url: String,
    http: reqwest::Client,
}

impl ChatClient {
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let value = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&value) {
                headers.insert(AUTHORIZATION, value);
            }
        }
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: trim_base_url(base_url.into()),
            http,
        }
    }

    pub fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }

    pub async fn send(&self, request: &ChatRequest) -> Result<ChatResponse, reqwest::Error> {
        self.post_json("/v1/chat", request).await
    }

    pub async fn agentic(&self, request: &ChatRequest) -> Result<ChatResponse, reqwest::Error> {
        self.post_json("/v1/chat/agentic", request).await
    }

    pub fn stream_url(&self) -> String {
        self.url("/v1/chat/stream")
    }

    pub fn stream_request(&self, request: ChatRequest) -> ChatRequest {
        ChatRequest {
            stream: true,
            ..request
        }
    }

    pub fn parse_stream(&self, text: &str) -> Vec<ChatStreamItem> {
        parse_sse_events(text)
            .into_iter()
            .filter(|event| event.data.as_ref().is_some_and(|data| data != "[DONE]"))
            .map(|event| chat_stream_item_from_event(event))
            .collect()
    }

    async fn post_json<B, T>(&self, path: &str, body: &B) -> Result<T, reqwest::Error>
    where
        B: Serialize + ?Sized,
        T: for<'de> Deserialize<'de>,
    {
        self.http
            .post(self.url(path))
            .json(body)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }
}

fn chat_stream_item_from_event(event: EventStreamMessage) -> ChatStreamItem {
    let mut item = ChatStreamItem {
        kind: if event.event == "message" {
            "raw".to_string()
        } else {
            event.event.clone()
        },
        event: event.event,
        data: event.data.clone(),
        raw: event.raw,
        ..Default::default()
    };
    if let Some(serde_json::Value::Object(data)) = &event.data {
        if let Some(content) = data.get("content").and_then(|value| value.as_str()) {
            item.kind = "delta".to_string();
            item.content = content.to_string();
        }
        if data.get("type").and_then(|value| value.as_str()) == Some("error")
            || data.get("error").is_some()
        {
            item.kind = "error".to_string();
            item.message = data
                .get("error")
                .or_else(|| data.get("message"))
                .and_then(|value| value.as_str())
                .unwrap_or("")
                .to_string();
        }
    }
    item
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct RealtimeMessage {
    #[serde(rename = "type")]
    pub r#type: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub content: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub session: String,
    #[serde(flatten)]
    pub extra: serde_json::Map<String, serde_json::Value>,
}

/// Small Rust helper over `/v1/ws` realtime WebSocket chat integration.
#[derive(Debug, Clone)]
pub struct RealtimeClient {
    base_url: String,
    token: String,
    api_key: String,
    http: reqwest::Client,
}

impl RealtimeClient {
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let value = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&value) {
                headers.insert(AUTHORIZATION, value);
            }
        }
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self {
            base_url: trim_base_url(base_url.into()),
            token: token.to_string(),
            api_key: String::new(),
            http,
        })
    }

    pub fn new_with_api_key(
        base_url: impl Into<String>,
        api_key: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        Ok(Self {
            base_url: trim_base_url(base_url.into()),
            token: String::new(),
            api_key: api_key.as_ref().to_string(),
            http: reqwest::Client::new(),
        })
    }

    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: trim_base_url(base_url.into()),
            token: String::new(),
            api_key: String::new(),
            http,
        }
    }

    pub fn ws_url(&self) -> String {
        self.ws_url_with_query(&[])
    }

    pub fn ws_url_with_query(&self, query: &[(&str, &str)]) -> String {
        let mut base = self.base_url.clone();
        if let Some(rest) = base.strip_prefix("http://") {
            base = format!("ws://{rest}");
        } else if let Some(rest) = base.strip_prefix("https://") {
            base = format!("wss://{rest}");
        }
        let mut params: Vec<(String, String)> = query
            .iter()
            .filter(|(_, value)| !value.is_empty())
            .map(|(key, value)| (key.to_string(), value.to_string()))
            .collect();
        if !params
            .iter()
            .any(|(key, _)| matches!(key.as_str(), "key" | "api_key" | "token" | "access_token"))
        {
            if !self.api_key.is_empty() {
                params.push(("api_key".to_string(), self.api_key.clone()));
            } else if !self.token.is_empty() {
                params.push(("access_token".to_string(), self.token.clone()));
            }
        }
        if params.is_empty() {
            return format!("{base}/v1/ws");
        }
        let encoded = params
            .into_iter()
            .map(|(key, value)| {
                format!(
                    "{}={}",
                    encode_query_component(&key),
                    encode_query_component(&value)
                )
            })
            .collect::<Vec<_>>()
            .join("&");
        format!("{base}/v1/ws?{encoded}")
    }

    pub fn ping(&self, extra: serde_json::Map<String, serde_json::Value>) -> RealtimeMessage {
        RealtimeMessage {
            r#type: "ping".to_string(),
            extra,
            ..Default::default()
        }
    }

    pub fn chat(
        &self,
        content: impl Into<String>,
        session: impl Into<String>,
        extra: serde_json::Map<String, serde_json::Value>,
    ) -> RealtimeMessage {
        RealtimeMessage {
            r#type: "chat".to_string(),
            content: content.into(),
            session: session.into(),
            extra,
        }
    }

    pub fn serialize(&self, message: &RealtimeMessage) -> Result<String, serde_json::Error> {
        serde_json::to_string(message)
    }

    pub fn parse(&self, text: &str) -> Result<RealtimeMessage, serde_json::Error> {
        serde_json::from_str(text)
    }

    pub fn http_client(&self) -> &reqwest::Client {
        &self.http
    }
}

fn encode_query_component(value: &str) -> String {
    let mut out = String::new();
    for b in value.bytes() {
        match b {
            b'A'..=b'Z' | b'a'..=b'z' | b'0'..=b'9' | b'-' | b'_' | b'.' | b'~' => {
                out.push(b as char)
            }
            b' ' => out.push('+'),
            _ => out.push_str(&format!("%{b:02X}")),
        }
    }
    out
}

pub type ReverieThought = serde_json::Map<String, serde_json::Value>;
pub type ReverieConfig = serde_json::Map<String, serde_json::Value>;
pub type ReverieConfigResponse = serde_json::Value;
pub type ReverieThinkResponse = serde_json::Value;
pub type ReverieDeleteResponse = serde_json::Value;
pub type ReverieActionsResponse = serde_json::Value;
pub type ReverieTargetsResponse = serde_json::Value;

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ReverieJournalResponse {
    #[serde(default)]
    pub thoughts: Vec<ReverieThought>,
    #[serde(default)]
    pub total: i32,
    #[serde(default)]
    pub limit: i32,
    #[serde(default)]
    pub offset: i32,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ReverieJournalQuery {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub category: String,
    #[serde(default, skip_serializing_if = "is_default")]
    pub min_significance: f64,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub delivered: Option<bool>,
    #[serde(default, skip_serializing_if = "is_default")]
    pub limit: i32,
    #[serde(default, skip_serializing_if = "is_default")]
    pub offset: i32,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ReverieThinkRequest {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub event_type: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub trigger: String,
}

/// Small Rust helper over `/v1/reverie/*` proactive thought loop endpoints.
#[derive(Debug, Clone)]
pub struct ReverieClient {
    base_url: String,
    http: reqwest::Client,
}

impl ReverieClient {
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let value = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&value) {
                headers.insert(AUTHORIZATION, value);
            }
        }
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: trim_base_url(base_url.into()),
            http,
        }
    }

    pub fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }

    pub async fn journal(
        &self,
        query: &ReverieJournalQuery,
    ) -> Result<ReverieJournalResponse, reqwest::Error> {
        let mut pairs = Vec::new();
        if !query.category.is_empty() {
            pairs.push(format!(
                "category={}",
                url_encode_query_component(&query.category)
            ));
        }
        if query.min_significance > 0.0 {
            pairs.push(format!("min_significance={}", query.min_significance));
        }
        if let Some(delivered) = query.delivered {
            pairs.push(format!("delivered={delivered}"));
        }
        if query.limit > 0 {
            pairs.push(format!("limit={}", query.limit));
        }
        if query.offset > 0 {
            pairs.push(format!("offset={}", query.offset));
        }
        let suffix = if pairs.is_empty() {
            String::new()
        } else {
            format!("?{}", pairs.join("&"))
        };
        self.get_json(&format!("/v1/reverie/journal{}", suffix))
            .await
    }

    pub async fn stats(&self) -> Result<serde_json::Value, reqwest::Error> {
        self.get_json("/v1/reverie/stats").await
    }

    pub async fn config(&self) -> Result<ReverieConfigResponse, reqwest::Error> {
        self.get_json("/v1/reverie/config").await
    }

    pub async fn update_config(
        &self,
        config: &ReverieConfig,
    ) -> Result<ReverieConfigResponse, reqwest::Error> {
        self.http
            .put(self.url("/v1/reverie/config"))
            .json(config)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn think(
        &self,
        request: &ReverieThinkRequest,
    ) -> Result<ReverieThinkResponse, reqwest::Error> {
        self.http
            .post(self.url("/v1/reverie/think"))
            .json(request)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn delete_thought(&self, id: &str) -> Result<ReverieDeleteResponse, reqwest::Error> {
        self.http
            .delete(self.url(&format!(
                "/v1/reverie/thought?id={}",
                url_encode_query_component(id)
            )))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn actions(&self) -> Result<ReverieActionsResponse, reqwest::Error> {
        self.get_json("/v1/reverie/actions").await
    }

    pub async fn targets(&self) -> Result<ReverieTargetsResponse, reqwest::Error> {
        self.get_json("/v1/reverie/targets").await
    }

    async fn get_json<T>(&self, path: &str) -> Result<T, reqwest::Error>
    where
        T: for<'de> Deserialize<'de>,
    {
        self.http
            .get(self.url(path))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct CostSummaryResponse {
    #[serde(default)]
    pub today_cost: f64,
    #[serde(default)]
    pub month_cost: f64,
    #[serde(flatten)]
    pub extra: serde_json::Map<String, serde_json::Value>,
}

pub type CostBudget = serde_json::Map<String, serde_json::Value>;
pub type SetCostBudgetResponse = serde_json::Value;
pub type CostTaskResponse = serde_json::Value;
pub type CostTimelineResponse = serde_json::Value;
pub type CostBreakdownResponse = serde_json::Value;
pub type CostHistoryResponse = serde_json::Value;
pub type CostAlertsResponse = serde_json::Value;
pub type UsageResponse = serde_json::Value;
pub type SetQuotaResponse = serde_json::Value;

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct CostHistoryQuery {
    #[serde(default, skip_serializing_if = "is_default")]
    pub page: i32,
    #[serde(default, skip_serializing_if = "is_default")]
    pub limit: i32,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub task_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub model: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub channel: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub runner_type: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub provider_id: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct SetQuotaRequest {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub tenant_id: String,
    pub quota: serde_json::Value,
}

/// Small Rust helper over host `/v1/cost/*`, `/v1/usage`, and `/v1/quota` endpoints.
#[derive(Debug, Clone)]
pub struct CostClient {
    base_url: String,
    http: reqwest::Client,
}

impl CostClient {
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let value = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&value) {
                headers.insert(AUTHORIZATION, value);
            }
        }
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: trim_base_url(base_url.into()),
            http,
        }
    }

    pub fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }

    pub async fn summary(&self) -> Result<CostSummaryResponse, reqwest::Error> {
        self.http
            .get(self.url("/v1/cost/summary"))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn set_budget(
        &self,
        budget: &CostBudget,
    ) -> Result<SetCostBudgetResponse, reqwest::Error> {
        self.http
            .post(self.url("/v1/cost/budget"))
            .json(budget)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn task(&self, id: &str) -> Result<CostTaskResponse, reqwest::Error> {
        self.http
            .get(self.url(&format!(
                "/v1/cost/task?id={}",
                url_encode_query_component(id)
            )))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn task_timeline(&self, id: &str) -> Result<CostTimelineResponse, reqwest::Error> {
        self.http
            .get(self.url(&format!(
                "/v1/cost/task/timeline?id={}",
                url_encode_query_component(id)
            )))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn breakdown(&self) -> Result<CostBreakdownResponse, reqwest::Error> {
        self.http
            .get(self.url("/v1/cost/breakdown"))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn history(
        &self,
        query: &CostHistoryQuery,
    ) -> Result<CostHistoryResponse, reqwest::Error> {
        let mut params = Vec::new();
        if query.page > 0 {
            params.push(format!("page={}", query.page));
        }
        if query.limit > 0 {
            params.push(format!("limit={}", query.limit));
        }
        if !query.task_id.is_empty() {
            params.push(format!(
                "task_id={}",
                url_encode_query_component(&query.task_id)
            ));
        }
        if !query.model.is_empty() {
            params.push(format!(
                "model={}",
                url_encode_query_component(&query.model)
            ));
        }
        if !query.channel.is_empty() {
            params.push(format!(
                "channel={}",
                url_encode_query_component(&query.channel)
            ));
        }
        if !query.runner_type.is_empty() {
            params.push(format!(
                "runner_type={}",
                url_encode_query_component(&query.runner_type)
            ));
        }
        if !query.provider_id.is_empty() {
            params.push(format!(
                "provider_id={}",
                url_encode_query_component(&query.provider_id)
            ));
        }
        let suffix = if params.is_empty() {
            String::new()
        } else {
            format!("?{}", params.join("&"))
        };
        self.http
            .get(self.url(&format!("/v1/cost/history{}", suffix)))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn alerts(&self) -> Result<CostAlertsResponse, reqwest::Error> {
        self.http
            .get(self.url("/v1/cost/alerts"))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn usage(&self) -> Result<UsageResponse, reqwest::Error> {
        self.http
            .get(self.url("/v1/usage"))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn set_quota(
        &self,
        request: &SetQuotaRequest,
    ) -> Result<SetQuotaResponse, reqwest::Error> {
        self.http
            .post(self.url("/v1/quota"))
            .json(request)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }
}

/// Small Rust helper over host `/v1/fork*` conversation branch endpoints.
#[derive(Debug, Clone)]
pub struct ForkClient {
    base_url: String,
    http: reqwest::Client,
}

impl ForkClient {
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let value = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&value) {
                headers.insert(AUTHORIZATION, value);
            }
        }
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: trim_base_url(base_url.into()),
            http,
        }
    }

    pub fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }

    pub async fn root(&self, session_id: &str) -> Result<ForkRootResponse, reqwest::Error> {
        self.http
            .get(self.url(&format!(
                "/v1/fork?session_id={}",
                url_encode_query_component(session_id)
            )))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn get(&self, id: &str) -> Result<ConversationFork, reqwest::Error> {
        self.http
            .get(self.url(&format!("/v1/fork?id={}", url_encode_query_component(id))))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn create(
        &self,
        request: &ForkCreateRequest,
    ) -> Result<ConversationFork, reqwest::Error> {
        self.http
            .post(self.url("/v1/fork"))
            .json(request)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn remove(&self, id: &str) -> Result<ForkDeleteResponse, reqwest::Error> {
        self.http
            .delete(self.url(&format!("/v1/fork?id={}", url_encode_query_component(id))))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn branch(
        &self,
        request: &ForkBranchRequest,
    ) -> Result<ConversationFork, reqwest::Error> {
        self.http
            .post(self.url("/v1/fork/branch"))
            .json(request)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn list(&self, session_id: &str) -> Result<ForkListResponse, reqwest::Error> {
        self.http
            .get(self.url(&format!(
                "/v1/fork/list?session_id={}",
                url_encode_query_component(session_id)
            )))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }
}

/// Skill marketplace entry returned by `/v1/market/*`.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct SkillMarketSkill {
    pub name: String,
    #[serde(default)]
    pub version: String,
    #[serde(default)]
    pub description: String,
    #[serde(default)]
    pub author: String,
    #[serde(default)]
    pub category: String,
    #[serde(default)]
    pub tags: Vec<String>,
    #[serde(default)]
    pub license: String,
    #[serde(default)]
    pub homepage: String,
    #[serde(default)]
    pub deprecated: bool,
    #[serde(default)]
    pub installs: i32,
    #[serde(default)]
    pub rating: f64,
    #[serde(default)]
    pub rating_count: i32,
    #[serde(default)]
    pub created_at: String,
    #[serde(default)]
    pub updated_at: String,
    #[serde(default)]
    pub min_version: String,
    #[serde(default)]
    pub dependencies: Vec<String>,
    #[serde(flatten)]
    pub extra: serde_json::Map<String, serde_json::Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct SkillMarketSearchResponse {
    #[serde(default)]
    pub skills: Vec<SkillMarketSkill>,
    #[serde(default)]
    pub count: i32,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct SkillMarketTopOptions {
    #[serde(default, skip_serializing_if = "is_default")]
    pub n: i32,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub by: String,
}

pub type SkillMarketStatsResponse = serde_json::Map<String, serde_json::Value>;

/// Small Rust helper over host `/v1/market/*` skill marketplace endpoints.
#[derive(Debug, Clone)]
pub struct SkillMarketClient {
    base_url: String,
    http: reqwest::Client,
}

impl SkillMarketClient {
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let value = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&value) {
                headers.insert(AUTHORIZATION, value);
            }
        }
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: trim_base_url(base_url.into()),
            http,
        }
    }

    pub fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }

    pub async fn search(
        &self,
        query: impl AsRef<str>,
    ) -> Result<SkillMarketSearchResponse, reqwest::Error> {
        let query = query.as_ref();
        let path = if query.is_empty() {
            "/v1/market/search".to_string()
        } else {
            format!("/v1/market/search?q={}", url_encode_query_component(query))
        };
        self.http
            .get(self.url(&path))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn top(
        &self,
        options: &SkillMarketTopOptions,
    ) -> Result<SkillMarketSearchResponse, reqwest::Error> {
        let mut query: Vec<(&str, String)> = Vec::new();
        if options.n > 0 {
            query.push(("n", options.n.to_string()));
        }
        if !options.by.is_empty() {
            query.push(("by", options.by.clone()));
        }
        let suffix = if query.is_empty() {
            String::new()
        } else {
            format!(
                "?{}",
                query
                    .into_iter()
                    .map(|(key, value)| format!("{key}={}", url_encode_query_component(&value)))
                    .collect::<Vec<_>>()
                    .join("&")
            )
        };
        self.http
            .get(self.url(&format!("/v1/market/top{suffix}")))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn stats(&self) -> Result<SkillMarketStatsResponse, reqwest::Error> {
        self.http
            .get(self.url("/v1/market/stats"))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }
}

/// Project workspace record managed by `/v1/projects*`.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct Project {
    #[serde(default)]
    pub id: String,
    pub name: String,
    pub repo_path: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub repo_url: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub description: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub default_caps: Vec<String>,
    #[serde(default, skip_serializing_if = "std::collections::BTreeMap::is_empty")]
    pub meta: std::collections::BTreeMap<String, String>,
    #[serde(default)]
    pub created_at: String,
    #[serde(default)]
    pub updated_at: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ProjectsListResponse {
    #[serde(default)]
    pub projects: Vec<Project>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct CreateProjectRequest {
    pub name: String,
    pub repo_path: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub repo_url: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub description: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub default_caps: Vec<String>,
    #[serde(default, skip_serializing_if = "std::collections::BTreeMap::is_empty")]
    pub meta: std::collections::BTreeMap<String, String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct UpdateProjectRequest {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub name: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub repo_path: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub repo_url: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub description: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub default_caps: Vec<String>,
    #[serde(default, skip_serializing_if = "std::collections::BTreeMap::is_empty")]
    pub meta: std::collections::BTreeMap<String, String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct DeleteProjectResponse {
    #[serde(default)]
    pub status: String,
}

/// Small Rust helper over host `/v1/projects*` project workspace endpoints.
#[derive(Debug, Clone)]
pub struct ProjectsClient {
    base_url: String,
    http: reqwest::Client,
}

impl ProjectsClient {
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let value = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&value) {
                headers.insert(AUTHORIZATION, value);
            }
        }
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: trim_base_url(base_url.into()),
            http,
        }
    }

    pub fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }

    pub async fn list(&self) -> Result<ProjectsListResponse, reqwest::Error> {
        self.http
            .get(self.url("/v1/projects"))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn create(&self, request: &CreateProjectRequest) -> Result<Project, reqwest::Error> {
        self.http
            .post(self.url("/v1/projects"))
            .json(request)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn detail(&self, id: &str) -> Result<Project, reqwest::Error> {
        self.http
            .get(self.url(&format!(
                "/v1/projects/detail?id={}",
                url_encode_query_component(id)
            )))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn update(
        &self,
        id: &str,
        request: &UpdateProjectRequest,
    ) -> Result<Project, reqwest::Error> {
        self.http
            .put(self.url(&format!(
                "/v1/projects/detail?id={}",
                url_encode_query_component(id)
            )))
            .json(request)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn remove(&self, id: &str) -> Result<DeleteProjectResponse, reqwest::Error> {
        self.http
            .post(self.url("/v1/projects/remove"))
            .json(&serde_json::json!({ "id": id }))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }
}

/// Small Rust helper over connector catalog, auth, and action execution endpoints.
#[derive(Debug, Clone)]
pub struct ConnectorsClient {
    base_url: String,
    http: reqwest::Client,
}

impl ConnectorsClient {
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let value = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&value) {
                headers.insert(AUTHORIZATION, value);
            }
        }
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: trim_base_url(base_url.into()),
            http,
        }
    }

    pub async fn list(&self) -> Result<ConnectorListResponse, reqwest::Error> {
        self.http
            .get(self.url("/api/connectors"))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn detail(
        &self,
        id: impl AsRef<str>,
    ) -> Result<ConnectorDetailResponse, reqwest::Error> {
        self.http
            .get(self.url(&format!(
                "/api/connectors/detail?id={}",
                url_encode_query_component(id.as_ref())
            )))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn connect(
        &self,
        request: &ConnectorConnectRequest,
    ) -> Result<ConnectorConnectResponse, reqwest::Error> {
        self.http
            .post(self.url("/api/connectors/connect"))
            .json(request)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn disconnect(
        &self,
        connector_id: impl AsRef<str>,
    ) -> Result<ConnectorOkResponse, reqwest::Error> {
        #[derive(Serialize)]
        struct DisconnectRequest<'a> {
            connector_id: &'a str,
        }
        self.http
            .post(self.url("/api/connectors/disconnect"))
            .json(&DisconnectRequest {
                connector_id: connector_id.as_ref(),
            })
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn execute(
        &self,
        request: &ConnectorExecuteRequest,
    ) -> Result<ConnectorExecuteResponse, reqwest::Error> {
        self.http
            .post(self.url("/api/connectors/execute"))
            .json(request)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }
}

/// Small Rust helper over notification channels, tests, and share dispatch endpoints.
#[derive(Debug, Clone)]
pub struct NotifyClient {
    base_url: String,
    http: reqwest::Client,
}

impl NotifyClient {
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let value = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&value) {
                headers.insert(AUTHORIZATION, value);
            }
        }
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: trim_base_url(base_url.into()),
            http,
        }
    }

    pub async fn channels(&self) -> Result<NotifyChannelsResponse, reqwest::Error> {
        self.http
            .get(self.url("/api/notify/channels"))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn add_channel(
        &self,
        channel: &NotifyChannel,
    ) -> Result<NotifyOkResponse, reqwest::Error> {
        self.http
            .post(self.url("/api/notify/add"))
            .json(channel)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn remove_channel(
        &self,
        id: impl AsRef<str>,
    ) -> Result<NotifyOkResponse, reqwest::Error> {
        self.http
            .post(self.url(&format!(
                "/api/notify/remove?id={}",
                url_encode_query_component(id.as_ref())
            )))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn toggle_channel(
        &self,
        request: &NotifyToggleRequest,
    ) -> Result<NotifyOkResponse, reqwest::Error> {
        self.http
            .post(self.url("/api/notify/toggle"))
            .json(request)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn test_channel(
        &self,
        id: impl AsRef<str>,
    ) -> Result<NotifyOkResponse, reqwest::Error> {
        #[derive(Serialize)]
        struct TestRequest<'a> {
            id: &'a str,
        }
        self.http
            .post(self.url("/api/notify/test"))
            .json(&TestRequest { id: id.as_ref() })
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn share(
        &self,
        request: &NotifyShareRequest,
    ) -> Result<NotifyShareResponse, reqwest::Error> {
        self.http
            .post(self.url("/api/notify/share"))
            .json(request)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }
}

/// Small Rust helper over host `/v1/lora/*` local-brain training lifecycle endpoints.
#[derive(Debug, Clone)]
pub struct LoRAClient {
    base_url: String,
    http: reqwest::Client,
}

impl LoRAClient {
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let value = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&value) {
                headers.insert(AUTHORIZATION, value);
            }
        }
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: trim_base_url(base_url.into()),
            http,
        }
    }

    pub async fn status(&self) -> Result<LoRAStatusResponse, reqwest::Error> {
        self.get_map("/v1/lora/status").await
    }

    pub async fn history(&self) -> Result<LoRAHistoryResponse, reqwest::Error> {
        self.get_map("/v1/lora/history").await
    }

    pub async fn summary(&self) -> Result<LoRASummaryResponse, reqwest::Error> {
        self.get_map("/v1/lora/summary").await
    }

    pub async fn preview(
        &self,
        options: &LoRAPreviewOptions,
    ) -> Result<serde_json::Map<String, serde_json::Value>, reqwest::Error> {
        let path = if options.tenant_id.is_empty() {
            "/v1/lora/preview".to_string()
        } else {
            format!(
                "/v1/lora/preview?tenant_id={}",
                url_encode_query_component(&options.tenant_id)
            )
        };
        self.get_map(&path).await
    }

    pub async fn trigger(
        &self,
        request: &TriggerLoRARequest,
    ) -> Result<TriggerLoRAResponse, reqwest::Error> {
        self.http
            .post(self.url("/v1/lora/trigger"))
            .json(request)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn rollback(&self) -> Result<LoRARollbackResponse, reqwest::Error> {
        self.http
            .post(self.url("/v1/lora/rollback"))
            .json(&serde_json::json!({}))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    pub async fn evolution(&self) -> Result<LoRAEvolutionResponse, reqwest::Error> {
        self.get_map("/v1/lora/evolution").await
    }

    pub async fn config(&self) -> Result<LoRAConfigResponse, reqwest::Error> {
        self.get_map("/v1/lora/config").await
    }

    pub async fn update_config(
        &self,
        config: &LoRAConfig,
    ) -> Result<LoRAConfigResponse, reqwest::Error> {
        self.http
            .put(self.url("/v1/lora/config"))
            .json(config)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    async fn get_map(
        &self,
        path: &str,
    ) -> Result<serde_json::Map<String, serde_json::Value>, reqwest::Error> {
        self.http
            .get(self.url(path))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }
}

/// Small Rust helper over `/v1/triggers/v2*` automation endpoints.
///
/// Use this when an external Rust CLI, sidecar, plugin runner, or automation
/// binary wants trigger definitions, event emission, or recent trigger history
/// without importing the generated all-in-one API surface.
#[derive(Debug, Clone)]
pub struct TriggersClient {
    base_url: String,
    http: reqwest::Client,
}

impl TriggersClient {
    /// Create a TriggersClient using a bearer token.
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let value = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&value) {
                headers.insert(AUTHORIZATION, value);
            }
        }
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    /// Create a TriggersClient with a caller-provided reqwest client.
    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: trim_base_url(base_url.into()),
            http,
        }
    }

    /// List Triggers v2 definitions.
    pub async fn list(
        &self,
        options: &TriggerListOptions,
    ) -> Result<TriggerListResponse, reqwest::Error> {
        self.http
            .get(self.url(&format!("/v1/triggers/v2{}", trigger_list_query(options))))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    /// Get one Triggers v2 definition by id.
    pub async fn get(&self, id: impl AsRef<str>) -> Result<TriggerDef, reqwest::Error> {
        self.http
            .get(self.url(&format!("/v1/triggers/v2?id={}", id.as_ref())))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    /// Create a Triggers v2 definition.
    pub async fn create(&self, definition: &TriggerDef) -> Result<TriggerDef, reqwest::Error> {
        self.http
            .post(self.url("/v1/triggers/v2"))
            .json(definition)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    /// Update a Triggers v2 definition.
    pub async fn update(&self, definition: &TriggerDef) -> Result<TriggerDef, reqwest::Error> {
        self.http
            .put(self.url("/v1/triggers/v2"))
            .json(definition)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    /// Delete a Triggers v2 definition by id.
    pub async fn delete(
        &self,
        id: impl AsRef<str>,
    ) -> Result<TriggerDeleteResponse, reqwest::Error> {
        self.http
            .delete(self.url(&format!("/v1/triggers/v2?id={}", id.as_ref())))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    /// Emit an event to the Triggers v2 automation runtime.
    pub async fn emit(
        &self,
        payload: &TriggerPayload,
    ) -> Result<TriggerEmitResponse, reqwest::Error> {
        self.http
            .post(self.url("/v1/triggers/v2/emit"))
            .json(payload)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    /// List recent trigger runs.
    pub async fn runs(
        &self,
        options: &TriggerHistoryOptions,
    ) -> Result<TriggerRunsResponse, reqwest::Error> {
        self.http
            .get(self.url(&format!(
                "/v1/triggers/v2/runs{}",
                trigger_history_query(options)
            )))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    /// List recent trigger events.
    pub async fn events(
        &self,
        options: &TriggerHistoryOptions,
    ) -> Result<TriggerEventsResponse, reqwest::Error> {
        self.http
            .get(self.url(&format!(
                "/v1/triggers/v2/events{}",
                trigger_history_query(options)
            )))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }
}

/// Small Rust helper over `/v1/scheduler/*`.
///
/// Use this when an external Rust CLI, sidecar, plugin runner, or automation
/// binary wants to list, add, or remove prompt scheduler jobs without importing
/// the generated all-in-one API surface.
#[derive(Debug, Clone)]
pub struct SchedulerClient {
    base_url: String,
    http: reqwest::Client,
}

impl SchedulerClient {
    /// Create a SchedulerClient using a bearer token.
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let value = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&value) {
                headers.insert(AUTHORIZATION, value);
            }
        }
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    /// Create a SchedulerClient with a caller-provided reqwest client.
    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: trim_base_url(base_url.into()),
            http,
        }
    }

    /// List prompt scheduler jobs.
    pub async fn jobs(&self) -> Result<SchedulerJobsResponse, reqwest::Error> {
        self.http
            .get(self.url("/v1/scheduler/jobs"))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    /// Add a recurring prompt scheduler job. Interval uses Go duration strings such as `1h`.
    pub async fn add(&self, request: &SchedulerAddRequest) -> Result<SchedulerJob, reqwest::Error> {
        self.http
            .post(self.url("/v1/scheduler/add"))
            .json(request)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    /// Remove a prompt scheduler job by id.
    pub async fn remove(
        &self,
        id: impl AsRef<str>,
    ) -> Result<SchedulerRemoveResponse, reqwest::Error> {
        self.http
            .post(self.url("/v1/scheduler/remove"))
            .json(&serde_json::json!({ "id": id.as_ref() }))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }
}

/// Small Rust helper over `/v1/missions/parse`.
///
/// Use this when an external Rust CLI, sidecar, plugin runner, or automation
/// binary wants to turn natural-language user intent into a structured mission
/// draft without importing the generated all-in-one API surface.
#[derive(Debug, Clone)]
pub struct MissionsClient {
    base_url: String,
    http: reqwest::Client,
}

impl MissionsClient {
    /// Create a MissionsClient using a bearer token.
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let value = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&value) {
                headers.insert(AUTHORIZATION, value);
            }
        }
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    /// Create a MissionsClient with a caller-provided reqwest client.
    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: trim_base_url(base_url.into()),
            http,
        }
    }

    /// Parse a natural-language mission description into a structured draft.
    pub async fn parse(
        &self,
        description: impl AsRef<str>,
    ) -> Result<MissionParseResult, reqwest::Error> {
        self.http
            .post(self.url("/v1/missions/parse"))
            .json(&serde_json::json!({ "description": description.as_ref() }))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }
}

/// Small Rust helper over the core `/v1/plugin-api/*` runtime capabilities.
///
/// Use this when a Rust CLI, sidecar, or plugin runner only needs runtime
/// calls without coupling to the full generated OpenAPI client.
#[derive(Debug, Clone)]
pub struct PluginApiClient {
    base_url: String,
    http: reqwest::Client,
}

impl PluginApiClient {
    /// Create a PluginApiClient using a plugin-scoped bearer token.
    pub fn new(
        base_url: impl Into<String>,
        token: impl AsRef<str>,
    ) -> Result<Self, reqwest::Error> {
        let token = token.as_ref();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if !token.is_empty() {
            let value = format!("Bearer {token}");
            if let Ok(value) = HeaderValue::from_str(&value) {
                headers.insert(AUTHORIZATION, value);
            }
        }
        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        Ok(Self::new_with_client(base_url, http))
    }

    /// Create a PluginApiClient with a caller-provided reqwest client.
    pub fn new_with_client(base_url: impl Into<String>, http: reqwest::Client) -> Self {
        Self {
            base_url: trim_base_url(base_url.into()),
            http,
        }
    }

    /// Call the configured LLM through `/v1/plugin-api/llm`.
    pub async fn llm(
        &self,
        request: &PluginLLMRequest,
    ) -> Result<PluginLLMResponse, reqwest::Error> {
        self.http
            .post(self.url("/v1/plugin-api/llm"))
            .json(request)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    /// Search through configured providers via `/v1/plugin-api/search`.
    pub async fn search(
        &self,
        query: impl AsRef<str>,
        limit: i32,
    ) -> Result<PluginSearchResponse, reqwest::Error> {
        #[derive(Serialize)]
        struct SearchRequest<'a> {
            query: &'a str,
            #[serde(skip_serializing_if = "is_default")]
            limit: i32,
        }
        self.http
            .post(self.url("/v1/plugin-api/search"))
            .json(&SearchRequest {
                query: query.as_ref(),
                limit,
            })
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    /// Send a channel message via `/v1/plugin-api/send`.
    pub async fn send(
        &self,
        channel: impl AsRef<str>,
        target: impl AsRef<str>,
        content: impl AsRef<str>,
        format: impl AsRef<str>,
    ) -> Result<PluginSendResponse, reqwest::Error> {
        #[derive(Serialize)]
        struct SendRequest<'a> {
            channel: &'a str,
            target: &'a str,
            content: &'a str,
            #[serde(skip_serializing_if = "str::is_empty")]
            format: &'a str,
        }
        self.http
            .post(self.url("/v1/plugin-api/send"))
            .json(&SendRequest {
                channel: channel.as_ref(),
                target: target.as_ref(),
                content: content.as_ref(),
                format: format.as_ref(),
            })
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    /// Read plugin-private memory.
    pub async fn memory_get(
        &self,
        key: impl AsRef<str>,
    ) -> Result<PluginMemoryValueResponse, reqwest::Error> {
        #[derive(Serialize)]
        struct KeyRequest<'a> {
            key: &'a str,
        }
        self.post_json(
            "/v1/plugin-api/memory/get",
            &KeyRequest { key: key.as_ref() },
        )
        .await
    }

    /// Write plugin-private memory.
    pub async fn memory_set(
        &self,
        key: impl AsRef<str>,
        value: impl AsRef<str>,
    ) -> Result<PluginOkResponse, reqwest::Error> {
        #[derive(Serialize)]
        struct MemorySetRequest<'a> {
            key: &'a str,
            value: &'a str,
        }
        self.post_json(
            "/v1/plugin-api/memory/set",
            &MemorySetRequest {
                key: key.as_ref(),
                value: value.as_ref(),
            },
        )
        .await
    }

    /// Delete plugin-private memory.
    pub async fn memory_delete(
        &self,
        key: impl AsRef<str>,
    ) -> Result<PluginOkResponse, reqwest::Error> {
        #[derive(Serialize)]
        struct KeyRequest<'a> {
            key: &'a str,
        }
        self.post_json(
            "/v1/plugin-api/memory/delete",
            &KeyRequest { key: key.as_ref() },
        )
        .await
    }

    /// List plugin-private memory entries.
    pub async fn memory_list(
        &self,
        prefix: impl AsRef<str>,
    ) -> Result<PluginMemoryListResponse, reqwest::Error> {
        #[derive(Serialize)]
        struct MemoryListRequest<'a> {
            #[serde(skip_serializing_if = "str::is_empty")]
            prefix: &'a str,
        }
        self.post_json(
            "/v1/plugin-api/memory/list",
            &MemoryListRequest {
                prefix: prefix.as_ref(),
            },
        )
        .await
    }

    /// Search plugin-private memory.
    pub async fn memory_search(
        &self,
        query: impl AsRef<str>,
        limit: i32,
    ) -> Result<PluginMemorySearchResponse, reqwest::Error> {
        #[derive(Serialize)]
        struct MemorySearchRequest<'a> {
            query: &'a str,
            #[serde(skip_serializing_if = "is_default")]
            limit: i32,
        }
        self.post_json(
            "/v1/plugin-api/memory/search",
            &MemorySearchRequest {
                query: query.as_ref(),
                limit,
            },
        )
        .await
    }

    /// Search shared agent memory.
    pub async fn agent_memory_search(
        &self,
        query: impl AsRef<str>,
        top_k: i32,
    ) -> Result<PluginAgentMemorySearchResponse, reqwest::Error> {
        #[derive(Serialize)]
        struct AgentMemorySearchRequest<'a> {
            query: &'a str,
            #[serde(skip_serializing_if = "is_default")]
            top_k: i32,
        }
        self.post_json(
            "/v1/plugin-api/agent-memory/search",
            &AgentMemorySearchRequest {
                query: query.as_ref(),
                top_k,
            },
        )
        .await
    }

    /// Add a fact to shared agent memory.
    pub async fn agent_memory_add(
        &self,
        fact: impl AsRef<str>,
        source: impl AsRef<str>,
    ) -> Result<PluginOkResponse, reqwest::Error> {
        #[derive(Serialize)]
        struct AgentMemoryAddRequest<'a> {
            fact: &'a str,
            #[serde(skip_serializing_if = "str::is_empty")]
            source: &'a str,
        }
        self.post_json(
            "/v1/plugin-api/agent-memory/add",
            &AgentMemoryAddRequest {
                fact: fact.as_ref(),
                source: source.as_ref(),
            },
        )
        .await
    }

    /// Search the agent knowledge base.
    pub async fn knowledge_search(
        &self,
        query: impl AsRef<str>,
        limit: i32,
    ) -> Result<PluginKnowledgeSearchResponse, reqwest::Error> {
        #[derive(Serialize)]
        struct KnowledgeSearchRequest<'a> {
            query: &'a str,
            #[serde(skip_serializing_if = "is_default")]
            limit: i32,
        }
        self.post_json(
            "/v1/plugin-api/knowledge/search",
            &KnowledgeSearchRequest {
                query: query.as_ref(),
                limit,
            },
        )
        .await
    }

    /// Ingest content into the agent knowledge base.
    pub async fn knowledge_ingest(
        &self,
        content: impl AsRef<str>,
        source: impl AsRef<str>,
        filename: impl AsRef<str>,
    ) -> Result<PluginOkResponse, reqwest::Error> {
        #[derive(Serialize)]
        struct KnowledgeIngestRequest<'a> {
            content: &'a str,
            #[serde(skip_serializing_if = "str::is_empty")]
            source: &'a str,
            #[serde(skip_serializing_if = "str::is_empty")]
            filename: &'a str,
        }
        self.post_json(
            "/v1/plugin-api/knowledge/ingest",
            &KnowledgeIngestRequest {
                content: content.as_ref(),
                source: source.as_ref(),
                filename: filename.as_ref(),
            },
        )
        .await
    }

    /// Create a plugin-owned scheduled task.
    pub async fn cron_add(
        &self,
        name: impl AsRef<str>,
        expression: impl AsRef<str>,
        message: impl AsRef<str>,
    ) -> Result<PluginCronAddResponse, reqwest::Error> {
        #[derive(Serialize)]
        struct CronAddRequest<'a> {
            name: &'a str,
            expression: &'a str,
            #[serde(skip_serializing_if = "str::is_empty")]
            message: &'a str,
        }
        self.post_json(
            "/v1/plugin-api/cron/add",
            &CronAddRequest {
                name: name.as_ref(),
                expression: expression.as_ref(),
                message: message.as_ref(),
            },
        )
        .await
    }

    /// Remove a plugin-owned scheduled task.
    pub async fn cron_remove(
        &self,
        id: impl AsRef<str>,
    ) -> Result<PluginOkResponse, reqwest::Error> {
        #[derive(Serialize)]
        struct CronRemoveRequest<'a> {
            id: &'a str,
        }
        self.post_json(
            "/v1/plugin-api/cron/remove",
            &CronRemoveRequest { id: id.as_ref() },
        )
        .await
    }

    /// List plugin-owned scheduled tasks.
    pub async fn cron_list(
        &self,
        plugin: impl AsRef<str>,
    ) -> Result<PluginCronListResponse, reqwest::Error> {
        let plugin = plugin.as_ref();
        let path = if plugin.is_empty() {
            "/v1/plugin-api/cron/list".to_string()
        } else {
            format!(
                "/v1/plugin-api/cron/list?plugin={}",
                url_encode_query_component(plugin)
            )
        };
        self.get_json(&path).await
    }

    /// Register a plugin-provided LLM provider.
    pub async fn register_provider(
        &self,
        config: &serde_json::Value,
    ) -> Result<PluginExtensionRegisterResponse, reqwest::Error> {
        self.post_json("/v1/plugin-api/register/provider", config)
            .await
    }

    /// Register a plugin-provided channel adapter.
    pub async fn register_channel(
        &self,
        config: &serde_json::Value,
    ) -> Result<PluginExtensionRegisterResponse, reqwest::Error> {
        self.post_json("/v1/plugin-api/register/channel", config)
            .await
    }

    /// Register a plugin-provided search engine.
    pub async fn register_search(
        &self,
        config: &serde_json::Value,
    ) -> Result<PluginExtensionRegisterResponse, reqwest::Error> {
        self.post_json("/v1/plugin-api/register/search", config)
            .await
    }

    /// Register a plugin-provided guardrail.
    pub async fn register_guardrail(
        &self,
        config: &serde_json::Value,
    ) -> Result<PluginExtensionRegisterResponse, reqwest::Error> {
        self.post_json("/v1/plugin-api/register/guardrail", config)
            .await
    }

    /// Register a plugin-provided embedding provider.
    pub async fn register_embedding(
        &self,
        config: &serde_json::Value,
    ) -> Result<PluginExtensionRegisterResponse, reqwest::Error> {
        self.post_json("/v1/plugin-api/register/embedding", config)
            .await
    }

    /// Register a plugin-provided speech provider.
    pub async fn register_speech(
        &self,
        config: &serde_json::Value,
    ) -> Result<PluginExtensionRegisterResponse, reqwest::Error> {
        self.post_json("/v1/plugin-api/register/speech", config)
            .await
    }

    /// List plugin-contributed system extensions.
    pub async fn extensions(&self) -> Result<PluginExtensionsResponse, reqwest::Error> {
        self.get_json("/v1/plugin-api/extensions").await
    }

    async fn post_json<B, T>(&self, path: &str, body: &B) -> Result<T, reqwest::Error>
    where
        B: Serialize + ?Sized,
        T: for<'de> Deserialize<'de>,
    {
        self.http
            .post(self.url(path))
            .json(body)
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    async fn get_json<T>(&self, path: &str) -> Result<T, reqwest::Error>
    where
        T: for<'de> Deserialize<'de>,
    {
        self.http
            .get(self.url(path))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }

    fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }
}

fn trigger_list_query(options: &TriggerListOptions) -> String {
    let mut params = Vec::new();
    if !options.tenant_id.is_empty() {
        params.push(format!("tenant_id={}", options.tenant_id));
    }
    if !options.r#type.is_empty() {
        params.push(format!("type={}", options.r#type));
    }
    if !options.status.is_empty() {
        params.push(format!("status={}", options.status));
    }
    if params.is_empty() {
        String::new()
    } else {
        format!("?{}", params.join("&"))
    }
}

fn trigger_history_query(options: &TriggerHistoryOptions) -> String {
    let mut params = Vec::new();
    if !options.trigger_id.is_empty() {
        params.push(format!("trigger_id={}", options.trigger_id));
    }
    if options.limit > 0 {
        params.push(format!("limit={}", options.limit));
    }
    if params.is_empty() {
        String::new()
    } else {
        format!("?{}", params.join("&"))
    }
}

fn knowledge_search_query(options: &KnowledgeSearchOptions) -> String {
    let mut params = vec![format!("q={}", url_encode_query_component(&options.query))];
    if options.limit > 0 {
        params.push(format!("n={}", options.limit));
    }
    if !options.file.is_empty() {
        params.push(format!(
            "file={}",
            url_encode_query_component(&options.file)
        ));
    }
    if !options.lang.is_empty() {
        params.push(format!(
            "lang={}",
            url_encode_query_component(&options.lang)
        ));
    }
    format!("/v1/knowledge/search?{}", params.join("&"))
}

fn trim_base_url(base_url: String) -> String {
    base_url.trim_end_matches('/').to_string()
}

fn reflect_query(options: &ReflectOptions, stats: bool) -> String {
    let mut query: Vec<(&str, String)> = Vec::new();
    if !options.q.is_empty() {
        query.push(("q", options.q.clone()));
    }
    if !options.source.is_empty() {
        query.push(("source", options.source.clone()));
    }
    if !options.category.is_empty() {
        query.push(("category", options.category.clone()));
    }
    if !options.outcome.is_empty() {
        query.push(("outcome", options.outcome.clone()));
    }
    if !options.tag.is_empty() {
        query.push(("tag", options.tag.clone()));
    }
    if options.limit > 0 {
        query.push(("limit", options.limit.to_string()));
    }
    if stats {
        query.push(("stats", "true".to_string()));
    }
    if query.is_empty() {
        return String::new();
    }
    let encoded = query
        .into_iter()
        .map(|(key, value)| format!("{key}={}", url_encode_query_component(&value)))
        .collect::<Vec<_>>()
        .join("&");
    format!("?{encoded}")
}

fn url_encode_query_component(value: &str) -> String {
    let mut encoded = String::new();
    for byte in value.bytes() {
        match byte {
            b'A'..=b'Z' | b'a'..=b'z' | b'0'..=b'9' | b'-' | b'_' | b'.' | b'~' => {
                encoded.push(byte as char)
            }
            b' ' => encoded.push('+'),
            _ => encoded.push_str(&format!("%{byte:02X}")),
        }
    }
    encoded
}

fn is_default<T>(value: &T) -> bool
where
    T: Default + PartialEq,
{
    value == &T::default()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn state_snapshot_deserializes_missing_sections_to_empty() {
        let snapshot: StateSnapshot =
            serde_json::from_str(r#"{"goals":[],"resources":[]}"#).unwrap();
        assert!(snapshot.recent_actions.is_empty());
        assert_eq!(snapshot.capabilities, StateCapabilities::default());
    }

    #[test]
    fn state_goal_serializes_incremental_body() {
        let goal = StateGoal {
            title: "Ship Rust state helper".to_string(),
            priority: 2,
            ..StateGoal::default()
        };
        let value = serde_json::to_value(goal).unwrap();
        assert_eq!(value["title"], "Ship Rust state helper");
        assert_eq!(value["priority"], 2);
        assert!(value.get("id").is_none());
    }

    #[test]
    fn state_client_trims_base_url() {
        let client = StateClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(client.url("/v1/state"), "http://localhost:9090/v1/state");
    }

    #[test]
    fn reflect_query_serializes_filters() {
        let query = reflect_query(
            &ReflectOptions {
                q: "code review".to_string(),
                source: "task".to_string(),
                outcome: "partial".to_string(),
                tag: "quality:9".to_string(),
                limit: 5,
                ..ReflectOptions::default()
            },
            false,
        );
        assert_eq!(
            query,
            "?q=code+review&source=task&outcome=partial&tag=quality%3A9&limit=5"
        );
    }

    #[test]
    fn reflect_stats_query_appends_stats_flag() {
        let query = reflect_query(
            &ReflectOptions {
                tag: "quality:9".to_string(),
                ..ReflectOptions::default()
            },
            true,
        );
        assert_eq!(query, "?tag=quality%3A9&stats=true");
    }

    #[test]
    fn reflect_client_trims_base_url() {
        let client =
            ReflectClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(
            client.url("/v1/reflect/strategies"),
            "http://localhost:9090/v1/reflect/strategies"
        );
    }

    #[test]
    fn plugin_llm_request_serializes_incremental_body() {
        let request = PluginLLMRequest {
            messages: vec![PluginLLMMessage {
                role: "user".to_string(),
                content: "hello".to_string(),
            }],
            model: "test-model".to_string(),
            ..PluginLLMRequest::default()
        };
        let value = serde_json::to_value(request).unwrap();
        assert_eq!(value["messages"][0]["role"], "user");
        assert_eq!(value["model"], "test-model");
        assert!(value.get("temperature").is_none());
    }

    #[test]
    fn plugin_api_client_trims_base_url() {
        let client =
            PluginApiClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(
            client.url("/v1/plugin-api/llm"),
            "http://localhost:9090/v1/plugin-api/llm"
        );
    }

    #[test]
    fn browser_helpers_build_urls_and_payloads() {
        let client =
            BrowserClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(
            client.url("/v1/browser/status"),
            "http://localhost:9090/v1/browser/status"
        );
        let mut action = BrowserAction::new();
        action.insert(
            "type".to_string(),
            serde_json::Value::String("browser_screenshot".to_string()),
        );
        assert_eq!(action["type"], "browser_screenshot");
        let payload = serde_json::json!({ "scenario_id": "open-page" });
        assert_eq!(payload["scenario_id"], "open-page");
    }

    #[test]
    fn files_helpers_build_urls_and_metadata() {
        let client = FilesClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(client.url("/api/files"), "http://localhost:9090/api/files");
        let entry = FileEntry {
            name: "report.md".to_string(),
            path: "artifacts/report.md".to_string(),
            size: 12,
            is_dir: false,
        };
        let value = serde_json::to_value(entry).unwrap();
        assert_eq!(value["name"], "report.md");
        assert_eq!(value["is_dir"], false);
        assert_eq!(
            filename_from_disposition("attachment; filename=\"report.md\""),
            "report.md"
        );
    }

    #[test]
    fn rbac_helpers_build_urls_and_requests() {
        let client = RBACClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(
            client.url("/v1/rbac/roles"),
            "http://localhost:9090/v1/rbac/roles"
        );
        let binding = RBACRoleBindingRequest {
            subject_id: "u1".to_string(),
            role_id: "operator".to_string(),
            tenant_id: String::new(),
        };
        let value = serde_json::to_value(binding).unwrap();
        assert_eq!(value["subject_id"], "u1");
        assert!(value.get("tenant_id").is_none());
        let check = RBACCheckRequest {
            subject_id: "u1".to_string(),
            resource: "tasks".to_string(),
            action: "write".to_string(),
            ..Default::default()
        };
        let check_value = serde_json::to_value(check).unwrap();
        assert_eq!(check_value["resource"], "tasks");
        assert_eq!(check_value["action"], "write");
        let mut role = RBACRole::new();
        role.insert(
            "id".to_string(),
            serde_json::Value::String("operator".to_string()),
        );
        assert_eq!(role["id"], "operator");
    }

    #[test]
    fn approvals_helpers_build_urls_and_requests() {
        let client =
            ApprovalsClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(
            client.url("/v1/approvals"),
            "http://localhost:9090/v1/approvals"
        );
        let opts = ListApprovalsOptions {
            status: "approved".to_string(),
            history: true,
        };
        let value = serde_json::to_value(opts).unwrap();
        assert_eq!(value["status"], "approved");
        assert_eq!(value["history"], true);
        let empty = serde_json::to_value(ListApprovalsOptions::default()).unwrap();
        assert!(empty.get("status").is_none());
        assert!(empty.get("history").is_none());
        let mut rule = ApprovalRule::new();
        rule.insert(
            "id".to_string(),
            serde_json::Value::String("r1".to_string()),
        );
        rule.insert(
            "decision".to_string(),
            serde_json::Value::String("allow_always".to_string()),
        );
        assert_eq!(rule["decision"], "allow_always");
    }

    #[test]
    fn conversations_helpers_build_urls_and_requests() {
        let client =
            ConversationsClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(
            client.url("/v1/conversations"),
            "http://localhost:9090/v1/conversations"
        );
        let manage = ManageConversationRequest {
            session_id: "s1".to_string(),
            name: "新的会话".to_string(),
            ..Default::default()
        };
        let value = serde_json::to_value(manage).unwrap();
        assert_eq!(value["session_id"], "s1");
        assert_eq!(value["name"], "新的会话");
        assert!(value.get("pinned").is_none());
        let opts = ConversationReplayOptions {
            raw: true,
            limit: 10,
            offset: 2,
        };
        let opts_value = serde_json::to_value(opts).unwrap();
        assert_eq!(opts_value["raw"], true);
        assert_eq!(opts_value["limit"], 10);
    }

    #[test]
    fn chat_helpers_build_requests_and_parse_stream() {
        let client = ChatClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(client.url("/v1/chat"), "http://localhost:9090/v1/chat");
        assert_eq!(client.stream_url(), "http://localhost:9090/v1/chat/stream");
        let request = ChatRequest {
            messages: vec![ChatMessage {
                role: "user".to_string(),
                content: "hi".to_string(),
                name: String::new(),
            }],
            session_id: "s1".to_string(),
            ..Default::default()
        };
        let stream_request = client.stream_request(request.clone());
        assert!(stream_request.stream);
        let value = serde_json::to_value(request).unwrap();
        assert_eq!(value["messages"][0]["role"], "user");
        assert_eq!(value["session_id"], "s1");
        let items = client.parse_stream("event: message\ndata: {\"type\":\"delta\",\"content\":\"你\"}\n\nevent: error\ndata: {\"error\":\"bad\"}\n\n");
        assert_eq!(items.len(), 2);
        assert_eq!(items[0].kind, "delta");
        assert_eq!(items[0].content, "你");
        assert_eq!(items[1].kind, "error");
    }

    #[test]
    fn realtime_helpers_build_urls_and_messages() {
        let client = RealtimeClient::new("http://localhost:9090/", "token-1").unwrap();
        assert_eq!(
            client.ws_url_with_query(&[("tenant", "t1")]),
            "ws://localhost:9090/v1/ws?tenant=t1&access_token=token-1"
        );
        let chat = client.chat("你好", "s1", serde_json::Map::new());
        let encoded = client.serialize(&chat).unwrap();
        let parsed = client.parse(&encoded).unwrap();
        assert_eq!(parsed.r#type, "chat");
        assert_eq!(parsed.content, "你好");
        assert_eq!(parsed.session, "s1");
        let ping = client.ping(serde_json::Map::new());
        assert_eq!(ping.r#type, "ping");
        assert!(client.parse("[]").is_err());
    }

    #[test]
    fn agent_kit_groups_lightweight_clients() {
        let kit = AgentKit::new_with_clients(
            "http://localhost:9090/",
            reqwest::Client::new(),
            reqwest::Client::new(),
            reqwest::Client::new(),
        );
        assert_eq!(kit.state.url("/v1/state"), "http://localhost:9090/v1/state");
        assert_eq!(
            kit.reflect.url("/v1/reflect/strategies"),
            "http://localhost:9090/v1/reflect/strategies"
        );
        assert_eq!(
            kit.missions.url("/v1/missions/parse"),
            "http://localhost:9090/v1/missions/parse"
        );
        assert_eq!(
            kit.scheduler.url("/v1/scheduler/jobs"),
            "http://localhost:9090/v1/scheduler/jobs"
        );
        assert_eq!(
            kit.cron.url("/v1/cron/list"),
            "http://localhost:9090/v1/cron/list"
        );
        assert_eq!(
            kit.triggers.url("/v1/triggers/v2"),
            "http://localhost:9090/v1/triggers/v2"
        );
        assert_eq!(
            kit.memory.url("/v1/memory/search"),
            "http://localhost:9090/v1/memory/search"
        );
        assert_eq!(
            kit.graph.url("/v1/graph/stats"),
            "http://localhost:9090/v1/graph/stats"
        );
        assert_eq!(
            kit.knowledge.url("/v1/knowledge/stats"),
            "http://localhost:9090/v1/knowledge/stats"
        );
        assert_eq!(
            kit.lora.url("/v1/lora/status"),
            "http://localhost:9090/v1/lora/status"
        );
        assert_eq!(
            kit.workflows.url("/v1/workflows"),
            "http://localhost:9090/v1/workflows"
        );
        assert_eq!(
            kit.connectors.url("/api/connectors"),
            "http://localhost:9090/api/connectors"
        );
        assert_eq!(
            kit.notify.url("/api/notify/channels"),
            "http://localhost:9090/api/notify/channels"
        );
        assert_eq!(
            kit.projects.url("/v1/projects"),
            "http://localhost:9090/v1/projects"
        );
        assert_eq!(
            kit.market.url("/v1/market/stats"),
            "http://localhost:9090/v1/market/stats"
        );
        assert_eq!(
            kit.dispatch.url("/v1/workers"),
            "http://localhost:9090/v1/workers"
        );
        assert_eq!(
            kit.cost.url("/v1/cost/summary"),
            "http://localhost:9090/v1/cost/summary"
        );
        assert_eq!(
            kit.providers.url("/api/providers"),
            "http://localhost:9090/api/providers"
        );
        assert_eq!(
            kit.cognis.url("/v1/cognis"),
            "http://localhost:9090/v1/cognis"
        );
        assert_eq!(
            kit.trace.url("/v1/trace/recent"),
            "http://localhost:9090/v1/trace/recent"
        );
        assert_eq!(
            kit.heartbeat.url("/v1/heartbeat"),
            "http://localhost:9090/v1/heartbeat"
        );
        assert_eq!(
            kit.runtime.url("/v1/sessions/queue"),
            "http://localhost:9090/v1/sessions/queue"
        );
        assert_eq!(
            kit.subagents.url("/v1/subagent"),
            "http://localhost:9090/v1/subagent"
        );
        assert_eq!(
            kit.tools.url("/v1/tools/list"),
            "http://localhost:9090/v1/tools/list"
        );
        assert_eq!(
            kit.audit.url("/v1/audit/tail"),
            "http://localhost:9090/v1/audit/tail"
        );
        assert_eq!(
            kit.trust.url("/api/trust/scores"),
            "http://localhost:9090/api/trust/scores"
        );
        assert_eq!(
            kit.iterate.url("/api/iterate/status"),
            "http://localhost:9090/api/iterate/status"
        );
        assert_eq!(
            kit.persona.url("/v1/persona"),
            "http://localhost:9090/v1/persona"
        );
        assert_eq!(
            kit.emotion.url("/v1/emotion/history"),
            "http://localhost:9090/v1/emotion/history"
        );
        assert_eq!(
            kit.instructions.url("/v1/instructions"),
            "http://localhost:9090/v1/instructions"
        );
        assert_eq!(
            kit.reactions.url("/v1/react"),
            "http://localhost:9090/v1/react"
        );
        assert_eq!(
            kit.permissions.url("/v1/rbac/check"),
            "http://localhost:9090/v1/rbac/check"
        );
        assert_eq!(
            kit.plugin.url("/v1/plugin-api/search"),
            "http://localhost:9090/v1/plugin-api/search"
        );
    }

    #[test]
    fn agent_kit_accepts_shared_or_separate_tokens() {
        assert!(AgentKit::new("http://localhost:9090", "shared-token").is_ok());
        assert!(AgentKit::new_with_plugin_token(
            "http://localhost:9090",
            "api-token",
            "plugin-token"
        )
        .is_ok());
    }

    #[test]
    fn mission_parse_result_deserializes_incremental_body() {
        let result: MissionParseResult = serde_json::from_str(
            r#"{"type":"cron","name":"每日总结","description":"每天总结昨天的任务","config":{"cron_expr":"0 8 * * *"},"confidence":0.9,"explanation":"mentions daily schedule"}"#,
        )
        .unwrap();
        assert_eq!(result.r#type, "cron");
        assert_eq!(result.config["cron_expr"], "0 8 * * *");
    }

    #[test]
    fn missions_client_trims_base_url() {
        let client =
            MissionsClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(
            client.url("/v1/missions/parse"),
            "http://localhost:9090/v1/missions/parse"
        );
    }

    #[test]
    fn scheduler_job_deserializes_incremental_body() {
        let jobs: SchedulerJobsResponse = serde_json::from_str(
            r#"{"jobs":[{"id":"job_1","name":"daily","tenant_id":"default","interval":60000000000,"prompt":"复盘"}],"count":1}"#,
        )
        .unwrap();
        assert_eq!(jobs.count, 1);
        assert_eq!(jobs.jobs[0].id, "job_1");
        assert_eq!(jobs.jobs[0].interval, serde_json::json!(60000000000_i64));

        let removed: SchedulerRemoveResponse =
            serde_json::from_str(r#"{"status":"removed"}"#).unwrap();
        assert_eq!(removed.status, "removed");
    }

    #[test]
    fn scheduler_client_trims_base_url() {
        let client =
            SchedulerClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(
            client.url("/v1/scheduler/jobs"),
            "http://localhost:9090/v1/scheduler/jobs"
        );
    }

    #[test]
    fn memory_types_serialize_incremental_body() {
        let add = MemoryAddRequest {
            content: "喜欢中文回复".to_string(),
            layer: "long".to_string(),
            source: "sdk-test".to_string(),
            ..MemoryAddRequest::default()
        };
        let value = serde_json::to_value(add).unwrap();
        assert_eq!(value["content"], "喜欢中文回复");
        assert_eq!(value["layer"], "long");
        assert!(value.get("key").is_none());

        let search: MemorySearchResponse = serde_json::from_str(
            r#"{"results":[{"key":"pref","value":"喜欢短回答","layer":"mid","score":0.9}],"count":1}"#,
        )
        .unwrap();
        assert_eq!(search.count, 1);
        assert_eq!(search.results[0].key, "pref");
    }

    #[test]
    fn memory_client_trims_base_url() {
        let client =
            MemoryClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(
            client.url("/v1/memory/stats"),
            "http://localhost:9090/v1/memory/stats"
        );
    }

    #[test]
    fn graph_types_deserialize_incremental_bodies() {
        let entities: GraphEntitiesResponse =
            serde_json::from_str(r#"{"entities":[{"id":"e1","name":"云雀","type":"agent"}]}"#)
                .unwrap();
        assert_eq!(entities.entities[0].name, "云雀");
        assert_eq!(entities.entities[0].r#type, "agent");

        let relation = GraphRelation {
            from_id: "e1".to_string(),
            to_id: "e2".to_string(),
            r#type: "uses".to_string(),
            ..GraphRelation::default()
        };
        let value = serde_json::to_value(relation).unwrap();
        assert_eq!(value["from_id"], "e1");
        assert_eq!(value["type"], "uses");

        let stats: GraphStatsResponse =
            serde_json::from_str(r#"{"entities":2,"relations":1}"#).unwrap();
        assert_eq!(stats.entities, 2);
    }

    #[test]
    fn graph_client_trims_base_url() {
        let client = GraphClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(
            client.url("/v1/graph/entities"),
            "http://localhost:9090/v1/graph/entities"
        );
    }

    #[test]
    fn knowledge_types_deserialize_incremental_bodies() {
        let search: KnowledgeSearchResponse = serde_json::from_str(
            r#"{"chunks":[{"id":"c1","source_id":"src_1","content":"SDK slice","score":0.9}],"count":1}"#,
        )
        .unwrap();
        assert_eq!(search.count, 1);
        assert_eq!(search.chunks[0].content, "SDK slice");

        let sources: KnowledgeSourcesResponse = serde_json::from_str(
            r#"{"sources":[{"id":"src_1","name":"README.md","type":"file"}]}"#,
        )
        .unwrap();
        assert_eq!(sources.sources[0].id, "src_1");

        let mutation: KnowledgeMutationResponse = serde_json::from_str(
            r#"{"source":{"id":"src_2","name":"inline"},"stats":{"sources":2}}"#,
        )
        .unwrap();
        assert_eq!(mutation.source.unwrap().name, "inline");
        assert_eq!(mutation.stats["sources"], serde_json::json!(2));
    }

    #[test]
    fn knowledge_search_query_encodes_filters() {
        let query = knowledge_search_query(&KnowledgeSearchOptions {
            query: "增量 SDK".to_string(),
            limit: 3,
            file: "README.md".to_string(),
            lang: "zh cn".to_string(),
        });
        assert_eq!(
            query,
            "/v1/knowledge/search?q=%E5%A2%9E%E9%87%8F+SDK&n=3&file=README.md&lang=zh+cn"
        );
    }

    #[test]
    fn knowledge_client_trims_base_url() {
        let client =
            KnowledgeClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(
            client.url("/v1/knowledge/sources"),
            "http://localhost:9090/v1/knowledge/sources"
        );
    }

    #[test]
    fn workflow_types_deserialize_incremental_bodies() {
        let list: WorkflowListResponse = serde_json::from_str(
            r#"{"workflows":[{"id":"wf_1","name":"SDK flow","nodes":[{"id":"start"}]}],"total":1}"#,
        )
        .unwrap();
        assert_eq!(list.total, 1);
        assert_eq!(list.workflows[0].name, "SDK flow");

        let request = WorkflowRunRequest {
            definition_id: "wf_1".to_string(),
            variables: serde_json::Map::new(),
        };
        let value = serde_json::to_value(request).unwrap();
        assert_eq!(value["definition_id"], "wf_1");

        let client =
            WorkflowClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(
            client.url("/v1/workflows"),
            "http://localhost:9090/v1/workflows"
        );
    }

    #[test]
    fn connector_types_deserialize_incremental_bodies() {
        let list: ConnectorListResponse = serde_json::from_str(
            r#"{"connectors":[{"id":"github","name":"GitHub","supported":true,"status":"connected","action_count":2}]}"#,
        )
        .unwrap();
        assert_eq!(list.connectors[0].id, "github");
        assert_eq!(list.connectors[0].action_count, 2);

        let detail: ConnectorDetailResponse = serde_json::from_str(
            r#"{"connector":{"id":"github","name":"GitHub","actions":[{"id":"create_issue"}]},"supported":true,"status":"connected"}"#,
        )
        .unwrap();
        assert_eq!(detail.connector.actions[0].id, "create_issue");

        let request = ConnectorExecuteRequest {
            connector_id: "github".to_string(),
            action_id: "create_issue".to_string(),
            params: serde_json::Map::new(),
        };
        let value = serde_json::to_value(request).unwrap();
        assert_eq!(value["connector_id"], "github");

        let client =
            ConnectorsClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(
            client.url("/api/connectors"),
            "http://localhost:9090/api/connectors"
        );
    }

    #[test]
    fn dispatch_types_deserialize_incremental_bodies() {
        let workers: DispatchWorkersResponse = serde_json::from_str(
            r#"{"workers":[{"id":"w1","type":"cursor","capabilities":["coding"]}],"count":1}"#,
        )
        .unwrap();
        assert_eq!(workers.count, 1);
        assert_eq!(workers.workers[0].r#type, "cursor");

        let request = DispatchEnqueueRequest {
            task_id: "t1".to_string(),
            capabilities: vec!["coding".to_string()],
            priority: 10,
        };
        let value = serde_json::to_value(request).unwrap();
        assert_eq!(value["task_id"], "t1");

        let client =
            DispatchClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(
            client.url("/v1/workers"),
            "http://localhost:9090/v1/workers"
        );
    }

    #[test]
    fn providers_types_deserialize_incremental_bodies() {
        let models: ModelsResponse =
            serde_json::from_str(r#"{"models":[{"id":"m1","model_id":"deepseek-chat"}]}"#).unwrap();
        assert_eq!(models.models[0]["id"], "m1");

        let providers: ProvidersResponse = serde_json::from_str(
            r#"{"providers":[{"id":"deepseek","model":"deepseek-chat"}],"mode":"hybrid"}"#,
        )
        .unwrap();
        assert_eq!(providers.providers[0]["id"], "deepseek");
        assert_eq!(providers.mode, "hybrid");

        let session = ProviderSessionOverrideRequest {
            session_id: "s1".to_string(),
            provider_id: "deepseek".to_string(),
        };
        let value = serde_json::to_value(session).unwrap();
        assert_eq!(value["provider_id"], "deepseek");

        let client =
            ProvidersClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(
            client.url("/api/providers"),
            "http://localhost:9090/api/providers"
        );
    }

    #[test]
    fn cognis_client_trims_base_url() {
        let client =
            CognisClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(client.url("/v1/cognis"), "http://localhost:9090/v1/cognis");
    }

    #[test]
    fn trace_types_deserialize_incremental_bodies() {
        let recent: TraceEventsResponse =
            serde_json::from_str(r#"{"events":[{"trace_id":"tr-1"}],"count":1}"#).unwrap();
        assert_eq!(recent.count, 1);
        assert_eq!(recent.events[0]["trace_id"], "tr-1");

        let by_trace: TraceByIdResponse =
            serde_json::from_str(r#"{"trace_id":"tr/1","events":[],"count":0,"raw":true}"#)
                .unwrap();
        assert_eq!(by_trace.trace_id, "tr/1");
        assert!(by_trace.raw);

        let client = TraceClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(
            client.url("/v1/trace/recent"),
            "http://localhost:9090/v1/trace/recent"
        );
    }

    #[test]
    fn heartbeat_types_serialize_incremental_body() {
        let request = HeartbeatUpdateRequest {
            enabled: Some(true),
            interval_minutes: Some(30),
        };
        let value = serde_json::to_value(request).unwrap();
        assert_eq!(value["enabled"], true);
        assert_eq!(value["interval_minutes"], 30);

        let client =
            HeartbeatClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(
            client.url("/v1/heartbeat"),
            "http://localhost:9090/v1/heartbeat"
        );
    }

    #[test]
    fn tasks_helpers_build_urls_and_payloads() {
        let client = TasksClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(client.url("/v1/tasks"), "http://localhost:9090/v1/tasks");
        assert_eq!(
            client.url(&format!("/v1/tasks?id={}", url_encode_query_component("task 1"))),
            "http://localhost:9090/v1/tasks?id=task+1"
        );
        let request = serde_json::to_value(CreateTaskRequest {
            title: "SDK".to_string(),
            description: "ship lightweight tasks SDK".to_string(),
            constraints: TaskConstraints { max_steps: 3, risk_level: "low".to_string(), ..Default::default() },
        }).unwrap();
        assert_eq!(request["description"], "ship lightweight tasks SDK");
        assert_eq!(request["constraints"]["max_steps"], 3);
        assert_eq!(request["constraints"]["risk_level"], "low");
        let listed: Vec<Task> = serde_json::from_str(r#"[{"id":"task-1","status":"running"}]"#).unwrap();
        assert_eq!(listed[0]["id"], "task-1");
        assert_eq!(
            client.url(&format!("/v1/tasks/templates?id={}", url_encode_query_component("tpl 1"))),
            "http://localhost:9090/v1/tasks/templates?id=tpl+1"
        );
        let template = serde_json::to_value(CreateTaskTemplateRequest {
            id: "tpl-1".to_string(),
            name: "Review".to_string(),
            steps: vec![TaskTemplateStep { action: "review".to_string(), skill_name: "code".to_string(), ..Default::default() }],
            variables: vec![TaskTemplateVariable { name: "repo".to_string(), required: true, ..Default::default() }],
            ..Default::default()
        }).unwrap();
        assert_eq!(template["id"], "tpl-1");
        assert_eq!(template["steps"][0]["action"], "review");
        assert_eq!(template["variables"][0]["required"], true);
        let instantiate = serde_json::to_value(InstantiateTaskTemplateRequest {
            template_id: "tpl-1".to_string(),
            variables: std::collections::BTreeMap::from([("repo".to_string(), "yunque".to_string())]),
        }).unwrap();
        assert_eq!(instantiate["template_id"], "tpl-1");
        assert_eq!(instantiate["variables"]["repo"], "yunque");
        assert_eq!(
            client.url(&format!("/v1/tasks/gaps?type={}", url_encode_query_component("skill missing"))),
            "http://localhost:9090/v1/tasks/gaps?type=skill+missing"
        );
        let gaps: Vec<TaskGap> = serde_json::from_str(r#"[{"id":"gap-1","gap_type":"skill_missing"}]"#).unwrap();
        assert_eq!(gaps[0]["gap_type"], "skill_missing");
    }
    #[test]
    fn permissions_helpers_build_urls_and_payloads() {
        let client =
            PermissionsClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(
            client.url("/v1/rbac/check"),
            "http://localhost:9090/v1/rbac/check"
        );
        assert_eq!(
            client.url("/v1/rbac/my-roles"),
            "http://localhost:9090/v1/rbac/my-roles"
        );
        let check = serde_json::to_value(RBACCheckRequest {
            subject_id: "u1".to_string(),
            resource: "knowledge".to_string(),
            action: "read".to_string(),
            ..Default::default()
        })
        .unwrap();
        assert_eq!(check["resource"], "knowledge");
        assert_eq!(check["action"], "read");
    }

    #[test]
    fn reactions_helpers_build_urls_and_payloads() {
        let client =
            ReactionsClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(client.url("/v1/react"), "http://localhost:9090/v1/react");
        assert_eq!(
            client.url("/v1/sticker/send"),
            "http://localhost:9090/v1/sticker/send"
        );
        let react = serde_json::to_value(ReactRequest {
            channel_type: "wechat".to_string(),
            target: "u1".to_string(),
            message_id: "m1".to_string(),
            emoji: "👍".to_string(),
        })
        .unwrap();
        assert_eq!(react["message_id"], "m1");
        let sticker = serde_json::to_value(SendStickerRequest {
            channel_type: "wechat".to_string(),
            target: "u1".to_string(),
            emoji: "🌟".to_string(),
            ..Default::default()
        })
        .unwrap();
        assert_eq!(sticker["emoji"], "🌟");
        assert!(sticker.get("package_id").is_none());
    }

    #[test]
    fn instructions_helpers_build_urls_queries_and_payloads() {
        let client =
            InstructionsClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(
            client.url("/v1/instructions"),
            "http://localhost:9090/v1/instructions"
        );
        assert_eq!(
            instructions_list_query("style guide"),
            "?category=style+guide"
        );
        assert_eq!(instructions_list_query(""), "");
        let mut instruction = serde_json::Map::new();
        instruction.insert("category".to_string(), serde_json::json!("style"));
        instruction.insert("content".to_string(), serde_json::json!("保持简洁"));
        assert_eq!(instruction["content"], "保持简洁");
        let ids = vec!["ins-2".to_string(), "ins-1".to_string()];
        let body = serde_json::json!({ "ids": ids });
        assert_eq!(body["ids"][0], "ins-2");
    }

    #[test]
    fn emotion_helpers_build_urls_queries_and_payloads() {
        let client =
            EmotionClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(
            client.url("/v1/emotion/stickers"),
            "http://localhost:9090/v1/emotion/stickers"
        );
        assert_eq!(
            emotion_history_query(&EmotionHistoryQuery {
                session_id: "s1".to_string(),
                limit: 5,
                from: "".to_string(),
                to: "".to_string(),
            }),
            "?session_id=s1&limit=5"
        );
        let register = serde_json::to_value(RegisterStickersRequest {
            platform: "wechat".to_string(),
            emotion: "happy".to_string(),
            stickers: vec![StickerSuggestion {
                package_id: "p1".to_string(),
                sticker_id: "s1".to_string(),
            }],
        })
        .unwrap();
        assert_eq!(register["stickers"][0]["sticker_id"], "s1");
        let clear = serde_json::to_value(ClearStickersRequest {
            platform: "wechat".to_string(),
            emotion: "happy".to_string(),
        })
        .unwrap();
        assert_eq!(clear["emotion"], "happy");
    }

    #[test]
    fn persona_helpers_build_urls_and_payloads() {
        let client =
            PersonaClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(
            client.url("/v1/persona"),
            "http://localhost:9090/v1/persona"
        );
        let update = serde_json::to_value(UpdatePersonaRequest {
            identity: "Tori".to_string(),
            soul: "careful".to_string(),
        })
        .unwrap();
        assert_eq!(update["identity"], "Tori");
        let skill = serde_json::to_value(AddPersonaSkillRequest {
            name: "review".to_string(),
            description: "Review".to_string(),
            content: "review code".to_string(),
        })
        .unwrap();
        assert_eq!(skill["name"], "review");
        let state: serde_json::Value = serde_json::from_str(
            r#"{"identity":"Tori","soul":"careful","skills":[{"name":"review"}]}"#,
        )
        .unwrap();
        assert_eq!(state["skills"][0]["name"], "review");
        let mut features = std::collections::BTreeMap::new();
        features.insert("emotion".to_string(), true);
        let preset = serde_json::to_value(UpdatePersonaPresetFeaturesRequest {
            id: "studio".to_string(),
            features,
        })
        .unwrap();
        assert_eq!(preset["features"]["emotion"], true);
    }

    #[test]
    fn iterate_helpers_build_urls_and_payloads() {
        let client =
            IterateClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(
            client.url("/api/iterate/proposals"),
            "http://localhost:9090/api/iterate/proposals"
        );
        assert_eq!(
            iterate_proposals_query(&IterateProposalsQuery {
                status: "pending".to_string(),
            }),
            "?status=pending"
        );
        let body = serde_json::to_value(IterateDecisionRequest {
            id: "it-1".to_string(),
        })
        .unwrap();
        assert_eq!(body["id"], "it-1");
        let proposals: serde_json::Value =
            serde_json::from_str(r#"{"proposals":[{"id":"it-1","status":"pending"}],"count":1}"#)
                .unwrap();
        assert_eq!(proposals["proposals"][0]["id"], "it-1");
    }

    #[test]
    fn trust_helpers_build_urls_and_payloads() {
        let client = TrustClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(
            client.url("/api/trust/scores"),
            "http://localhost:9090/api/trust/scores"
        );
        assert_eq!(
            client.url("/api/review/status"),
            "http://localhost:9090/api/review/status"
        );
        let body = serde_json::to_value(TrustSlugRequest {
            slug: "shell".to_string(),
        })
        .unwrap();
        assert_eq!(body["slug"], "shell");
        let scores: serde_json::Value =
            serde_json::from_str(r#"{"scores":{"shell":{"score":80,"level":"review"}},"count":1}"#)
                .unwrap();
        assert_eq!(scores["scores"]["shell"]["score"], 80);
    }

    #[test]
    fn audit_helpers_build_urls_and_types() {
        let client = AuditClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(
            client.url("/v1/audit/tail"),
            "http://localhost:9090/v1/audit/tail"
        );
        assert_eq!(
            audit_tail_query(&AuditTailQuery {
                n: 10,
                r#type: "system event".to_string(),
                actor: "tenant".to_string(),
            }),
            "?n=10&type=system+event&actor=tenant"
        );
        assert_eq!(
            audit_trail_query(&AuditTrailQuery {
                date: "2026-05-11".to_string(),
                r#type: "nl_config".to_string(),
            }),
            "?date=2026-05-11&type=nl_config"
        );
        let tail: AuditTailResponse =
            serde_json::from_str(r#"{"records":[{"id":"r1","type":"system"}],"count":1}"#).unwrap();
        assert_eq!(tail.count, 1);
        assert_eq!(tail.records[0]["id"], "r1");
    }

    #[test]
    fn tools_helpers_build_urls_and_payloads() {
        let client = ToolsClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(
            client.url("/v1/tools/list"),
            "http://localhost:9090/v1/tools/list"
        );
        assert_eq!(
            client.url(&format!(
                "/v1/tools/poll?id={}",
                url_encode_query_component("session 1")
            )),
            "http://localhost:9090/v1/tools/poll?id=session+1"
        );
        let opts = ToolExecOptions {
            command: "echo ok".to_string(),
            cwd: "work".to_string(),
            timeout_ms: 1000,
            env: vec!["A=B".to_string()],
            ..ToolExecOptions::default()
        };
        let body = serde_json::to_value(&opts).unwrap();
        assert_eq!(body["Command"], "echo ok");
        assert_eq!(body["Cwd"], "work");
        assert_eq!(body["TimeoutMs"], 1000);
        let list: ToolListResponse = serde_json::from_str(
            r#"{"sessions":[{"id":"s1","command":"npm test","state":"running"}]}"#,
        )
        .unwrap();
        assert_eq!(list.sessions[0].id, "s1");
    }

    #[test]
    fn subagents_helpers_build_urls_and_types() {
        let client =
            SubagentsClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(
            client.url("/v1/subagent"),
            "http://localhost:9090/v1/subagent"
        );
        assert_eq!(
            client.url(&format!(
                "/v1/subagent?parent_id={}",
                url_encode_query_component("task 1")
            )),
            "http://localhost:9090/v1/subagent?parent_id=task+1"
        );
        let listed: SubagentsResponse = serde_json::from_str(
            r#"{"subagents":[{"id":"sa-1","name":"reviewer","skills":["review"]}]}"#,
        )
        .unwrap();
        assert_eq!(listed.subagents[0].id, "sa-1");
        let spawned = SpawnSubagentRequest {
            parent_id: "task-1".to_string(),
            name: "planner".to_string(),
            description: "计划拆解".to_string(),
            skills: vec!["plan".to_string()],
        };
        assert_eq!(spawned.skills[0], "plan");
    }

    #[test]
    fn runtime_helpers_build_urls_and_payloads() {
        let client =
            RuntimeClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(
            client.url("/v1/sessions/queue"),
            "http://localhost:9090/v1/sessions/queue"
        );
        assert_eq!(
            client.url(&format!(
                "/v1/sessions/queue?id={}",
                url_encode_query_component("session 1")
            )),
            "http://localhost:9090/v1/sessions/queue?id=session+1"
        );
        assert_eq!(
            client.events_stream_url(),
            "http://localhost:9090/v1/events/stream"
        );
    }

    #[test]
    fn events_parse_sse_frames() {
        let client =
            EventsClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(
            client.stream_url(),
            "http://localhost:9090/v1/events/stream"
        );
        let messages = client.parse("event: connected\nid: evt-1\ndata: {\"client_id\":\"sse-1\"}\n\ndata: plain\nretry: 1500\n\n");
        assert_eq!(messages.len(), 2);
        assert_eq!(messages[0].event, "connected");
        assert_eq!(messages[0].id, "evt-1");
        assert_eq!(messages[0].data.as_ref().unwrap()["client_id"], "sse-1");
        assert_eq!(messages[1].data, Some(serde_json::json!("plain")));
        assert_eq!(messages[1].retry, 1500);
    }

    #[test]
    fn reverie_types_serialize_incremental_bodies() {
        let journal: ReverieJournalResponse =
            serde_json::from_str(r#"{"thoughts":[{"id":"t1"}],"total":1,"limit":10,"offset":0}"#)
                .unwrap();
        assert_eq!(journal.total, 1);
        assert_eq!(journal.thoughts[0]["id"], serde_json::json!("t1"));

        let request = ReverieThinkRequest {
            event_type: "task_completed".to_string(),
            trigger: "sdk".to_string(),
        };
        let value = serde_json::to_value(request).unwrap();
        assert_eq!(value["event_type"], "task_completed");

        let client =
            ReverieClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(
            client.url("/v1/reverie/stats"),
            "http://localhost:9090/v1/reverie/stats"
        );
    }

    #[test]
    fn cost_types_deserialize_incremental_bodies() {
        let summary: CostSummaryResponse =
            serde_json::from_str(r#"{"today_cost":0.12,"month_cost":1.5,"summary":{"calls":2}}"#)
                .unwrap();
        assert_eq!(summary.today_cost, 0.12);
        assert_eq!(summary.extra["summary"]["calls"], serde_json::json!(2));

        let query = CostHistoryQuery {
            page: 2,
            limit: 25,
            task_id: "task/1".to_string(),
            model: "gpt-test".to_string(),
            ..CostHistoryQuery::default()
        };
        let client = CostClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(
            client.url("/v1/cost/summary"),
            "http://localhost:9090/v1/cost/summary"
        );
        assert_eq!(query.task_id, "task/1");

        let quota = SetQuotaRequest {
            tenant_id: "tenant-1".to_string(),
            quota: serde_json::json!({"max_chat_calls": 10}),
        };
        let value = serde_json::to_value(quota).unwrap();
        assert_eq!(value["tenant_id"], "tenant-1");
        assert_eq!(value["quota"]["max_chat_calls"], 10);
    }

    #[test]
    fn fork_types_deserialize_incremental_bodies() {
        let list: ForkListResponse = serde_json::from_str(
            r#"{"forks":[{"id":"fork_1","session_id":"s1","messages":[{"role":"user","content":"hi"}],"created_at":"2026-05-12T00:00:00Z"}]}"#,
        )
        .unwrap();
        assert_eq!(list.forks[0].id, "fork_1");
        assert_eq!(list.forks[0].messages[0].content, "hi");

        let request = ForkBranchRequest {
            fork_id: "fork_1".to_string(),
            at_index: 0,
            label: "alt".to_string(),
        };
        let value = serde_json::to_value(request).unwrap();
        assert_eq!(value["fork_id"], "fork_1");
        assert_eq!(value["label"], "alt");

        let client = ForkClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(client.url("/v1/fork"), "http://localhost:9090/v1/fork");
    }

    #[test]
    fn orchestrator_types_deserialize_incremental_bodies() {
        let status: OrchestratorStatusResponse = serde_json::from_str(
            r#"{"running":true,"adapters":["cursor"],"active_sessions":1,"event_count":2,"policy":{"allow_auto_launch":true}}"#,
        )
        .unwrap();
        assert!(status.running);
        assert_eq!(status.adapters[0], "cursor");
        assert_eq!(status.policy["allow_auto_launch"], serde_json::json!(true));

        let events: OrchestratorEventsResponse = serde_json::from_str(
            r#"{"events":[{"id":"e1","type":"task_assigned","task_id":"t1","message":"assigned"}],"total":1}"#,
        )
        .unwrap();
        assert_eq!(events.total, 1);
        assert_eq!(events.events[0].r#type, "task_assigned");

        let adapter = OrchestratorAdapterConfig {
            adapter_name: "custom".to_string(),
            binary: "worker.exe".to_string(),
            mcp_config_path: "mcp.json".to_string(),
            lifecycle: "persistent".to_string(),
            ..OrchestratorAdapterConfig::default()
        };
        let value = serde_json::to_value(adapter).unwrap();
        assert_eq!(value["adapter_name"], "custom");
        assert!(value.get("launch_args").is_none());

        let client =
            OrchestratorClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(
            client.url("/v1/orchestrator/status"),
            "http://localhost:9090/v1/orchestrator/status"
        );
    }

    #[test]
    fn skill_market_types_deserialize_incremental_bodies() {
        let found: SkillMarketSearchResponse = serde_json::from_str(
            r#"{"skills":[{"name":"doc_parse","version":"1.0.0","category":"data","tags":["docx"]}],"count":1}"#,
        )
        .unwrap();
        assert_eq!(found.count, 1);
        assert_eq!(found.skills[0].name, "doc_parse");

        let client =
            SkillMarketClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(
            client.url("/v1/market/stats"),
            "http://localhost:9090/v1/market/stats"
        );
    }

    #[test]
    fn project_types_deserialize_incremental_bodies() {
        let list: ProjectsListResponse = serde_json::from_str(
            r#"{"projects":[{"id":"p1","name":"云雀","repo_path":"C:/repo","default_caps":["read"]}]}"#,
        )
        .unwrap();
        assert_eq!(list.projects[0].id, "p1");
        assert_eq!(list.projects[0].default_caps[0], "read");

        let request = CreateProjectRequest {
            name: "云雀".to_string(),
            repo_path: "C:/repo".to_string(),
            ..CreateProjectRequest::default()
        };
        let value = serde_json::to_value(request).unwrap();
        assert_eq!(value["repo_path"], "C:/repo");
        assert!(value.get("repo_url").is_none());

        let client =
            ProjectsClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(
            client.url("/v1/projects"),
            "http://localhost:9090/v1/projects"
        );
    }

    #[test]
    fn notify_types_deserialize_incremental_bodies() {
        let channels: NotifyChannelsResponse = serde_json::from_str(
            r#"{"channels":[{"id":"feishu-main","type":"feishu","name":"Feishu","enabled":true}]}"#,
        )
        .unwrap();
        assert_eq!(channels.channels[0].id, "feishu-main");
        assert_eq!(channels.channels[0].r#type, "feishu");

        let share: NotifyShareResponse = serde_json::from_str(
            r#"{"ok":true,"sent_at":"2026-05-12T00:00:00Z","share":{"code":"yq_abc"},"channel":{"id":"feishu-main"}}"#,
        )
        .unwrap();
        assert!(share.ok);
        assert_eq!(share.share["code"], "yq_abc");

        let request = NotifyShareRequest {
            channel_id: "feishu-main".to_string(),
            message: "done".to_string(),
            ..NotifyShareRequest::default()
        };
        let value = serde_json::to_value(request).unwrap();
        assert_eq!(value["channel_id"], "feishu-main");

        let client =
            NotifyClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(
            client.url("/api/notify/channels"),
            "http://localhost:9090/api/notify/channels"
        );
    }

    #[test]
    fn lora_types_deserialize_incremental_bodies() {
        let status: LoRAStatusResponse = serde_json::from_str(
            r#"{"scheduler":{"status":"idle"},"active_model":"adapter-a","rolling_success_rate":0.8}"#,
        )
        .unwrap();
        assert_eq!(status["active_model"], "adapter-a");

        let trigger = TriggerLoRARequest {
            tenant_id: "default".to_string(),
        };
        let value = serde_json::to_value(trigger).unwrap();
        assert_eq!(value["tenant_id"], "default");

        let client = LoRAClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(
            client.url("/v1/lora/status"),
            "http://localhost:9090/v1/lora/status"
        );
    }

    #[test]
    fn trigger_types_deserialize_incremental_bodies() {
        let list: TriggerListResponse = serde_json::from_str(
            r#"{"triggers":[{"id":"tr_1","name":"review done","tenant_id":"default","type":"event","status":"enabled","actions":[{"kind":"notify"}],"source":"sdk"}],"total":1}"#,
        )
        .unwrap();
        assert_eq!(list.total, 1);
        assert_eq!(list.triggers[0].id, "tr_1");
        assert_eq!(list.triggers[0].extra["source"], "sdk");

        let emitted: TriggerEmitResponse =
            serde_json::from_str(r#"{"status":"emitted","event":"review.done"}"#).unwrap();
        assert_eq!(emitted.status, "emitted");

        let deleted: TriggerDeleteResponse = serde_json::from_str(r#"{"deleted":"tr_1"}"#).unwrap();
        assert_eq!(deleted.deleted, "tr_1");
    }

    #[test]
    fn triggers_client_trims_base_url_and_queries() {
        let client =
            TriggersClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(
            client.url("/v1/triggers/v2"),
            "http://localhost:9090/v1/triggers/v2"
        );
        assert_eq!(
            trigger_list_query(&TriggerListOptions {
                tenant_id: "default".to_string(),
                r#type: "event".to_string(),
                status: "enabled".to_string()
            }),
            "?tenant_id=default&type=event&status=enabled"
        );
        assert_eq!(
            trigger_history_query(&TriggerHistoryOptions {
                trigger_id: "tr_1".to_string(),
                limit: 2
            }),
            "?trigger_id=tr_1&limit=2"
        );
    }

    #[test]
    fn cron_types_deserialize_incremental_bodies() {
        let list: CronListResponse = serde_json::from_str(
            r#"{"jobs":[{"id":"job_1","name":"daily","schedule":{"type":"every","every_ms":60000},"payload":{"kind":"agentTurn","message":"ping"},"enabled":true,"created_at":"2026-05-12T00:00:00Z","run_count":0}]}"#,
        ).unwrap();
        assert_eq!(list.jobs[0].id, "job_1");
        assert_eq!(list.jobs[0].schedule.every_ms, 60000);

        let added: CronAddResponse = serde_json::from_str(
            r#"{"job":{"id":"job_2","name":"nightly","schedule":{"type":"cron","cron_expr":"0 2 * * *","timezone":"Asia/Shanghai"},"payload":{"kind":"systemEvent"},"enabled":true,"created_at":"2026-05-12T00:00:00Z","run_count":0}}"#,
        ).unwrap();
        assert_eq!(added.job.schedule.cron_expr, "0 2 * * *");

        let run: CronRunResponse = serde_json::from_str(
            r#"{"run":{"job_id":"job_1","run_id":"run_1","status":"success","output":"ok"}}"#,
        )
        .unwrap();
        assert_eq!(run.run.status, "success");
    }

    #[test]
    fn cron_client_trims_base_url() {
        let client = CronClient::new_with_client("http://localhost:9090/", reqwest::Client::new());
        assert_eq!(
            client.url("/v1/cron/list"),
            "http://localhost:9090/v1/cron/list"
        );
    }
}
