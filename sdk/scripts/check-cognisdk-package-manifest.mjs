import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

const repoRoot = resolve(import.meta.dirname, "../..");
const manifestPath = resolve(repoRoot, "sdk/manifest/cognisdk-package.json");
const manifest = JSON.parse(readFileSync(manifestPath, "utf8"));
const failures = [];
function fail(message) { failures.push(message); }
function readRepoFile(path) {
  const fullPath = resolve(repoRoot, path);
  if (!existsSync(fullPath)) {
    fail(`missing file: ${path}`);
    return "";
  }
  return readFileSync(fullPath, "utf8");
}

const packageSource = (manifest.goPackage?.implementationFiles ?? []).map(readRepoFile).join("\n");
if (!manifest.goPackage?.importPath?.includes("pkg/cognisdk")) fail("goPackage importPath must point at pkg/cognisdk");

for (const capability of manifest.capabilities ?? []) {
  if (!capability.name) fail("capability missing name");
  for (const symbol of capability.symbols ?? []) {
    const alternatives = [
      symbol,
      `func ${symbol}`,
      `type ${symbol}`,
      `${symbol}(`,
      `${symbol} `,
    ];
    if (!alternatives.some((candidate) => packageSource.includes(candidate))) {
      fail(`missing cognisdk symbol for ${capability.name}: ${symbol}`);
    }
  }
}

for (const cli of manifest.cli ?? []) {
  const text = readRepoFile(cli.file);
  for (const command of cli.commands ?? []) {
    if (!text.includes(`"${command}"`)) fail(`${cli.name} missing command ${command}`);
  }
  for (const flag of cli.flags ?? []) {
    if (!text.includes(`"${flag}"`) && !text.includes(flag)) fail(`${cli.name} missing flag ${flag}`);
  }
  if (cli.schemas?.length) {
    const schemaSource = readRepoFile("pkg/cognisdk/schema.go");
    for (const schemaName of cli.schemas) {
      if (!schemaSource.includes(`"${schemaName}"`)) fail(`${cli.name} schema name not exposed: ${schemaName}`);
    }
  }
}

for (const test of manifest.tests ?? []) {
  const text = readRepoFile(test);
  if (!/func Test|func Example/.test(text)) fail(`test file has no tests/examples: ${test}`);
}

for (const doc of manifest.docs ?? []) {
  const text = readRepoFile(doc);
  if (!/cognisdk|Cogni|PackBundle|bundle|反馈|增量包/i.test(text)) fail(`doc does not describe cognisdk package surface: ${doc}`);
}

if (failures.length) {
  console.error("Cognition SDK package manifest check failed:");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}
const flagCount = (manifest.cli ?? []).reduce((sum, cli) => sum + (cli.flags?.length ?? 0), 0);
console.log(`Cognition SDK package manifest ok: ${(manifest.capabilities ?? []).length} capabilities, ${(manifest.cli ?? []).length} cli tools, ${flagCount} cli flags`);
