#!/usr/bin/env node
import { existsSync } from "node:fs";
import { resolve } from "node:path";
import { spawnSync } from "node:child_process";

const repoRoot = resolve(import.meta.dirname, "..");
const failures = [];

function fail(message) {
  failures.push(message);
}

const expectedFiles = [
  "packs/examples/verifier-pack/pack.json",
  "packs/examples/verifier-pack/README.md",
  "internal/packs/verifierpack/handler.go",
  "heroui-web/src/app/packs/verifier-pack/page.tsx",
];

for (const file of expectedFiles) {
  if (existsSync(resolve(repoRoot, file))) {
    fail(`pre-existing verifier scaffold path would make dry-run ambiguous: ${file}`);
  }
}

const result = spawnSync(
  process.execPath,
  ["scripts/scaffold-pack.mjs", "verifier-pack", "--name", "Verifier Pack", "--dry-run", "--json"],
  { cwd: repoRoot, encoding: "utf8" },
);

if (result.status !== 0) {
  fail(`scaffold dry-run exited with ${result.status}: ${result.stderr || result.stdout}`);
}

let payload;
try {
  payload = JSON.parse(result.stdout);
} catch (error) {
  fail(`scaffold --json did not emit valid JSON: ${error.message}`);
}

if (payload) {
  if (payload.slug !== "verifier-pack") fail("payload.slug must be verifier-pack");
  if (payload.packId !== "yunque.pack.verifier-pack") fail("payload.packId must match yunque.pack.verifier-pack");
  if (payload.dryRun !== true) fail("payload.dryRun must be true");
  if (payload.manifest?.id !== "yunque.pack.verifier-pack") fail("manifest.id must match yunque.pack.verifier-pack");
  if (!Array.isArray(payload.manifest?.frontend?.menus) || payload.manifest.frontend.menus.length === 0) {
    fail("manifest.frontend.menus must be present");
  }
  if (!Array.isArray(payload.manifest?.frontend?.routes) || payload.manifest.frontend.routes.length === 0) {
    fail("manifest.frontend.routes must be present");
  }
  if (!payload.manifest?.sdk?.typescript) fail("manifest.sdk.typescript must be present");
  if (payload.manifest?.update?.rollback !== true) fail("manifest.update.rollback must be true");
  for (const file of expectedFiles) {
    if (!payload.files?.includes(file)) fail(`payload.files missing ${file}`);
  }
}

for (const file of expectedFiles) {
  if (existsSync(resolve(repoRoot, file))) {
    fail(`dry-run created a file unexpectedly: ${file}`);
  }
}

if (failures.length > 0) {
  console.error("Pack scaffold check failed:");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}

console.log("Pack scaffold ok: dry-run JSON contract verified without writing files");
