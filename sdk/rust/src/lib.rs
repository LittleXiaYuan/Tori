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
