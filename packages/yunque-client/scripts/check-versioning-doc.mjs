import { readFileSync } from "node:fs";
import { join, resolve } from "node:path";

const repoRoot = resolve(process.cwd(), "../..");
const docsPath = join(repoRoot, "docs", "SDK-VERSIONING.md");
const readmePath = join(process.cwd(), "README.md");
const docs = readFileSync(docsPath, "utf8");
const readme = readFileSync(readmePath, "utf8");

const requiredDocs = [
  "# SDK Versioning and Compatibility",
  "## Release levels",
  "| Patch |",
  "| Minor |",
  "| Breaking |",
  "## Stability tiers",
  "## Required checks before release",
  "npm run check:incremental --prefix packages/yunque-client",
  "npm run typecheck:test --prefix packages/yunque-client",
  "go test ./internal/agentcore/... ./internal/cognikernel ./internal/experimental/reflect ./internal/controlplane/... -count=1",
  "## Compatibility checklist for SDK changes",
  "## Deprecation policy",
  "BREAKING:",
];
const failures = [];
for (const token of requiredDocs) {
  if (!docs.includes(token)) failures.push(`docs/SDK-VERSIONING.md missing ${token}`);
}
if (!readme.includes("../../docs/SDK-VERSIONING.md") && !readme.includes("docs/SDK-VERSIONING.md")) {
  failures.push("packages/yunque-client/README.md must link docs/SDK-VERSIONING.md");
}
if (!readme.includes("patch / minor / breaking")) {
  failures.push("packages/yunque-client/README.md must summarize patch / minor / breaking policy");
}

if (failures.length > 0) {
  console.error(`SDK versioning doc check failed (${failures.length} issues):`);
  for (const item of failures) console.error(`- ${item}`);
  process.exit(1);
}
console.log("SDK versioning policy ok: docs/SDK-VERSIONING.md is linked and covers release levels, stability tiers, checks, deprecation, and breaking-change notes");
