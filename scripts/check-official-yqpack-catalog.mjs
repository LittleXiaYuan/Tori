#!/usr/bin/env node
// Validate the generated official .yqpack catalog against source manifests and
// artifact bytes. This is intentionally independent from Go tests so release
// operators can run it after building local dist artifacts.
import { createHash } from "node:crypto";
import { existsSync, readFileSync, readdirSync, statSync } from "node:fs";
import { dirname, join, relative, resolve, sep } from "node:path";

const repoRoot = resolve(import.meta.dirname, "..");
const argv = parseArgs(process.argv.slice(2));
const officialDir = resolveArgPath(argv["source-dir"] || "packs/official");
const catalogPath = resolveArgPath(argv.catalog || "dist/packs/official/catalog.json");

if (!existsSync(catalogPath)) fatal(`catalog not found: ${catalogPath}`);
const catalog = readJson(catalogPath);
if (catalog.schema !== "yunque.official-yqpack-catalog.v1") {
  fatal(`unexpected catalog schema: ${catalog.schema || "(missing)"}`);
}

const sourceManifests = findPackManifests(officialDir)
  .map((manifestPath) => ({ manifestPath, manifest: readJson(manifestPath) }))
  .sort((a, b) => String(a.manifest.id).localeCompare(String(b.manifest.id)));
const byID = new Map(sourceManifests.map((item) => [item.manifest.id, item]));
const entries = Array.isArray(catalog.entries) ? catalog.entries : [];
const errors = [];
const seen = new Set();

if (entries.length !== sourceManifests.length) {
  errors.push(`catalog entry count ${entries.length} != source manifest count ${sourceManifests.length}`);
}

for (const entry of entries) {
  const id = entry.id || entry.manifest?.id;
  const source = byID.get(id);
  if (!source) {
    errors.push(`catalog entry ${id || "(missing id)"} has no source manifest`);
    continue;
  }
  if (seen.has(id)) errors.push(`duplicate catalog entry id ${id}`);
  seen.add(id);
  const manifest = entry.manifest || {};
  compare(entry.version, source.manifest.version, `${id}: entry.version`);
  compare(manifest.id, source.manifest.id, `${id}: manifest.id`);
  compare(manifest.version, source.manifest.version, `${id}: manifest.version`);
  compare(entry.manifest_path || entry.manifestPath, slash(relative(repoRoot, source.manifestPath)), `${id}: manifest_path`);

  const artifactRel = entry.artifact_path || entry.artifactPath;
  if (typeof artifactRel !== "string" || artifactRel.trim() === "") {
    errors.push(`${id}: artifact_path missing`);
    continue;
  }
  const artifactPath = resolve(repoRoot, artifactRel);
  if (!existsSync(artifactPath)) {
    errors.push(`${id}: artifact not found: ${artifactRel}`);
    continue;
  }
  const info = statSync(artifactPath);
  compare(Number(entry.size_bytes ?? entry.sizeBytes), info.size, `${id}: size_bytes`);
  compare(String(entry.sha256 || ""), sha256File(artifactPath), `${id}: sha256`);
  if (!String(entry.package_url || entry.packageUrl || "").trim()) errors.push(`${id}: package_url missing`);
  if (entry.downloadable !== true) errors.push(`${id}: downloadable must be true`);
}

for (const source of sourceManifests) {
  if (!seen.has(source.manifest.id)) errors.push(`source manifest ${source.manifest.id} missing from catalog`);
}

if (errors.length > 0) {
  console.error(`[official-yqpack-check] ${errors.length} error(s):`);
  for (const error of errors) console.error(`  - ${error}`);
  process.exit(1);
}

console.log(`[official-yqpack-check] ok: ${entries.length} official yqpack catalog entries verified`);

function findPackManifests(root) {
  const out = [];
  walk(root, (path, dirent) => {
    if (dirent.isFile() && dirent.name === "pack.json") out.push(path);
  });
  return out;
}

function walk(dir, visit) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) walk(path, visit);
    else visit(path, entry);
  }
}

function compare(actual, expected, label) {
  if (actual !== expected) errors.push(`${label}: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`);
}

function sha256File(path) {
  return createHash("sha256").update(readFileSync(path)).digest("hex");
}

function readJson(path) {
  try {
    return JSON.parse(readFileSync(path, "utf8"));
  } catch (error) {
    fatal(`read ${path}: ${error.message}`);
  }
}

function parseArgs(args) {
  const out = {};
  for (let i = 0; i < args.length; i++) {
    const arg = args[i];
    if (!arg.startsWith("--")) continue;
    const key = arg.slice(2);
    const next = args[i + 1];
    if (next === undefined || next.startsWith("--")) {
      out[key] = true;
    } else {
      out[key] = next;
      i++;
    }
  }
  return out;
}

function resolveArgPath(path) {
  return resolve(repoRoot, String(path));
}

function slash(path) {
  return path.split(sep).join("/");
}

function fatal(message) {
  console.error(`[official-yqpack-check] error: ${message}`);
  process.exit(1);
}
