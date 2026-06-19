#!/usr/bin/env node
// Build all official Pack manifests into deterministic .yqpack artifacts and
// generate a release catalog that can be uploaded to an OSS/GitHub source.
//
// Usage:
//   node scripts/build-official-yqpacks.mjs
//   node scripts/build-official-yqpacks.mjs --base-url https://oss.example.com/yunque/packs/official
//   node scripts/build-official-yqpacks.mjs --out-dir dist/packs/official --clean
import { createHash } from "node:crypto";
import { existsSync, mkdirSync, readFileSync, readdirSync, rmSync, statSync, writeFileSync } from "node:fs";
import { basename, dirname, join, relative, resolve, sep } from "node:path";
import { spawnSync } from "node:child_process";

const repoRoot = resolve(import.meta.dirname, "..");
const argv = parseArgs(process.argv.slice(2));
const officialDir = resolveArgPath(argv["source-dir"] || "packs/official");
const outDir = resolveArgPath(argv["out-dir"] || "dist/packs/official");
const catalogName = String(argv["catalog-name"] || "catalog.json");
const baseUrl = normalizeBaseUrl(String(argv["base-url"] || process.env.YUNQUE_PACK_BASE_URL || ""));
const clean = argv.clean === true;

if (!existsSync(officialDir)) fatal(`official pack dir not found: ${officialDir}`);
if (clean) rmSync(outDir, { recursive: true, force: true });
mkdirSync(outDir, { recursive: true });

const manifestPaths = findPackManifests(officialDir);
if (manifestPaths.length === 0) fatal(`no pack.json files found under ${officialDir}`);

const packs = manifestPaths
  .map((manifestPath) => {
    const manifest = readJson(manifestPath);
    requireString(manifest.id, `${manifestPath}: id`);
    requireString(manifest.version, `${manifestPath}: version`);
    return { manifestPath, packDir: dirname(manifestPath), manifest };
  })
  .sort((a, b) => a.manifest.id.localeCompare(b.manifest.id));

const entries = [];
for (const pack of packs) {
  const assetName = `${safeFileSegment(pack.manifest.id)}-${safeFileSegment(pack.manifest.version)}.yqpack`;
  const outPath = join(outDir, assetName);
  log(`packing ${pack.manifest.id}@${pack.manifest.version}`);
  run("go", ["run", "./cmd/yunque-plugin", "pack", pack.packDir, "--out", outPath]);
  const info = statSync(outPath);
  const sha256 = sha256File(outPath);
  const packageUrl = baseUrl ? `${baseUrl}/${encodeURIComponent(assetName)}` : `./${assetName}`;
  const manifestRel = slash(relative(repoRoot, pack.manifestPath));
  const artifactRel = slash(relative(repoRoot, outPath));
  const packDirRel = slash(relative(repoRoot, pack.packDir));
  const capabilities = [...(pack.manifest.backend?.capabilities || [])].filter(Boolean).sort();
  const routeSpecs = pack.manifest.backend?.routeSpecs || [];
  const frontendRoutes = pack.manifest.frontend?.routes || [];
  const frontendMenus = pack.manifest.frontend?.menus || [];
  const frontendAssets = pack.manifest.frontend?.assets || {};

  entries.push({
    id: pack.manifest.id,
    name: pack.manifest.name || pack.manifest.id,
    version: pack.manifest.version,
    description: pack.manifest.description || "",
    status: pack.manifest.status || "",
    default_state: pack.manifest.defaultState || "",
    defaultState: pack.manifest.defaultState || "",
    release_url: baseUrl || "",
    release_tag: `pack/${pack.manifest.id}/v${pack.manifest.version}`,
    package_url: packageUrl,
    packageUrl,
    asset_name: assetName,
    assetName,
    sha256,
    size_bytes: info.size,
    sizeBytes: info.size,
    manifest_path: manifestRel,
    manifestPath: manifestRel,
    artifact_path: artifactRel,
    artifactPath: artifactRel,
    pack_dir: packDirRel,
    packDir: packDirRel,
    capabilities,
    capability_count: capabilities.length,
    backend_route_count: routeSpecs.length || (pack.manifest.backend?.routes || []).length,
    frontend_route_count: frontendRoutes.length,
    frontend_menu_count: frontendMenus.length,
    frontend_assets_type: frontendAssets.type || "",
    frontend_assets_entry: frontendAssets.entry || "",
    has_frontend: frontendRoutes.length > 0 || frontendMenus.length > 0 || Boolean(frontendAssets.type),
    has_backend: capabilities.length > 0 || routeSpecs.length > 0 || (pack.manifest.backend?.routes || []).length > 0,
    has_wasm: pack.manifest.backend?.runtime?.type === "wasm" || Boolean(pack.manifest.backend?.runtime?.module),
    has_dlc: frontendAssets.type === "iframe-bundle",
    installed: false,
    enabled: false,
    update_action: "install",
    downloadable: true,
    manifest: pack.manifest,
  });
}

const catalog = {
  schema: "yunque.official-yqpack-catalog.v1",
  generated_at: new Date().toISOString(),
  source: slash(relative(repoRoot, officialDir)),
  out_dir: slash(relative(repoRoot, outDir)),
  base_url: baseUrl,
  count: entries.length,
  entries,
};

const catalogPath = join(outDir, catalogName);
writeFileSync(catalogPath, JSON.stringify(catalog, null, 2) + "\n");
log(`wrote ${slash(relative(repoRoot, catalogPath))}`);
log(`built ${entries.length} official yqpack artifact(s)`);

function findPackManifests(root) {
  const out = [];
  walk(root, (path, dirent) => {
    if (dirent.isFile() && dirent.name === "pack.json") out.push(path);
  });
  return out;
}

function walk(dir, visit) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) {
      walk(path, visit);
    } else {
      visit(path, entry);
    }
  }
}

function run(cmd, args) {
  const res = spawnSync(cmd, args, { cwd: repoRoot, stdio: "inherit", encoding: "utf8" });
  if (res.error) fatal(`${cmd}: ${res.error.message}`);
  if (res.status !== 0) fatal(`${cmd} ${args.join(" ")} exited ${res.status}`);
}

function sha256File(path) {
  return createHash("sha256").update(readFileSync(path)).digest("hex");
}

function readJson(path) {
  try {
    return JSON.parse(readFileSync(path, "utf8"));
  } catch (error) {
    fatal(`read ${path}: ${error.message}`);
  }
}

function requireString(value, label) {
  if (typeof value !== "string" || value.trim() === "") fatal(`${label} must be a non-empty string`);
}

function parseArgs(args) {
  const out = {};
  for (let i = 0; i < args.length; i++) {
    const arg = args[i];
    if (!arg.startsWith("--")) continue;
    const key = arg.slice(2);
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

function resolveArgPath(path) {
  return resolve(repoRoot, String(path));
}

function normalizeBaseUrl(value) {
  return value.trim().replace(/\/+$/, "");
}

function safeFileSegment(value) {
  return String(value).trim().replace(/[^a-zA-Z0-9._-]+/g, "-");
}

function slash(path) {
  return path.split(sep).join("/");
}

function log(message) {
  console.log(`[official-yqpack] ${message}`);
}

function fatal(message) {
  console.error(`[official-yqpack] error: ${message}`);
  process.exit(1);
}
