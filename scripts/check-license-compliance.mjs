import fs from "fs";
import path from "path";
import { fileURLToPath } from "url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(__dirname, "..");

const npmLockfiles = [
  "apps/web/package-lock.json",
  "packages/yunque-client/package-lock.json",
  "apps/desktop/package-lock.json",
  "docs/package-lock.json",
  "yunque-mcp/package-lock.json",
];

const cargoLocks = [
  "apps/desktop/src-tauri/Cargo.lock",
  "sdk/rust/Cargo.lock",
];

const allowedFamilies = [
  "0BSD",
  "Apache",
  "Apache-2.0",
  "BSD",
  "BSD-2-Clause",
  "BSD-3-Clause",
  "CC-BY-4.0",
  "CC0-1.0",
  "ISC",
  "MIT",
  "MPL-2.0",
  "Unicode-3.0",
  "Unlicense",
  "Zlib",
];

const denyPatterns = [
  /\bAGPL\b/i,
  /\bGPL\b/i,
  /\bSSPL\b/i,
  /Commons Clause/i,
  /BUSL/i,
  /PolyForm/i,
  /Elastic License/i,
];

function fail(message) {
  console.error(`license compliance check failed: ${message}`);
  process.exitCode = 1;
}

function readJSON(rel) {
  return JSON.parse(fs.readFileSync(path.join(ROOT, rel), "utf8"));
}

function isDeniedLicense(license) {
  return Boolean(license && typeof license === "string" && denyPatterns.some((pattern) => pattern.test(license)));
}

function isAllowedLicense(license) {
  if (!license || typeof license !== "string") return false;
  if (isDeniedLicense(license)) return false;
  return allowedFamilies.some((name) => license.includes(name));
}

function npmPackages(lockRel) {
  const lock = readJSON(lockRel);
  const results = [];
  for (const [pkgPath, meta] of Object.entries(lock.packages ?? {})) {
    if (!pkgPath || !pkgPath.includes("node_modules")) continue;
    const name = meta.name ?? pkgPath.split("node_modules/").pop()?.replaceAll("/node_modules/", "/") ?? pkgPath;
    results.push({ name, version: meta.version ?? "", license: meta.license ?? "UNKNOWN", source: lockRel });
  }
  return results;
}

function parseCargoLock(lockRel) {
  const text = fs.readFileSync(path.join(ROOT, lockRel), "utf8");
  const packages = [];
  for (const block of text.split(/\n\[\[package\]\]\n/)) {
    if (!block.includes("name =")) continue;
    const name = block.match(/name = "([^"]+)"/)?.[1];
    const version = block.match(/version = "([^"]+)"/)?.[1];
    const source = block.match(/source = "([^"]+)"/)?.[1] ?? "local";
    if (name && source.includes("crates.io")) packages.push({ name, version, source: lockRel });
  }
  return packages;
}

function findCargoLicense(pkg) {
  const registryRoot = path.join(process.env.USERPROFILE || "", ".cargo", "registry", "src");
  const candidates = [];
  if (fs.existsSync(registryRoot)) {
    for (const indexDir of fs.readdirSync(registryRoot)) {
      const dir = path.join(registryRoot, indexDir, `${pkg.name}-${pkg.version}`);
      if (fs.existsSync(dir)) candidates.push(dir);
    }
  }
  for (const dir of candidates) {
    const manifest = path.join(dir, "Cargo.toml");
    if (!fs.existsSync(manifest)) continue;
    const text = fs.readFileSync(manifest, "utf8");
    const license = text.match(/^license\s*=\s*"([^"]+)"/m)?.[1];
    if (license) return license;
    const licenseFile = text.match(/^license-file\s*=\s*"([^"]+)"/m)?.[1];
    if (licenseFile) return `license-file:${licenseFile}`;
  }
  return "UNKNOWN";
}

function encodeGoModulePath(modulePath) {
  return modulePath.replace(/[A-Z]/g, (char) => `!${char.toLowerCase()}`);
}

function parseGoModules() {
  const modText = fs.readFileSync(path.join(ROOT, "go.mod"), "utf8");
  const modules = [];
  let inRequireBlock = false;
  for (const rawLine of modText.split(/\r?\n/)) {
    let line = rawLine.trim();
    if (line.startsWith("require (")) {
      inRequireBlock = true;
      continue;
    }
    if (inRequireBlock && line === ")") {
      inRequireBlock = false;
      continue;
    }
    if (!inRequireBlock && !line.startsWith("require ")) continue;
    line = line.replace(/^require\s+/, "").replace(/\/\/.*$/, "").trim();
    if (!line) continue;
    const [modulePath, version] = line.split(/\s+/);
    if (!modulePath || !version) continue;
    const cacheRoot = path.join(process.env.USERPROFILE || "", "go", "pkg", "mod");
    const dir = path.join(cacheRoot, `${encodeGoModulePath(modulePath)}@${version}`);
    modules.push({ path: modulePath, version, dir });
  }
  return modules;
}

function findGoLicense(mod) {
  if (!mod.dir || !fs.existsSync(mod.dir)) return "UNKNOWN";
  const files = fs.readdirSync(mod.dir).filter((name) => /^(license|licence|copying|notice)(\.|$)/i.test(name));
  const text = files
    .map((name) => {
      try {
        return fs.readFileSync(path.join(mod.dir, name), "utf8").slice(0, 30000);
      } catch {
        return "";
      }
    })
    .join("\n")
    .toLowerCase();
  if (text.includes("apache license") || text.includes("apache-2.0")) return "Apache-2.0";
  if (text.includes("mit license") || text.includes("permission is hereby granted")) return "MIT";
  if (text.includes("mozilla public license")) return "MPL-2.0";
  if (text.includes("isc license")) return "ISC";
  if (text.includes("bsd 3-clause") || text.includes("redistribution and use in source and binary forms")) return "BSD-style";
  if (text.includes("gnu lesser general public license")) return "LGPL";
  if (text.includes("gnu general public license")) return "GPL";
  return files.length > 0 ? "UNKNOWN_WITH_LICENSE_FILE" : "UNKNOWN";
}

const summary = {
  npm: { lockfiles: 0, packages: 0, unknown: [], denied: [] },
  cargo: { lockfiles: 0, packages: 0, unknown: [], denied: [] },
  go: { modules: 0, unknown: [], denied: [] },
};

for (const lockRel of npmLockfiles) {
  const abs = path.join(ROOT, lockRel);
  if (!fs.existsSync(abs)) {
    fail(`missing npm lockfile ${lockRel}`);
    continue;
  }
  summary.npm.lockfiles++;
  for (const pkg of npmPackages(lockRel)) {
    summary.npm.packages++;
    if (pkg.name === "yunque-client" && pkg.source === "apps/web/package-lock.json") continue;
    if (pkg.license === "UNKNOWN") summary.npm.unknown.push(pkg);
    if (isDeniedLicense(pkg.license)) summary.npm.denied.push(pkg);
  }
}

for (const lockRel of cargoLocks) {
  const abs = path.join(ROOT, lockRel);
  if (!fs.existsSync(abs)) {
    fail(`missing Cargo.lock ${lockRel}`);
    continue;
  }
  summary.cargo.lockfiles++;
  for (const pkg of parseCargoLock(lockRel)) {
    const license = findCargoLicense(pkg);
    summary.cargo.packages++;
    const record = { ...pkg, license };
    if (license === "UNKNOWN" || license.startsWith("license-file:")) summary.cargo.unknown.push(record);
    if (isDeniedLicense(license)) summary.cargo.denied.push(record);
  }
}

const goModules = parseGoModules();
summary.go.modules = goModules.length;
for (const mod of goModules) {
  const license = findGoLicense(mod);
  const record = { ...mod, license };
  if (license.startsWith("UNKNOWN")) summary.go.unknown.push(record);
  if (isDeniedLicense(license)) summary.go.denied.push(record);
}

if (summary.npm.unknown.length > 0) fail(`npm packages missing license metadata: ${summary.npm.unknown.map((p) => `${p.name}@${p.version}`).slice(0, 10).join(", ")}`);
if (summary.npm.denied.length > 0) fail(`npm packages have disallowed/unknown licenses: ${summary.npm.denied.map((p) => `${p.name}@${p.version}:${p.license}`).slice(0, 10).join(", ")}`);
if (summary.cargo.denied.length > 0) fail(`cargo crates need review: ${summary.cargo.denied.map((p) => `${p.name}@${p.version}:${p.license}`).slice(0, 10).join(", ")}`);
if (summary.go.denied.length > 0) fail(`go modules have disallowed licenses: ${summary.go.denied.map((p) => `${p.path}@${p.version}:${p.license}`).slice(0, 10).join(", ")}`);

if (summary.cargo.unknown.length > 0) {
  console.warn(`license compliance warning: cargo crates needing manual review (${summary.cargo.unknown.length}): ${summary.cargo.unknown.slice(0, 12).map((p) => `${p.name}@${p.version}:${p.license}`).join(", ")}`);
}
if (summary.go.unknown.length > 0) {
  console.warn(`license compliance warning: go modules needing manual review (${summary.go.unknown.length}): ${summary.go.unknown.slice(0, 12).map((p) => `${p.path}@${p.version}:${p.license}`).join(", ")}`);
}

if (process.exitCode) process.exit(process.exitCode);
console.log("license compliance check ok");
console.log(JSON.stringify(summary, null, 2));
