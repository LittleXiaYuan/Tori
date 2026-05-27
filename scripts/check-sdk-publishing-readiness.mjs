#!/usr/bin/env node
import { existsSync, readFileSync } from "node:fs";
import { relative, resolve, sep } from "node:path";
import { spawnSync } from "node:child_process";

const repoRoot = resolve(import.meta.dirname, "..");
const failures = [];

function fail(message) {
  failures.push(message);
}

function repoRel(path) {
  return relative(repoRoot, path).split(sep).join("/");
}

function read(path) {
  return readFileSync(resolve(repoRoot, path), "utf8");
}

function file(path) {
  const absolute = resolve(repoRoot, path);
  if (!existsSync(absolute)) fail(`missing ${path}`);
  return absolute;
}

function run(name, command, args, cwd) {
  const result = spawnSync(command, args, {
    cwd: resolve(repoRoot, cwd),
    encoding: "utf8",
    shell: false,
  });
  if (result.status !== 0) {
    const output = `${result.stdout ?? ""}\n${result.stderr ?? ""}`.trim();
    fail(`${name} failed${output ? `: ${output.split(/\r?\n/).slice(-8).join(" | ")}` : ""}`);
  }
}

function parseJSON(path) {
  try {
    return JSON.parse(read(path));
  } catch (error) {
    fail(`invalid JSON ${path}: ${error.message}`);
    return {};
  }
}

function parseTomlString(source, key) {
  const match = source.match(new RegExp(`^${key}\\s*=\\s*"([^"]+)"`, "m"));
  return match?.[1] ?? "";
}

function hasTomlKey(source, key) {
  return new RegExp(`^${key}\\s*=`, "m").test(source);
}

function checkTypeScript() {
  file("packages/yunque-client/package.json");
  file("packages/yunque-client/README.md");
  file("packages/yunque-client/src/index.ts");
  const pkg = parseJSON("packages/yunque-client/package.json");
  if (pkg.name !== "yunque-client") fail("TypeScript package name must be yunque-client");
  if (!/^\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$/.test(pkg.version ?? "")) fail("TypeScript package version must be semver");
  if (pkg.license !== "Apache-2.0") fail("TypeScript package license must remain Apache-2.0 unless SDK-VERSIONING.md is updated");
  if (pkg.type !== "module") fail("TypeScript package must publish ESM metadata");
  if (pkg.sideEffects !== false) fail("TypeScript package must declare sideEffects=false");
  for (const key of [".", "./agent-kit", "./packs", "./workloads", "./cognisdk-schema", "./memory-time-travel"]) {
    if (!pkg.exports?.[key]?.types || !pkg.exports?.[key]?.import) fail(`TypeScript package missing export ${key}`);
  }
  for (const script of ["test", "typecheck", "check:sdk-manifests", "check:pack", "prepublishOnly"]) {
    if (!pkg.scripts?.[script]) fail(`TypeScript package missing script ${script}`);
  }
  const prepublish = String(pkg.scripts?.prepublishOnly ?? "");
  for (const token of ["npm run test", "npm run typecheck", "npm run check:sdk-manifests", "npm run check:pack", "npm audit --audit-level=high"]) {
    if (!prepublish.includes(token)) fail(`TypeScript prepublishOnly missing ${token}`);
  }
  const readme = read("packages/yunque-client/README.md");
  for (const token of ["npm i yunque-client", "Versioning and compatibility", "npm run check:pack"]) {
    if (!readme.includes(token)) fail(`TypeScript README missing ${token}`);
  }
}

function checkPython() {
  file("sdk/python/pyproject.toml");
  file("sdk/python/MANIFEST.in");
  file("sdk/python/README.md");
  file("sdk/python/yunque/__init__.py");
  file("sdk/python/yunque_client/__init__.py");
  const pyproject = read("sdk/python/pyproject.toml");
  const setup = read("sdk/python/setup.py");
  if (!pyproject.includes("[build-system]")) fail("Python pyproject missing [build-system]");
  if (parseTomlString(pyproject, "name") !== "yunque") fail("Python package name must be yunque");
  if (!/^\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$/.test(parseTomlString(pyproject, "version"))) fail("Python package version must be semver");
  if (parseTomlString(pyproject, "license") !== "MIT") fail("Python package license must be MIT to match repository LICENSE");
  if (!pyproject.includes("attrs>=") || !pyproject.includes("httpx>=") || !pyproject.includes("python-dateutil>=")) {
    fail("Python package dependencies must include attrs, httpx, and python-dateutil for generated yunque_client");
  }
  for (const token of ["include = [\"yunque*\", \"yunque_client*\"]", "Homepage", "Source", "Issues"]) {
    if (!pyproject.includes(token)) fail(`Python pyproject missing ${token}`);
  }
  if (!setup.includes("setup()")) fail("Python setup.py shim must call setup()");
  const readme = read("sdk/python/README.md");
  for (const token of ["pip install yunque", "yunque", "yunque_client"]) {
    if (!readme.includes(token)) fail(`Python README missing ${token}`);
  }
}

function checkRust() {
  file("sdk/rust/Cargo.toml");
  file("sdk/rust/Cargo.lock");
  file("sdk/rust/README.md");
  file("sdk/rust/build.rs");
  file("sdk/rust/src/lib.rs");
  const cargo = read("sdk/rust/Cargo.toml");
  if (parseTomlString(cargo, "name") !== "yunque-client") fail("Rust crate name must be yunque-client");
  if (!/^\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$/.test(parseTomlString(cargo, "version"))) fail("Rust crate version must be semver");
  if (parseTomlString(cargo, "license") !== "MIT") fail("Rust crate license must be MIT to match repository LICENSE");
  if (/^publish\s*=\s*false/m.test(cargo)) fail("Rust crate must not set publish=false for T31 release readiness");
  for (const key of ["repository", "homepage", "documentation", "readme", "keywords", "categories", "include"]) {
    if (!hasTomlKey(cargo, key)) fail(`Rust Cargo.toml missing ${key}`);
  }
  for (const dep of ["progenitor-client", "reqwest", "serde", "serde_json", "chrono", "uuid", "futures", "bytes", "base64", "http"]) {
    if (!cargo.includes(`${dep} =`)) fail(`Rust Cargo.toml missing dependency ${dep}`);
  }
  const build = read("sdk/rust/build.rs");
  if (!build.includes("docs") || !build.includes("openapi.yaml") || !build.includes("progenitor")) {
    fail("Rust build.rs must generate from docs/openapi.yaml with progenitor");
  }
  const readme = read("sdk/rust/README.md");
  for (const token of ["cargo add yunque-client", "Source spec", "cargo check"]) {
    if (!readme.includes(token)) fail(`Rust README missing ${token}`);
  }
}

function checkSharedDocs() {
  file("docs/SDK-VERSIONING.md");
  const docs = existsSync(resolve(repoRoot, "docs/SDK-PUBLISHING.md")) ? read("docs/SDK-PUBLISHING.md") : "";
  if (!docs) fail("missing docs/SDK-PUBLISHING.md");
  for (const token of ["pip install yunque", "cargo add yunque-client", "npm i yunque-client", "npm publish", "twine upload", "cargo publish", "scripts/check-sdk-publishing-readiness.mjs"]) {
    if (docs && !docs.includes(token)) fail(`SDK publishing docs missing ${token}`);
  }
  const docsReadme = read("docs/README.md");
  if (!docsReadme.includes("SDK-PUBLISHING.md")) fail("docs/README.md must link SDK-PUBLISHING.md");
}

checkTypeScript();
checkPython();
checkRust();
checkSharedDocs();

if (process.argv.includes("--run-pack-dry-run")) {
  run("npm pack dry-run", process.execPath, [resolve(repoRoot, "packages/yunque-client/scripts/check-pack.mjs")], "packages/yunque-client");
}
if (process.argv.includes("--run-cargo-package")) {
  run("cargo package dry-run", "cargo", ["package", "--allow-dirty", "--no-verify"], "sdk/rust");
}

if (failures.length > 0) {
  console.error("SDK publishing readiness check failed:");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}

console.log("SDK publishing readiness ok: PyPI/npm/crates metadata, docs, and release gates are present");
