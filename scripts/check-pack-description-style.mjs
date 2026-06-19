#!/usr/bin/env node
import { existsSync, readdirSync, readFileSync } from "node:fs";
import { join, relative, resolve, sep } from "node:path";

const repoRoot = resolve(import.meta.dirname, "..");
const officialPacksDir = resolve(repoRoot, "packs/official");
const failures = [];

const MAX_DESCRIPTION_CHARS = 96;
const MAX_EXAMPLE_CHARS = 96;
const FORBIDDEN_TECHNICAL_FRAGMENTS = [
  "handoff plan",
  "writeback",
  "write-back",
  "preview gate",
  "cutover readiness",
  "collector pipeline",
];

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

if (!existsSync(officialPacksDir)) fail("missing packs/official");

let manifestCount = 0;
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
    const description = String(manifest.description ?? "").trim();
    if (!description) fail(`${location}: description is required`);
    if (description.length > MAX_DESCRIPTION_CHARS) {
      fail(`${location}: description must be a concise one-line summary <= ${MAX_DESCRIPTION_CHARS} chars (got ${description.length})`);
    }
    if (/\r|\n/.test(description)) fail(`${location}: description must stay on one line`);
    if (!/[。.!?]$/.test(description)) fail(`${location}: description must end with punctuation`);
    for (const fragment of FORBIDDEN_TECHNICAL_FRAGMENTS) {
      if (description.toLowerCase().includes(fragment.toLowerCase())) {
        fail(`${location}: description exposes technical handoff jargon: ${JSON.stringify(fragment)}`);
      }
    }

    const metadata = manifest.metadata ?? {};
    if (metadata.descriptionStyle && metadata.descriptionStyle !== "one-line-plus-three-examples") {
      fail(`${location}: metadata.descriptionStyle must be one-line-plus-three-examples when present`);
    }
    const examples = [metadata.example1, metadata.example2, metadata.example3];
    for (const [index, example] of examples.entries()) {
      const key = `metadata.example${index + 1}`;
      if (typeof example !== "string" || example.trim() === "") {
        fail(`${location}: ${key} is required for the one-line + three examples style`);
        continue;
      }
      const normalized = example.trim();
      if (normalized.length > MAX_EXAMPLE_CHARS) fail(`${location}: ${key} must be <= ${MAX_EXAMPLE_CHARS} chars (got ${normalized.length})`);
      if (/\r|\n/.test(normalized)) fail(`${location}: ${key} must stay on one line`);
      if (!/[。.!?]$/.test(normalized)) fail(`${location}: ${key} must end with punctuation`);
    }
    if (new Set(examples.map((example) => String(example).trim())).size !== 3) {
      fail(`${location}: metadata.example1..3 must be distinct`);
    }
  }
}

if (manifestCount === 0) fail("expected at least one official pack manifest");

if (failures.length > 0) {
  console.error("Official pack description style check failed:");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}

console.log(`Official pack description style ok: ${manifestCount} manifests have concise descriptions and three examples`);
