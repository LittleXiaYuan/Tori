import fs from "fs";
import path from "path";
import { fileURLToPath } from "url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(__dirname, "..");

const schemaPath = path.join(ROOT, "docs/spec/cogni-declaration.schema.json");
const guidePath = path.join(ROOT, "docs/spec/cogni-declaration.md");
const goPath = path.join(ROOT, "pkg/cogni/cogni.go");

function fail(message) {
  console.error(`Cogni declaration schema check failed: ${message}`);
  process.exitCode = 1;
}

if (!fs.existsSync(schemaPath)) fail("missing docs/spec/cogni-declaration.schema.json");
if (!fs.existsSync(guidePath)) fail("missing docs/spec/cogni-declaration.md");

const schema = JSON.parse(fs.readFileSync(schemaPath, "utf8"));
if (schema.$schema !== "https://json-schema.org/draft/2020-12/schema") fail("schema must use JSON Schema draft 2020-12");
if (schema.$id !== "https://yunque.ai/schemas/cogni-declaration.schema.json") fail("schema $id mismatch");
if (!schema.required?.includes("id")) fail("schema must require id");

const requiredTopLevel = [
  "id",
  "display_name",
  "description",
  "capsule",
  "activation",
  "surface",
  "context",
  "mcp",
  "workflows",
  "experience",
  "economics",
  "memory",
  "priority",
  "exclusive",
  "checks",
];
for (const key of requiredTopLevel) {
  if (!schema.properties?.[key]) fail(`schema missing top-level property ${key}`);
}

const requiredDefs = [
  "activationRules",
  "perceptionRule",
  "toolSurface",
  "contextInjection",
  "mcpConfig",
  "mcpServerDef",
  "mcpToolFilter",
  "workflowDef",
  "workflowStep",
  "experienceConfig",
  "economicsConfig",
  "memoryPolicy",
  "activationCheck",
];
for (const key of requiredDefs) {
  if (!schema.$defs?.[key]) fail(`schema missing $defs.${key}`);
}

const go = fs.readFileSync(goPath, "utf8");
for (const jsonTag of requiredTopLevel) {
  if (!go.includes(`json:"${jsonTag}`)) fail(`Go Declaration appears to be missing json tag ${jsonTag}`);
}

const guide = fs.readFileSync(guidePath, "utf8");
for (const token of [
  "Cogni Declaration 公开规范",
  "pkg/cogni.Declaration",
  "activation",
  "surface",
  "context",
  "memory",
  "workflows",
  "checks",
  "docs/spec/cogni-declaration.schema.json",
  "node scripts/check-cogni-declaration-schema.mjs",
]) {
  if (!guide.includes(token)) fail(`guide missing token ${JSON.stringify(token)}`);
}

if (process.exitCode) process.exit(process.exitCode);
console.log("Cogni declaration schema check ok: schema, guide, and pkg/cogni.Declaration top-level JSON fields are aligned");
