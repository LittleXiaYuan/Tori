import { existsSync, readdirSync, readFileSync, statSync } from "node:fs";
import { join, resolve } from "node:path";

const sdkRoot = process.cwd();
const repoRoot = resolve(sdkRoot, "../..");
const srcDir = join(sdkRoot, "src");
const pkg = JSON.parse(readFileSync(join(sdkRoot, "package.json"), "utf8"));
const tsconfig = JSON.parse(readFileSync(join(sdkRoot, "tsconfig.test.json"), "utf8"));
const runner = readFileSync(join(sdkRoot, "scripts/run-incremental-tests.mjs"), "utf8");
const readme = readFileSync(join(sdkRoot, "README.md"), "utf8");
const maxSliceLines = 350;
const maxSliceBytes = 12_000;

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
  const source = readFileSync(join(srcDir, `${name}.ts`), "utf8");
  const lineCount = source.split(/\r?\n/).length;
  const byteCount = Buffer.byteLength(source, "utf8");
  if (!existsSync(join(srcDir, `${name}.test.ts`))) fail(`missing test file for ${name}`);
  if (!tsconfigFiles.has(name)) fail(`tsconfig.test.json missing src/${name}.ts`);
  if (!runner.includes(`"src/${name}.ts"`)) fail(`run-incremental-tests.mjs missing src/${name}.ts`);
  if (!runner.includes(`"src/${name}.test.ts"`)) fail(`run-incremental-tests.mjs missing src/${name}.test.ts`);
  if (!runner.includes(`"${name}.test"`)) fail(`run-incremental-tests.mjs does not execute ${name}.test`);
  if (!runner.includes(`from "./${name}"`) && !runner.includes(`from './${name}'`)) fail(`run-incremental-tests.mjs missing import rewrite for ${name}`);
  if (!readme.includes(`yunque-client/${name}`)) fail(`README.md missing import documentation for yunque-client/${name}`);
  if (!readme.includes(`src/${name}.ts`)) fail(`README.md missing slice map row for src/${name}.ts`);
  if (/^\s*(?:import|export)\s+.*from\s+["']\.\/(?:client|sdk|types)\.gen["']/m.test(source)) {
    fail(`src/${name}.ts imports generated SDK internals instead of staying incremental`);
  }
  if (/messageFrom(?:Error)?Body/.test(source)) {
    const hasNestedErrorParser =
      source.includes('key === "error"') ||
      source.includes("messageFromErrorBody(value)") ||
      source.includes("messageFromBody(value)");
    const testSourcePath = join(srcDir, `${name}.test.ts`);
    const testSource = existsSync(testSourcePath) ? readFileSync(testSourcePath, "utf8") : "";
    if (!hasNestedErrorParser) fail(`src/${name}.ts error parser does not read nested error.message bodies`);
    if (!testSource.includes("nested")) fail(`src/${name}.test.ts missing nested gateway error coverage`);
  }
  if (lineCount > maxSliceLines) fail(`src/${name}.ts has ${lineCount} lines, exceeds incremental slice budget ${maxSliceLines}`);
  if (byteCount > maxSliceBytes) fail(`src/${name}.ts has ${byteCount} bytes, exceeds incremental slice budget ${maxSliceBytes}`);
}

const gatewayDir = join(repoRoot, "internal/controlplane/gateway");

function listGoFiles(dir) {
  const out = [];
  for (const name of readdirSync(dir)) {
    const path = join(dir, name);
    const stat = statSync(path);
    if (stat.isDirectory()) out.push(...listGoFiles(path));
    else if (name.endsWith(".go") && !name.endsWith("_test.go")) out.push(path);
  }
  return out;
}
const routePrefixAliases = new Set([
  ...exportedSlices,
  "auth", "token", "chat", "conversations", "subagent", "bots", "persona", "emotion", "instructions", "react", "sticker", "channels", "inbox",
  "memory", "graph", "identity", "embeddings", "search", "knowledge", "plugin-api", "plugins", "skills", "market", "approvals", "trace", "browser",
  "sessions", "router", "desktop", "system", "metrics", "cache", "modules", "tenants", "config", "backup", "tori", "upload", "speech", "heartbeat",
  "federation", "nl-config", "tasks", "planner", "missions", "state", "reflect", "documents", "workers", "dispatch", "setup", "usage", "quota", "cost",
  "cron", "scheduler", "tools", "sandbox", "sandboxes", "triggers", "workflows", "ide", "rbac", "skill-suggestions", "reverie", "models", "providers",
  "version", "ws", "events", "ext",
  // /api control planes that are intentionally owned by existing slices.
  "settings", "providers", "files", "browser", "connectors", "notify", "skillhub", "iterate", "trust", "audit", "review", "skillgrow", "breaker",
]);
const routeRe = /"(\/(?:v1|api)\/[^"?#]+)(?:\?[^"#]*)?"/g;
let routeRefs = 0;
for (const filePath of listGoFiles(gatewayDir)) {
  const text = readFileSync(filePath, "utf8");
  const rel = filePath.slice(gatewayDir.length + 1).replaceAll("\\", "/");
  for (const line of text.split(/\r?\n/)) {
    if (!line.includes("HandleFunc(") && !line.includes(".Handle(")) continue;
    for (const match of line.matchAll(routeRe)) {
      const path = match[1];
      const prefix = path.split("/")[2];
      if (!prefix) continue;
      routeRefs += 1;
      if (!routePrefixAliases.has(prefix)) fail(`unmapped route prefix "${prefix}" from ${rel}: ${path}`);
    }
  }
}

if (failures.length > 0) {
  console.error(`incremental coverage check failed (${failures.length} issues):`);
  for (const item of failures) console.error(`- ${item}`);
  process.exit(1);
}

console.log(`incremental coverage ok: ${srcSlices.length} slices, ${routeRefs} runtime /v1+/api route references checked, slice budget <= ${maxSliceLines} lines / ${maxSliceBytes} bytes`);
