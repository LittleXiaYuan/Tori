import {
  buildCognitionSchemaArtifacts,
  cognitionSchemaFileName,
  findCognitionSchema,
  isCognitionSchemaName,
  normalizeCognitionSchemaCatalog,
  verifyCognitionSchemaArtifacts,
  type CognitionSchemaCatalogEntry,
} from "./cognisdk-schema";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }

const catalogInput = [
  {
    name: "pack-bundle",
    title: "Cognition SDK Pack Bundle",
    schema: "https://yunque.local/schemas/cognisdk/pack-bundle.json",
    description: "Portable bundle.",
    schema_document: { $id: "https://yunque.local/schemas/cognisdk/pack-bundle.json", type: "object" },
  },
  {
    name: "pack-bundle-apply-checklist-summary",
    title: "Cognition SDK Pack Bundle Apply Checklist Summary",
    schema: "https://yunque.local/schemas/cognisdk/pack-bundle-apply-checklist-summary.json",
  },
] satisfies unknown[];

test("normalizes schema catalogs from CLI JSON output", () => {
  const catalog = normalizeCognitionSchemaCatalog(catalogInput);
  assertEqual(catalog.length, 2);
  assertEqual(catalog[0]?.name, "pack-bundle");
  assertEqual(catalog[0]?.schema_document?.$id, "https://yunque.local/schemas/cognisdk/pack-bundle.json");
  assert(findCognitionSchema(catalog, "pack-bundle-apply-checklist-summary"));
});

test("builds portable artifact filenames", () => {
  const catalog = normalizeCognitionSchemaCatalog(catalogInput);
  const artifacts = buildCognitionSchemaArtifacts(catalog);
  assertEqual(cognitionSchemaFileName("pack-bundle"), "pack-bundle.schema.json");
  assertDeepEqual(artifacts.map((item) => item.file), ["pack-bundle.schema.json", "pack-bundle-apply-checklist-summary.schema.json"]);
});

test("verifies schema artifact ids for frontend and plugin bundles", () => {
  const catalog = normalizeCognitionSchemaCatalog(catalogInput);
  const artifacts = buildCognitionSchemaArtifacts(catalog);
  const checks = verifyCognitionSchemaArtifacts(artifacts, {
    "pack-bundle.schema.json": { $id: "https://yunque.local/schemas/cognisdk/pack-bundle.json" },
    "pack-bundle-apply-checklist-summary.schema.json": { $id: "stale" },
  });
  assertEqual(checks[0]?.match, true);
  assertEqual(checks[1]?.match, false);
  assertEqual(checks[1]?.error, "schema id mismatch");
});

test("rejects unknown schema names", () => {
  assert(isCognitionSchemaName("pack-bundle"));
  assert(!isCognitionSchemaName("unknown"));
  try {
    normalizeCognitionSchemaCatalog([{ name: "unknown", title: "Unknown", schema: "x" }]);
    throw new Error("expected catalog normalization to reject");
  } catch (error) {
    assert(error instanceof Error);
    assert(error.message.includes("unknown schema name"));
  }
});

test("keeps normalized catalog assignable to public types", () => {
  const catalog: CognitionSchemaCatalogEntry[] = normalizeCognitionSchemaCatalog(catalogInput);
  assertEqual(catalog[0]?.title, "Cognition SDK Pack Bundle");
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
