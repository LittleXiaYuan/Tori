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

fn trim_base_url(base_url: String) -> String {
    base_url.trim_end_matches('/').to_string()
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
}
