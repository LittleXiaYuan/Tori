import { readFileSync, readdirSync, rmSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { spawnSync } from "node:child_process";

const outDir = ".tmp/incremental-tests";
const srcDir = "src";
const generated = new Set(["client.gen", "sdk.gen", "types.gen", "index"]);

function listIncrementalSlices() {
  return readdirSync(srcDir)
    .filter((name) => name.endsWith(".ts") && !name.endsWith(".test.ts"))
    .map((name) => name.replace(/\.ts$/, ""))
    .filter((name) => !generated.has(name))
    .sort();
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

rmSync(outDir, { recursive: true, force: true });

const slices = listIncrementalSlices();
const sources = slices.flatMap((name) => [`${srcDir}/${name}.ts`, `${srcDir}/${name}.test.ts`]);

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
  if (compile.error) console.error(compile.error);
  process.exit(compile.status ?? 1);
}

for (const testName of slices.map((name) => `${name}.test`)) {
  const compiledTestPath = join(outDir, `${testName}.js`);
  const compiledTest = addJsExtensionToRelativeImports(readFileSync(compiledTestPath, "utf8"));
  writeFileSync(compiledTestPath, compiledTest);

  const run = spawnSync(process.execPath, [compiledTestPath], { stdio: "inherit" });
  if (run.error || run.status !== 0) {
    rmSync(outDir, { recursive: true, force: true });
    if (run.error) console.error(run.error);
    process.exit(run.status ?? 1);
  }
}

rmSync(outDir, { recursive: true, force: true });
