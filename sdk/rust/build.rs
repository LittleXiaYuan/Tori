// build.rs — Generate the Yunque API client from docs/openapi.yaml.
//
// Run: cargo build (the spec change will trigger a rebuild)
//
// We read the spec, downgrade the `openapi:` declaration to 3.0.3 in-memory
// (progenitor 0.10 expects OpenAPI 3.0.x, but our spec only uses features
// compatible with both 3.0 and 3.1), then feed it to progenitor.

use std::env;
use std::fs;
use std::path::PathBuf;

fn main() {
    let spec_path = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("..")
        .join("..")
        .join("docs")
        .join("openapi.yaml");

    println!("cargo:rerun-if-changed={}", spec_path.display());

    let raw = fs::read_to_string(&spec_path)
        .unwrap_or_else(|e| panic!("failed to read {}: {}", spec_path.display(), e));

    // Downgrade `3.1.x` → `3.0.3` so progenitor's openapiv3 parser accepts it.
    // Our spec doesn't use 3.1-only features (yet).
    let normalized = raw.replace("openapi: 3.1.0", "openapi: 3.0.3");

    // Parse YAML → JSON (progenitor consumes openapiv3::OpenAPI from JSON).
    let yaml: serde_yaml::Value =
        serde_yaml::from_str(&normalized).expect("failed to parse OpenAPI YAML");
    let json = serde_json::to_string(&yaml).expect("failed to convert YAML to JSON");
    let spec: openapiv3::OpenAPI =
        serde_json::from_str(&json).expect("failed to parse OpenAPI JSON via openapiv3");

    let mut generator = progenitor::Generator::default();
    let tokens = generator
        .generate_tokens(&spec)
        .expect("progenitor failed to generate API");
    let ast = syn::parse2(tokens).expect("progenitor produced invalid Rust tokens");
    let pretty = prettyplease::unparse(&ast);

    let out_dir = env::var_os("OUT_DIR").expect("OUT_DIR not set");
    let out_path = PathBuf::from(out_dir).join("yunque_client.rs");
    fs::write(&out_path, pretty).expect("failed to write generated code");

    println!("cargo:warning=yunque-client: generated {}", out_path.display());
}
