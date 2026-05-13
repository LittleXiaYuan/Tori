#!/usr/bin/env node
import { existsSync, mkdirSync, writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";

const repoRoot = resolve(import.meta.dirname, "..");
const args = process.argv.slice(2);

function usage() {
  console.error("Usage: node scripts/scaffold-pack.mjs <slug> [--name <display-name>] [--route /v1/<slug>/ping] [--sdk yunque-client/<slug>]");
  process.exit(1);
}

function argValue(flag) {
  const index = args.indexOf(flag);
  return index >= 0 ? args[index + 1] : undefined;
}

const slug = args[0];
if (!slug || slug.startsWith("--")) usage();
if (!/^[a-z0-9][a-z0-9-]*$/.test(slug)) {
  console.error("Pack slug must use lowercase letters, numbers, and hyphens, and must start with a letter or number.");
  process.exit(1);
}

const pascal = slug.split("-").map((part) => part.charAt(0).toUpperCase() + part.slice(1)).join("");
const name = argValue("--name") ?? `${pascal} Pack`;
const route = argValue("--route") ?? `/v1/${slug}/ping`;
const sdk = argValue("--sdk") ?? `yunque-client/${slug}`;
if (!route.startsWith("/")) {
  console.error("--route must start with /");
  process.exit(1);
}

const packDir = resolve(repoRoot, "packs/examples", slug);
const handlerDir = resolve(repoRoot, "internal/packs", slug.replaceAll("-", ""));
const pageDir = resolve(repoRoot, "heroui-web/src/app/packs", slug);
for (const dir of [packDir, handlerDir, pageDir]) {
  if (existsSync(dir)) {
    console.error(`Refusing to overwrite existing path: ${dir}`);
    process.exit(1);
  }
}

const packID = `yunque.pack.${slug}`;
const menuKey = slug;
const pagePath = `/packs/${slug}`;
const component = `${slug}/${pascal}PackPage`;
const capability = `${slug.replaceAll("-", ".")}.ping`;
const permission = `${slug}:read`;

const manifest = {
  id: packID,
  name,
  version: "0.1.0",
  description: `${name} optional capability pack scaffolded for Pack Runtime.`,
  requiresCore: ">=0.1.0",
  optional: true,
  defaultState: "disabled",
  backend: {
    capabilities: [capability],
    routes: [route],
    permissions: [permission],
  },
  frontend: {
    menus: [{ key: menuKey, label: name, path: pagePath, icon: "package", order: 120 }],
    routes: [{ path: pagePath, component, title: name }],
    assets: { type: "builtin", entry: component },
  },
  sdk: { typescript: sdk },
  update: { channel: "stable", rollback: true },
  metadata: { scaffold: "scripts/scaffold-pack.mjs", sync: "backend-registry-drives-frontend" },
};

const handler = `package ${slug.replaceAll("-", "")}

import (
\t"net/http"

\t"yunque-agent/pkg/packruntime"
)

const PackID = "${packID}"

type Handler struct{}

func New() *Handler { return &Handler{} }

func DefaultHandler() *Handler { return New() }

func (h *Handler) PackID() string { return PackID }

func (h *Handler) Routes() []packruntime.BackendRoute {
\treturn []packruntime.BackendRoute{
\t\t{Method: http.MethodGet, Path: "${route}", Handler: h.Ping},
\t}
}

func (h *Handler) Ping(w http.ResponseWriter, r *http.Request) {
\tw.Header().Set("Content-Type", "application/json")
\t_, _ = w.Write([]byte(` + "`" + `{"ok":true,"pack_id":"${packID}"}` + "`" + `))
}
`;

const page = `"use client";

import { Card } from "@heroui/react";

export default function ${pascal}PackPage() {
  return (
    <div className="page-root space-y-4 animate-fade-in-up">
      <Card className="section-card p-5">
        <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>${name}</div>
        <div className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>
          This page is synchronized from ${packID}. Replace it with the pack-specific UI.
        </div>
      </Card>
    </div>
  );
}
`;

const readme = `# ${name}

Scaffolded Pack Runtime capability pack.

- Pack ID: \`${packID}\`
- Backend route: \`${route}\`
- Frontend route: \`${pagePath}\`
- TypeScript SDK: \`${sdk}\`

Next steps:

1. Wire \`internal/packs/${slug.replaceAll("-", "")}\` through \`GatewayConfig.BackendPacks\` or \`RegisterBackendPack\`.
2. Replace the ping handler with real pack logic.
3. Replace the frontend page with the pack UI.
4. Add a focused SDK slice if \`${sdk}\` does not exist yet.
5. Run \`node scripts/check-pack-contract.mjs\`.
`;

for (const dir of [packDir, handlerDir, pageDir]) mkdirSync(dir, { recursive: true });
writeFileSync(resolve(packDir, "pack.json"), `${JSON.stringify(manifest, null, 2)}\n`, "utf8");
writeFileSync(resolve(packDir, "README.md"), readme, "utf8");
writeFileSync(resolve(handlerDir, "handler.go"), handler, "utf8");
writeFileSync(resolve(pageDir, "page.tsx"), page, "utf8");

console.log(`Pack scaffold created: ${slug}`);
console.log(`- ${packDir}`);
console.log(`- ${handlerDir}`);
console.log(`- ${pageDir}`);
