#!/usr/bin/env node
import { existsSync, readdirSync, readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { join, relative, resolve, sep } from "node:path";

const repoRoot = resolve(import.meta.dirname, "..");
const officialPacksDir = resolve(repoRoot, "packs/official");
const failures = [];

function fail(message) {
  failures.push(message);
}

function repoRel(path) {
  return relative(repoRoot, path).split(sep).join("/");
}

function readJSON(path) {
  try {
    return JSON.parse(readFileSync(path, "utf8"));
  } catch (error) {
    fail(`invalid json: ${repoRel(path)}: ${error.message}`);
    return undefined;
  }
}

function runCheck(label, args) {
  const result = spawnSync(process.execPath, args, { cwd: repoRoot, encoding: "utf8" });
  if (result.status !== 0) {
    fail(`${label} failed:\n${result.stderr || result.stdout}`);
  }
}

function arrayOfStrings(value) {
  return Array.isArray(value) && value.every((item) => typeof item === "string" && item.trim() !== "");
}

runCheck("manifest schema", ["scripts/check-pack-manifest-schema.mjs"]);
runCheck("capability registry", ["scripts/check-pack-capability-registry.mjs"]);
runCheck("pack usability", ["scripts/check-pack-usability.mjs", "--strict"]);
runCheck("official yqpack catalog", ["scripts/check-official-yqpack-catalog.mjs"]);
runCheck("description style", ["scripts/check-pack-description-style.mjs"]);
runCheck("scaffold dry-run", ["scripts/check-pack-scaffold.mjs"]);

if (!existsSync(officialPacksDir)) fail("missing packs/official");

let manifestCount = 0;
const packIDs = new Set();
if (existsSync(officialPacksDir)) {
  for (const dirent of readdirSync(officialPacksDir, { withFileTypes: true }).sort((a, b) => a.name.localeCompare(b.name))) {
    if (!dirent.isDirectory()) continue;
    const manifestPath = join(officialPacksDir, dirent.name, "pack.json");
    if (!existsSync(manifestPath)) {
      fail(`official pack missing manifest: ${repoRel(manifestPath)}`);
      continue;
    }
    manifestCount += 1;
    const manifest = readJSON(manifestPath);
    if (!manifest) continue;
    const location = repoRel(manifestPath);

    if (packIDs.has(manifest.id)) fail(`${location}: duplicate pack id ${manifest.id}`);
    packIDs.add(manifest.id);

    if (!String(manifest.id || "").startsWith("yunque.pack.")) fail(`${location}: id must start with yunque.pack.`);
    if (!arrayOfStrings(manifest.backend?.capabilities)) fail(`${location}: backend.capabilities[] is required`);
    if (!manifest.metadata?.usability) fail(`${location}: metadata.usability is required`);
    if (!manifest.metadata?.primaryActionLabel) fail(`${location}: metadata.primaryActionLabel is required`);
    if (!manifest.metadata?.primaryActionPath) fail(`${location}: metadata.primaryActionPath is required`);
    if (!manifest.metadata?.usageSurface) fail(`${location}: metadata.usageSurface is required`);

    const routeSpecs = manifest.backend?.routeSpecs ?? [];
    for (const [index, spec] of routeSpecs.entries()) {
      if (!spec.method || !spec.path) fail(`${location}: backend.routeSpecs[${index}] must declare method and path`);
      if (spec.path && !String(spec.path).startsWith("/")) fail(`${location}: backend.routeSpecs[${index}].path must be absolute`);
    }

    const routes = manifest.frontend?.routes ?? [];
    for (const [index, route] of routes.entries()) {
      if (!route.path || !String(route.path).startsWith("/")) fail(`${location}: frontend.routes[${index}].path must be absolute`);
    }
  }
}

if (manifestCount === 0) fail("expected at least one official pack manifest");

if (failures.length > 0) {
  console.error("Pack contract check failed:");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}

console.log(`Pack contract ok: ${manifestCount} official manifests validated against current Pack Runtime gates`);
