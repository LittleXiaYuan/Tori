import {
  findCognitionSdkCapability,
  listCognitionSdkPackageEntrypoints,
  normalizeCognitionSdkPackageManifest,
  summarizeCognitionSdkPackageManifest,
  type CognitionSdkPackageManifest,
} from "./cognisdk-package";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }

const manifestInput = {
  version: 1,
  domain: "cognisdk-package",
  description: "Portable Cognition SDK package manifest.",
  principles: ["Keep pack flows rollback-friendly."],
  goPackage: { importPath: "yunque-agent/pkg/cognisdk", implementationFiles: ["pkg/cognisdk/pack.go", "pkg/cognisdk/schema.go"] },
  capabilities: [
    { name: "packBundle", symbols: ["PackBundle", "ExportBundle"] },
    { name: "jsonSchemas", symbols: ["JSONSchemaInfos"] },
  ],
  cli: [
    { name: "cognisdk-schema", file: "cmd/cognisdk-schema/main.go", commands: ["list", "export"], flags: ["--json"], schemas: ["pack-bundle"] },
    { name: "cognisdk-bundle", file: "cmd/cognisdk-bundle/main.go", commands: ["digest"], flags: ["--input"] },
  ],
  tests: ["pkg/cognisdk/pack_bundle_test.go"],
  docs: ["docs/guide/cognisdk-feedback.md"],
} satisfies unknown;

test("normalizes the package manifest for external callers", () => {
  const manifest = normalizeCognitionSdkPackageManifest(manifestInput);
  assertEqual(manifest.domain, "cognisdk-package");
  assertEqual(manifest.goPackage.importPath, "yunque-agent/pkg/cognisdk");
  assertEqual(manifest.cli[0]?.commands.length, 2);
});

test("summarizes SDK package coverage for dashboards and release gates", () => {
  const summary = summarizeCognitionSdkPackageManifest(normalizeCognitionSdkPackageManifest(manifestInput));
  assertEqual(summary.capabilityCount, 2);
  assertEqual(summary.cliToolCount, 2);
  assertEqual(summary.commandCount, 3);
  assertEqual(summary.schemaCount, 1);
  assertEqual(summary.testCount, 1);
});

test("lists rollback-friendly source, CLI, test, and doc entrypoints", () => {
  const entrypoints = listCognitionSdkPackageEntrypoints(normalizeCognitionSdkPackageManifest(manifestInput));
  assert(entrypoints.some((entry) => entry.kind === "go" && entry.path === "pkg/cognisdk/pack.go"));
  assert(entrypoints.some((entry) => entry.kind === "cli" && entry.name === "cognisdk-schema"));
  assert(entrypoints.some((entry) => entry.kind === "test"));
  assert(entrypoints.some((entry) => entry.kind === "doc"));
});

test("finds named capabilities without importing the full SDK", () => {
  const manifest: CognitionSdkPackageManifest = normalizeCognitionSdkPackageManifest(manifestInput);
  assertEqual(findCognitionSdkCapability(manifest, "jsonSchemas")?.symbols[0], "JSONSchemaInfos");
  assertEqual(findCognitionSdkCapability(manifest, "missing"), undefined);
});

test("rejects stale or unrelated package manifests", () => {
  try {
    normalizeCognitionSdkPackageManifest({ ...manifestInput, domain: "other" });
    throw new Error("expected invalid domain to reject");
  } catch (error) {
    assert(error instanceof Error);
    assert(error.message.includes("domain must be cognisdk-package"));
  }
  try {
    normalizeCognitionSdkPackageManifest({ ...manifestInput, goPackage: { importPath: "other/pkg", implementationFiles: [] } });
    throw new Error("expected invalid goPackage to reject");
  } catch (error) {
    assert(error instanceof Error);
    assert(error.message.includes("pkg/cognisdk"));
  }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
