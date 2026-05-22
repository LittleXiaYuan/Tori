#!/usr/bin/env node
import { existsSync, readdirSync, readFileSync } from "node:fs";
import { join, relative, resolve, sep } from "node:path";

const repoRoot = resolve(import.meta.dirname, "..");
const registryPath = resolve(repoRoot, "packs/capability-registry.json");
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

if (!existsSync(registryPath)) {
  fail("missing packs/capability-registry.json");
}
if (!existsSync(officialPacksDir)) {
  fail("missing packs/official");
}

const registry = existsSync(registryPath) ? readJSON(registryPath) : undefined;
const registryEntries = Array.isArray(registry?.capabilities) ? registry.capabilities : [];
if (registry && registry.version !== 1) fail("capability registry version must be 1");
if (registry && !Array.isArray(registry.capabilities)) fail("capability registry must contain capabilities[]");

const registered = new Map();
let previousID = "";
for (const [index, entry] of registryEntries.entries()) {
  const id = entry?.id;
  if (typeof id !== "string" || id.trim() === "") {
    fail(`capability registry entry #${index} must have a non-empty id`);
    continue;
  }
  if (!/^[a-z][a-z0-9_]*(\.[a-z0-9_]+)*$/.test(id)) {
    fail(`capability id has invalid shape: ${id}`);
  }
  if (previousID && previousID.localeCompare(id) > 0) {
    fail(`capability registry must be sorted by id: ${previousID} appears before ${id}`);
  }
  previousID = id;
  if (registered.has(id)) fail(`duplicate capability id in registry: ${id}`);
  registered.set(id, entry);
  if (typeof entry.ownerPack !== "string" || !entry.ownerPack.startsWith("yunque.pack.")) {
    fail(`registry entry ${id} must declare ownerPack`);
  }
  if (!["active", "optional", "deprecated"].includes(entry.lifecycle)) {
    fail(`registry entry ${id} must use lifecycle active|optional|deprecated`);
  }
}

const manifestCapabilities = new Map();
if (existsSync(officialPacksDir)) {
  for (const dirent of readdirSync(officialPacksDir, { withFileTypes: true })) {
    if (!dirent.isDirectory()) continue;
    const manifestPath = join(officialPacksDir, dirent.name, "pack.json");
    if (!existsSync(manifestPath)) {
      fail(`official pack missing pack.json: ${repoRel(join(officialPacksDir, dirent.name))}`);
      continue;
    }
    const manifest = readJSON(manifestPath);
    if (!manifest) continue;
    const packID = manifest.id;
    const capabilities = manifest.backend?.capabilities;
    if (!Array.isArray(capabilities) || capabilities.length === 0) {
      fail(`${repoRel(manifestPath)} must declare backend.capabilities[]`);
      continue;
    }
    const seenInPack = new Set();
    for (const capability of capabilities) {
      if (typeof capability !== "string" || capability.trim() === "") {
        fail(`${repoRel(manifestPath)} declares an empty capability`);
        continue;
      }
      if (seenInPack.has(capability)) {
        fail(`${repoRel(manifestPath)} duplicates capability ${capability}`);
      }
      seenInPack.add(capability);
      if (manifestCapabilities.has(capability)) {
        const previous = manifestCapabilities.get(capability);
        fail(`capability ${capability} is declared by multiple packs: ${previous.packID} and ${packID}`);
      }
      manifestCapabilities.set(capability, { packID, manifestPath });
      const registeredEntry = registered.get(capability);
      if (!registeredEntry) {
        fail(`${repoRel(manifestPath)} declares unregistered capability: ${capability}`);
        continue;
      }
      if (registeredEntry.ownerPack !== packID) {
        fail(`capability ${capability} owner mismatch: registry=${registeredEntry.ownerPack}, manifest=${packID}`);
      }
    }
  }
}

for (const [capability, entry] of registered.entries()) {
  if (!manifestCapabilities.has(capability) && entry.lifecycle !== "deprecated") {
    fail(`registry capability ${capability} is not declared by any official pack`);
  }
}

if (failures.length > 0) {
  console.error("Pack capability registry check failed:");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}

console.log(`Pack capability registry ok: ${registryEntries.length} capabilities across ${new Set([...manifestCapabilities.values()].map((entry) => entry.packID)).size} official packs`);
