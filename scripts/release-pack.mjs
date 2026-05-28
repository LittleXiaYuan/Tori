#!/usr/bin/env node
// release-pack.mjs — package + sign + upload one Pack to a GitHub Release.
//
// Inputs (env or flags):
//   --pack <dir>            Pack source directory (must contain pack.json)
//   --tag <tag>             Git tag (created if missing, e.g. pack/inner-life/v0.1.0)
//   --key <key.key>         ed25519 private key (base64) used to sign pack.json
//   --publisher <id>        Publisher id, recorded in manifest + signature
//   --key-id <kid>          Public key id, used by trust root to look up the public bytes
//   --dry-run               Skip the gh release upload step
//
// What it does:
//   1. Run `yunque-plugin sign` against the pack dir.
//   2. Run `yunque-plugin pack`  to produce <id>-<version>.yqpack.
//   3. Update distribution.sha256 / sizeBytes in pack.json to match the artifact.
//   4. Re-sign (signing block depends on the rest of pack.json).
//   5. Re-pack so the artifact reflects the final manifest.
//   6. `gh release create` (idempotent) and `gh release upload` the .yqpack.
//
// The script does NOT mutate publisher metadata beyond what was passed in,
// and refuses to run if the working tree contains uncommitted changes inside
// the pack dir — releases must be reproducible from the committed source.
import { spawnSync } from "node:child_process";
import { existsSync, readFileSync, writeFileSync, statSync, mkdtempSync, mkdirSync, readdirSync, copyFileSync, rmSync } from "node:fs";
import { join, resolve, basename, dirname } from "node:path";
import { tmpdir } from "node:os";

const argv = parseArgs(process.argv.slice(2));
const repoRoot = resolve(import.meta.dirname, "..");

const dryRun = argv["dry-run"] === true;
const packArg = argv["pack"];
const tag = argv["tag"];
const keyPath = argv["key"];
const publisher = argv["publisher"];
const keyID = argv["key-id"];

if (!packArg) usageExit("--pack is required");
if (!tag) usageExit("--tag is required (e.g. pack/inner-life/v0.1.0)");

const packDir = resolve(packArg);
const manifestPath = join(packDir, "pack.json");
if (!existsSync(manifestPath)) {
  fatal(`no pack.json in ${packDir}`);
}

const signing = Boolean(keyPath || publisher || keyID);
if (signing && (!keyPath || !publisher || !keyID)) {
  fatal("when signing, --key, --publisher, and --key-id must all be provided");
}

const cleanCheck = run("git", ["status", "--porcelain", packDir], { cwd: repoRoot, capture: true });
if (cleanCheck.stdout.trim() !== "") {
  fatal(`pack dir has uncommitted changes:\n${cleanCheck.stdout}`);
}

const stagingRoot = mkdtempSync(join(tmpdir(), "yunque-release-"));
const stagingPack = join(stagingRoot, basename(packDir));
cpDir(packDir, stagingPack);

try {
  if (signing) {
    log(`signing ${stagingPack}`);
    pluginRun(["sign", stagingPack, "--key", keyPath, "--publisher", publisher, "--key-id", keyID]);
  }

  log(`packing ${stagingPack} (round 1)`);
  let { sha, size, outPath } = packAndCapture(stagingPack);

  // Round 2: ensure pack.json's distribution.sha256 / sizeBytes match the
  // artifact we just built. If they were stale, update + re-sign + re-pack so
  // the published manifest is self-consistent.
  const manifest = readJSON(join(stagingPack, "pack.json"));
  manifest.distribution = manifest.distribution || {};
  const needRefresh =
    manifest.distribution.sha256 !== sha ||
    Number(manifest.distribution.sizeBytes || 0) !== size;
  if (needRefresh) {
    log(`distribution stale: sha256/sizeBytes mismatch, refreshing manifest`);
    manifest.distribution.sha256 = sha;
    manifest.distribution.sizeBytes = size;
    writeFileSync(join(stagingPack, "pack.json"), JSON.stringify(manifest, null, 2) + "\n");
    if (signing) {
      pluginRun(["sign", stagingPack, "--key", keyPath, "--publisher", publisher, "--key-id", keyID]);
    }
    ({ sha, size, outPath } = packAndCapture(stagingPack));
  }

  log(`final artifact: ${outPath}`);
  log(`  sha256 = ${sha}`);
  log(`  size   = ${size}`);

  if (dryRun) {
    log("dry-run: skipping gh release create/upload");
    log(`artifact retained at: ${outPath}`);
  } else {
    log(`gh release create (if missing): ${tag}`);
    const create = run("gh", ["release", "view", tag], { cwd: repoRoot, capture: true, allowFail: true });
    if (create.code !== 0) {
      run("gh", ["release", "create", tag, "--notes", `Pack release ${tag}`], { cwd: repoRoot });
    } else {
      log(`release ${tag} already exists — uploading asset to it`);
    }

    log(`gh release upload: ${outPath}`);
    run("gh", ["release", "upload", tag, outPath, "--clobber"], { cwd: repoRoot });

    log(`✓ release published: ${tag}`);
    log(`  remember to point distribution.packageUrl in pack.json at:`);
    log(`    https://github.com/<owner>/<repo>/releases/download/${tag}/${basename(outPath)}`);
  }
} finally {
  rmSync(stagingRoot, { recursive: true, force: true });
}

function packAndCapture(dir) {
  const result = pluginRun(["pack", dir]);
  // Parse stdout to find the produced .yqpack path. The pack subcommand
  // prints "Packing X → Y" on the first line and "  ✓ Y (N bytes, sha256 H)"
  // on the third.
  const lines = result.stdout.split(/\r?\n/);
  const arrowLine = lines.find((l) => l.includes("→"));
  if (!arrowLine) fatal(`could not parse pack output:\n${result.stdout}`);
  const outPath = arrowLine.split("→")[1].trim();
  const okLine = lines.find((l) => l.startsWith("  ✓"));
  if (!okLine) fatal(`pack did not produce a success line:\n${result.stdout}`);
  const m = okLine.match(/(\d+)\s+bytes,\s+sha256\s+([0-9a-f]{64})/);
  if (!m) fatal(`could not parse sha/size from: ${okLine}`);
  return { sha: m[2], size: Number(m[1]), outPath: resolve(repoRoot, outPath) };
}

function pluginRun(args) {
  return run("go", ["run", "./cmd/yunque-plugin", ...args], { cwd: repoRoot, capture: true });
}

function run(cmd, args, opts = {}) {
  const { cwd = repoRoot, capture = false, allowFail = false } = opts;
  const res = spawnSync(cmd, args, {
    cwd,
    stdio: capture ? ["ignore", "pipe", "pipe"] : "inherit",
    encoding: "utf8",
  });
  if (res.error) fatal(`${cmd}: ${res.error.message}`);
  if (res.status !== 0 && !allowFail) {
    if (capture) {
      process.stderr.write(res.stdout || "");
      process.stderr.write(res.stderr || "");
    }
    fatal(`${cmd} ${args.join(" ")} exited ${res.status}`);
  }
  return { code: res.status, stdout: res.stdout || "", stderr: res.stderr || "" };
}

function readJSON(path) {
  return JSON.parse(readFileSync(path, "utf8"));
}

function cpDir(src, dst) {
  // Manual recursive copy. Node's cpSync crashes (STATUS_STACK_BUFFER_OVERRUN)
  // on Windows when the source path contains non-ASCII characters — see
  // https://github.com/nodejs/node/issues/55956. We only need plain dir+file
  // copies for pack staging.
  mkdirSync(dst, { recursive: true });
  for (const entry of readdirSync(src, { withFileTypes: true })) {
    const s = join(src, entry.name);
    const d = join(dst, entry.name);
    if (entry.isDirectory()) {
      cpDir(s, d);
    } else if (entry.isFile()) {
      copyFileSync(s, d);
    }
    // skip symlinks/sockets — packs shouldn't contain them.
  }
}

function parseArgs(args) {
  const out = {};
  for (let i = 0; i < args.length; i++) {
    const a = args[i];
    if (!a.startsWith("--")) continue;
    const key = a.slice(2);
    const next = args[i + 1];
    if (next === undefined || next.startsWith("--")) {
      out[key] = true;
    } else {
      out[key] = next;
      i++;
    }
  }
  return out;
}

function log(msg) {
  console.log(`[release-pack] ${msg}`);
}

function fatal(msg) {
  console.error(`[release-pack] error: ${msg}`);
  process.exit(1);
}

function usageExit(msg) {
  console.error(`[release-pack] ${msg}\n`);
  console.error(
    "usage: node scripts/release-pack.mjs --pack <dir> --tag <tag> [--key <key.key> --publisher <id> --key-id <kid>] [--dry-run]"
  );
  process.exit(2);
}
