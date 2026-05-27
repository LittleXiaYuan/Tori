import fs from "fs";
import path from "path";
import { fileURLToPath } from "url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(__dirname, "..");

const checks = [
  {
    label: "Baseline CI workflow",
    path: ".github/workflows/ci.yml",
    includes: [
      "name: ci",
      "dorny/paths-filter@v3",
      "Go backend",
      "go vet ./...",
      "go build ./...",
      "go test -count=1",
      "go test -race",
      "golangci/golangci-lint-action@v6",
      "TypeScript SDK",
      "npm run test",
      "npm run typecheck",
      "npm run check:sdk-manifests",
      "Frontend (apps/web)",
      "npm run build",
      "Vitest unit tests",
      "Desktop (Tauri / Rust)",
      "cargo check --locked",
      "Docker build + health smoke test",
      "docker/build-push-action@v6",
      "/healthz/cognitive",
    ],
  },
  {
    label: "Security workflow",
    path: ".github/workflows/security.yml",
    includes: [
      "name: security",
      "schedule:",
      "govulncheck ./...",
      "npm audit --omit=dev --audit-level=high",
      "npm audit --audit-level=high",
      "gitleaks/gitleaks-action@v2",
    ],
  },
  {
    label: "Desktop release workflow",
    path: ".github/workflows/desktop-release.yml",
    includes: [
      "name: desktop-release",
      "workflow_dispatch",
      "tags:",
      "windows-latest",
      "macos-13",
      "ubuntu-latest",
      "go build -ldflags",
      "npm run build",
      "actions/upload-artifact@v4",
      "apps/desktop/src-tauri/target/release/bundle/**/*",
    ],
  },
  {
    label: "CI/CD audit doc",
    path: "docs/CI-CD-COVERAGE.md",
    includes: [
      "CI/CD 覆盖矩阵",
      "ci.yml",
      "security.yml",
      "desktop-release.yml",
      "缺口",
      "node scripts/check-ci-coverage.mjs",
    ],
  },
];

let failed = false;
for (const check of checks) {
  const abs = path.join(ROOT, check.path);
  if (!fs.existsSync(abs) || !fs.statSync(abs).isFile()) {
    console.error(`CI coverage check failed: missing ${check.label}: ${check.path}`);
    failed = true;
    continue;
  }
  const text = fs.readFileSync(abs, "utf8");
  for (const token of check.includes) {
    if (!text.includes(token)) {
      console.error(`CI coverage check failed: ${check.path} missing token ${JSON.stringify(token)}`);
      failed = true;
    }
  }
}

if (failed) process.exit(1);

const matrix = [
  ["ci.yml", "go", "gofmt, go vet, go build, go test, race subset, coverage summary"],
  ["ci.yml", "go-lint", "golangci-lint"],
  ["ci.yml", "sdk-typescript", "OpenAPI generation, incremental SDK tests, typecheck, manifest guard, package size guard"],
  ["ci.yml", "web", "npm ci, TypeScript typecheck, Next build, optional Vitest"],
  ["ci.yml", "desktop", "Linux Tauri deps plus cargo check --locked"],
  ["ci.yml", "docker-smoke", "Slim image build, size warning, /livez /readyz /healthz /healthz/cognitive smoke"],
  ["security.yml", "govulncheck", "Go vulnerability scan"],
  ["security.yml", "npm-audit", "apps/web production high-severity npm audit"],
  ["security.yml", "sdk-npm-audit", "packages/yunque-client high-severity npm audit"],
  ["security.yml", "secret-scan", "gitleaks full-history secret scan"],
  ["desktop-release.yml", "package", "manual/tagged Windows/macOS/Linux Tauri installer artifacts"],
];

const gaps = [
  "No dedicated performance regression CI gate for scripts/perf-baseline.ps1 or cmd/perf-baseline.",
  "Docs site build is not part of baseline CI yet, although docs/package.json has a VitePress build script.",
  "License compliance is documented and locally checked, but not wired into CI yet.",
  "Desktop release builds on tag/manual only; baseline CI does cargo check but does not upload installers.",
  "Security workflow covers Go and selected npm trees, but not Rust cargo advisory/license checks yet.",
];

console.log("CI/CD coverage check ok");
console.log("coverage matrix:");
for (const [workflow, job, coverage] of matrix) console.log(`- ${workflow} :: ${job} -> ${coverage}`);
console.log("known gaps:");
gaps.forEach((gap) => console.log(`- ${gap}`));
