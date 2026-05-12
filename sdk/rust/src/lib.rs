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
    pub async fn strategies(
        &self,
        options: &ReflectOptions,
    ) -> Result<String, reqwest::Error> {
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
        self.http
            .get(self.url(&path))
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

    fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }
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
}
