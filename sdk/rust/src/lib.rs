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
