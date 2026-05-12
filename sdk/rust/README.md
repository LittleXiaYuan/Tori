# yunque-client (Rust)

Auto-generated Rust client for the Yunque (云雀) Agent HTTP API.

- Source spec: [`docs/openapi.yaml`](../../docs/openapi.yaml)
- Generator: [`progenitor`](https://github.com/oxidecomputer/progenitor) (build-time)
- Runtime: [`reqwest`](https://crates.io/crates/reqwest) with `rustls-tls`
- 425 async methods, ~19000 LOC of generated code

## Add to your project

```toml
[dependencies]
yunque-client = { path = "../yunque-agent/sdk/rust" }
tokio = { version = "1", features = ["rt-multi-thread", "macros"] }
```

(Path-dep for now; once published, use `cargo add yunque-client`.)

## Quick start

```rust
use yunque_client::Client;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let client = Client::new("http://localhost:9090");

    // Every endpoint has a typed async method on the Client.
    // Names follow `<method>_<sanitised_path>`, e.g. `get_v1_cognis`.
    let cognis = client.get_v1_cognis().send().await?;
    println!("{:?}", cognis.into_inner());

    // Cogni operations (curated names from the spec):
    // - generate_cogni / list_cognis / evolve_cogni / run_cogni_workflow
    // - get_cogni_economics / get_cogni_federation_status / ...

    Ok(())
}
```

## Authentication

```rust
let client = Client::new_with_client(
    "http://localhost:9090",
    reqwest::Client::builder()
        .default_headers({
            let mut h = reqwest::header::HeaderMap::new();
            h.insert(
                reqwest::header::AUTHORIZATION,
                "Bearer <your-jwt>".parse()?,
            );
            h
        })
        .build()?,
);
```

## Regenerating

The client is regenerated **automatically** on every `cargo build` —
`build.rs` reads `docs/openapi.yaml`, so any spec change triggers a rebuild.

```bash
# 1. Refresh OpenAPI from gateway routes
cd ../..        # back to repo root
make openapi

# 2. Rebuild the Rust SDK
cd sdk/rust
cargo build
cargo check     # quick verification
```

## Layout

| File | Purpose |
|---|---|
| `Cargo.toml` | Dependencies + build deps (`progenitor`, `openapiv3`, `prettyplease`) |
| `build.rs` | Reads spec, downgrades `openapi: 3.1.0` → `3.0.3` in-memory, runs progenitor |
| `src/lib.rs` | `include!` for the generated `yunque_client.rs` |
| `target/.../out/yunque_client.rs` | The actual generated code (~19000 LOC, not committed) |

## Status & caveats

- **OpenAPI 3.1 → 3.0.3 downgrade**: `progenitor 0.10` only supports 3.0.x
  parsing. We do an in-memory string substitution (`openapi: 3.1.0` →
  `openapi: 3.0.3`) inside `build.rs`. Our spec doesn't use 3.1-only features
  (yet), so this is safe today.
- **Streaming endpoints** (`/v1/chat/stream`, `/v1/events/stream`): generated
  as standard reqwest calls — for real SSE consumption, use
  [`eventsource-stream`](https://crates.io/crates/eventsource-stream) on the
  raw response body.
- **Lint warning**: `elided_named_lifetimes` rename warning comes from
  progenitor's generated output; benign on rustc 1.94+.
- **Body schemas** are mostly `serde_json::Value` placeholders since the spec
  is path-only. Hand-edit `docs/openapi.yaml` request/response bodies, then
  rebuild.

## Lightweight State Kernel helper

The generated client offers broad OpenAPI coverage. For sidecars, CLIs, plugins,
or dashboards that only need the agent state layer, use the hand-written
`StateClient` instead. It avoids coupling callers to the large generated method
surface and mirrors the incremental helpers in the TypeScript, Go, and Python SDKs.

```rust
use yunque_client::{StateClient, StateGoal};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let state = StateClient::new("http://localhost:9090", "<plugin-or-api-token>")?;

    let snapshot = state.snapshot().await?;
    println!("focus: {}", snapshot.focus);
    println!("goals: {}", snapshot.goals.len());
    println!("skills: {}", snapshot.capabilities.total_skills);

    let actions = state.actions().await?;
    let caps = state.capabilities().await?;
    let focus = state.focus().await?;
    let resources = state.resources().await?;

    let saved = state.save_goal(&StateGoal {
        title: "Ship a Rust SDK state slice".to_string(),
        priority: 2,
        ..StateGoal::default()
    }).await?;

    Ok(())
}
```
