#!/usr/bin/env node
import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

const packageRoot = resolve(import.meta.dirname, "..");
const indexPath = resolve(packageRoot, "src/index.ts");
const failures = [];

function fail(message) {
  failures.push(message);
}

if (!existsSync(indexPath)) {
  fail("missing src/index.ts");
} else {
  const text = readFileSync(indexPath, "utf8");
  const starExportMatches = [...text.matchAll(/export\s+\*\s+from\s+["']([^"']+)["']/g)];
  const namedExportMatches = [...text.matchAll(/export(?:\s+type)?\s+\{[\s\S]*?\}\s+from\s+["']([^"']+)["']/g)];

  const allowedStarSources = new Set(["./types.gen", "./sdk.gen"]);
  const allowedNamedSources = new Set(["./client", "./client.gen"]);

  for (const match of starExportMatches) {
    if (!allowedStarSources.has(match[1])) {
      fail(`root index must not star-export ${match[1]}; keep the root surface explicit`);
    }
  }

  for (const match of namedExportMatches) {
    if (!allowedNamedSources.has(match[1])) {
      fail(`root index must not export from ${match[1]}; keep focused slices behind subpaths`);
    }
  }
}

if (failures.length > 0) {
  console.error("SDK root boundary check failed:");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}

console.log("SDK root boundary check ok: root exports stay on the generated surface");
