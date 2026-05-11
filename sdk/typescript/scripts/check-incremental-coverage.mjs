import { existsSync, readdirSync, readFileSync } from "node:fs";
import { join, resolve } from "node:path";

const sdkRoot = process.cwd();
const repoRoot = resolve(sdkRoot, "../..");
const srcDir = join(sdkRoot, "src");
const pkg = JSON.parse(readFileSync(join(sdkRoot, "package.json"), "utf8"));
const tsconfig = JSON.parse(readFileSync(join(sdkRoot, "tsconfig.test.json"), "utf8"));
const runner = readFileSync(join(sdkRoot, "scripts/run-incremental-tests.mjs"), "utf8");

const generated = new Set(["client.gen", "sdk.gen", "types.gen", "index"]);
const srcSlices = readdirSync(srcDir)
  .filter((name) => name.endsWith(".ts") && !name.endsWith(".test.ts"))
  .map((name) => name.replace(/\.ts$/, ""))
  .filter((name) => !generated.has(name))
  .sort();
const exportedSlices = Object.keys(pkg.exports ?? {})
  .filter((key) => key.startsWith("./"))
  .map((key) => key.slice(2))
  .sort();

const failures = [];
function fail(message) { failures.push(message); }
function diff(a, b) { const bs = new Set(b); return a.filter((item) => !bs.has(item)); }

for (const name of diff(srcSlices, exportedSlices)) fail(`missing package export for src/${name}.ts`);
for (const name of diff(exportedSlices, srcSlices)) fail(`package export has no src/${name}.ts`);

const tsconfigEntries = tsconfig.files ?? tsconfig.include ?? [];
const tsconfigFiles = new Set(tsconfigEntries.map((file) => file.replace(/^src\//, "").replace(/\.ts$/, "")));
for (const name of srcSlices) {
  if (!existsSync(join(srcDir, `${name}.test.ts`))) fail(`missing test file for ${name}`);
  if (!tsconfigFiles.has(name)) fail(`tsconfig.test.json missing src/${name}.ts`);
  if (!runner.includes(`"src/${name}.ts"`)) fail(`run-incremental-tests.mjs missing src/${name}.ts`);
  if (!runner.includes(`"src/${name}.test.ts"`)) fail(`run-incremental-tests.mjs missing src/${name}.test.ts`);
  if (!runner.includes(`"${name}.test"`)) fail(`run-incremental-tests.mjs does not execute ${name}.test`);
  if (!runner.includes(`from "./${name}"`) && !runner.includes(`from './${name}'`)) fail(`run-incremental-tests.mjs missing import rewrite for ${name}`);
}

const gatewayDir = join(repoRoot, "internal/controlplane/gateway");
const routePrefixAliases = new Set([
  ...exportedSlices,
  "auth", "token", "chat", "conversations", "subagent", "bots", "persona", "emotion", "instructions", "react", "sticker", "channels", "inbox",
  "memory", "graph", "identity", "embeddings", "search", "knowledge", "plugin-api", "plugins", "skills", "market", "approvals", "trace", "browser",
  "sessions", "router", "desktop", "system", "metrics", "cache", "modules", "tenants", "config", "backup", "tori", "upload", "speech", "heartbeat",
  "federation", "nl-config", "tasks", "planner", "missions", "state", "reflect", "documents", "workers", "dispatch", "setup", "usage", "quota", "cost",
  "cron", "scheduler", "tools", "sandbox", "sandboxes", "triggers", "workflows", "ide", "rbac", "skill-suggestions", "reverie", "models", "providers",
  "version", "ws", "events", "ext",
]);
const routeRe = /"(\/v1\/[^"?#]+)(?:\?[^"#]*)?"/g;
let routeRefs = 0;
for (const file of readdirSync(gatewayDir).filter((name) => name.endsWith(".go"))) {
  const text = readFileSync(join(gatewayDir, file), "utf8");
  for (const match of text.matchAll(routeRe)) {
    const path = match[1];
    const prefix = path.split("/")[2];
    if (!prefix) continue;
    routeRefs += 1;
    if (!routePrefixAliases.has(prefix)) fail(`unmapped /v1 route prefix "${prefix}" from ${file}: ${path}`);
  }
}

if (failures.length > 0) {
  console.error(`incremental coverage check failed (${failures.length} issues):`);
  for (const item of failures) console.error(`- ${item}`);
  process.exit(1);
}

console.log(`incremental coverage ok: ${srcSlices.length} slices, ${routeRefs} /v1 route references checked`);
