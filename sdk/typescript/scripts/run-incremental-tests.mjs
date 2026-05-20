import { readFileSync, readdirSync, rmSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { spawnSync } from "node:child_process";

const outDir = `.tmp/incremental-tests-${process.pid}`;
const srcDir = "src";
const generated = new Set(["client.gen", "sdk.gen", "types.gen", "index"]);

function listIncrementalSlices() {
  return readdirSync(srcDir)
    .filter((name) => name.endsWith(".ts") && !name.endsWith(".test.ts"))
    .map((name) => name.replace(/\.ts$/, ""))
    .filter((name) => !generated.has(name))
    .sort();
}

function normalizeRequestedSlices(args) {
  return args
    .filter((arg) => arg !== "--")
    .flatMap((arg) => arg.split(","))
    .map((arg) => arg.trim())
    .filter(Boolean)
    .map((arg) => arg.replace(/^yunque-client\//, "").replace(/^src\//, "").replace(/\.test\.ts$|\.ts$|\.test$/g, ""));
}

function addJsExtensionToRelativeImports(source) {
  return source.replace(
    /(from\s+["'])(\.\/[^"']+)(["'])/g,
    (match, prefix, specifier, suffix) => {
      if (specifier.endsWith(".js") || /\.[cm]?js$/.test(specifier)) return match;
      return `${prefix}${specifier}.js${suffix}`;
    },
  );
}

const allSlices = listIncrementalSlices();
const requestedSlices = normalizeRequestedSlices(process.argv.slice(2));
const missingSlices = requestedSlices.filter((name) => !allSlices.includes(name));

if (missingSlices.length > 0) {
  console.error(`unknown incremental slice(s): ${missingSlices.join(", ")}`);
  console.error(`available slices include: ${allSlices.slice(0, 20).join(", ")}${allSlices.length > 20 ? ", ..." : ""}`);
  process.exit(1);
}

const slices = requestedSlices.length > 0 ? requestedSlices : allSlices;
const sources = slices.flatMap((name) => [`${srcDir}/${name}.ts`, `${srcDir}/${name}.test.ts`]);

function cleanup() {
  rmSync(outDir, { recursive: true, force: true });
}

cleanup();

const compile = spawnSync(
  process.execPath,
  [
    "node_modules/typescript/bin/tsc",
    "--module",
    "ES2022",
    "--moduleResolution",
    "Bundler",
    "--target",
    "ES2022",
    "--lib",
    "ES2022,DOM,DOM.Iterable",
    "--noEmit",
    "false",
    "--outDir",
    outDir,
    ...sources,
  ],
  { stdio: "inherit" },
);

if (compile.error || compile.status !== 0) {
  cleanup();
  if (compile.error) console.error(compile.error);
  process.exit(compile.status ?? 1);
}

for (const testName of slices.map((name) => `${name}.test`)) {
  const compiledTestPath = join(outDir, `${testName}.js`);
  const compiledTest = addJsExtensionToRelativeImports(readFileSync(compiledTestPath, "utf8"));
  writeFileSync(compiledTestPath, compiledTest);

  const run = spawnSync(process.execPath, [compiledTestPath], { stdio: "inherit" });
  if (run.error || run.status !== 0) {
    cleanup();
    if (run.error) console.error(run.error);
    process.exit(run.status ?? 1);
  }
}

cleanup();
