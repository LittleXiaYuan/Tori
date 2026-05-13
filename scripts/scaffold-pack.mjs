#!/usr/bin/env node
import { existsSync, mkdirSync, writeFileSync } from "node:fs";
import { relative, resolve } from "node:path";

const repoRoot = resolve(import.meta.dirname, "..");
const args = process.argv.slice(2);

function usage() {
  console.error("Usage: node scripts/scaffold-pack.mjs <slug> [--name <display-name>] [--route /v1/<slug>/ping] [--sdk yunque-client/<slug>] [--dry-run] [--json]");
  process.exit(1);
}

function argValue(flag) {
  const index = args.indexOf(flag);
  return index >= 0 ? args[index + 1] : undefined;
}

const dryRun = args.includes("--dry-run");
const jsonOutput = args.includes("--json");

const slug = args[0];
if (!slug || slug.startsWith("--")) usage();
if (!/^[a-z0-9][a-z0-9-]*$/.test(slug)) {
  console.error("Pack slug must use lowercase letters, numbers, and hyphens, and must start with a letter or number.");
  process.exit(1);
}

const pascal = slug.split("-").map((part) => part.charAt(0).toUpperCase() + part.slice(1)).join("");
const name = argValue("--name") ?? `${pascal} Pack`;
const route = argValue("--route") ?? `/v1/${slug}/ping`;
const routeMethod = (argValue("--method") ?? "GET").trim().toUpperCase();
const sdk = argValue("--sdk") ?? `yunque-client/${slug}`;
const manifestUrl = argValue("--manifest-url") ?? `https://packs.yunque.local/${slug}/pack.json`;
const packageUrl = argValue("--package-url") ?? `https://packs.yunque.local/${slug}/${slug}-0.1.0.tgz`;
const frontendUrl = argValue("--frontend-url") ?? `https://packs.yunque.local/${slug}/frontend/remoteEntry.js`;
const sha256 = argValue("--sha256") ?? "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef";
if (!route.startsWith("/")) {
  console.error("--route must start with /");
  process.exit(1);
}
if (!routeMethod) {
  console.error("--method must not be empty");
  process.exit(1);
}

const packDir = resolve(repoRoot, "packs/examples", slug);
const handlerDir = resolve(repoRoot, "internal/packs", slug.replaceAll("-", ""));
const pageDir = resolve(repoRoot, "heroui-web/src/app/packs", slug);
const frontendClient = resolve(repoRoot, "heroui-web/src/lib", `${slug}-pack-client.ts`);
const frontendClientTest = resolve(repoRoot, "heroui-web/src/lib/__tests__", `${slug}-pack-client.test.ts`);
for (const dir of [packDir, handlerDir, pageDir]) {
  if (existsSync(dir)) {
    console.error(`Refusing to overwrite existing path: ${dir}`);
    process.exit(1);
  }
}
for (const file of [frontendClient, frontendClientTest]) {
  if (existsSync(file)) {
    console.error(`Refusing to overwrite existing path: ${file}`);
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
    routeSpecs: [{ method: routeMethod, path: route, description: `${name} backend entrypoint.` }],
    permissions: [permission],
  },
  frontend: {
    menus: [{ key: menuKey, label: name, path: pagePath, icon: "package", order: 120 }],
    routes: [{ path: pagePath, component, title: name }],
    assets: { type: "builtin", entry: component },
  },
  sdk: { typescript: sdk },
  distribution: {
    manifestUrl,
    packageUrl,
    frontendUrl,
    sha256,
    sizeBytes: 4096,
  },
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
\t\t{Method: "${routeMethod}", Path: "${route}", Handler: h.Ping},
\t}
}

func (h *Handler) Ping(w http.ResponseWriter, r *http.Request) {
\tw.Header().Set("Content-Type", "application/json")
\t_, _ = w.Write([]byte(` + "`" + `{"ok":true,"pack_id":"${packID}"}` + "`" + `))
}
`;

const page = `"use client";

import { useEffect, useState } from "react";
import { Card } from "@heroui/react";
import { create${pascal}PackClient } from "@/lib/${slug}-pack-client";

const ${slug.replace(/-([a-z0-9])/g, (_, char) => char.toUpperCase())}Pack = create${pascal}PackClient();

export default function ${pascal}PackPage() {
  const [status, setStatus] = useState("checking");

  useEffect(() => {
    let alive = true;
    ${slug.replace(/-([a-z0-9])/g, (_, char) => char.toUpperCase())}Pack.ping()
      .then((res) => {
        if (alive) setStatus(res.ok ? "ready" : "unhealthy");
      })
      .catch(() => {
        if (alive) setStatus("unavailable");
      });
    return () => {
      alive = false;
    };
  }, []);

  return (
    <div className="page-root space-y-4 animate-fade-in-up">
      <Card className="section-card p-5">
        <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>${name}</div>
        <div className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>
          This page is synchronized from ${packID}. Replace it with the pack-specific UI.
        </div>
        <div className="text-xs mt-3 font-mono" style={{ color: "var(--yunque-text-muted)" }}>
          SDK/client entry: create${pascal}PackClient() → ${route}
        </div>
        <div className="text-xs mt-2" style={{ color: "var(--yunque-text-muted)" }}>
          Backend route status: {status}
        </div>
      </Card>
    </div>
  );
}
`;

const client = `import { fetcher } from "./api-core";

export interface ${pascal}PingResponse {
  ok: boolean;
  pack_id: string;
}

export interface ${pascal}PackClient {
  ping(): Promise<${pascal}PingResponse>;
}

export function create${pascal}PackClient(): ${pascal}PackClient {
  return {
    ping: () => fetcher<${pascal}PingResponse>("${route}"),
  };
}
`;

const clientTest = `import { afterEach, describe, expect, it, vi } from "vitest";
import { create${pascal}PackClient } from "../${slug}-pack-client";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("${slug}-pack-client", () => {
  it("calls the pack-owned backend route", async () => {
    const fetchSpy = vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      new Response(JSON.stringify({ ok: true, pack_id: "${packID}" }), { status: 200 }),
    );

    const result = await create${pascal}PackClient().ping();

    expect(fetchSpy.mock.calls[0]?.[0]).toBe("${route}");
    expect(result.pack_id).toBe("${packID}");
  });
});
`;

const readme = `# ${name}

Scaffolded Pack Runtime capability pack.

- Pack ID: \`${packID}\`
- Backend route: \`${routeMethod} ${route}\`
- Frontend route: \`${pagePath}\`
- TypeScript SDK: \`${sdk}\`
- Manifest URL: \`${manifestUrl}\`
- Package URL: \`${packageUrl}\`
- Frontend URL: \`${frontendUrl}\`

Next steps:

1. Wire \`internal/packs/${slug.replaceAll("-", "")}\` through \`GatewayConfig.BackendPacks\` or \`RegisterBackendPack\`.
2. Replace the ping handler with real pack logic.
3. Replace the frontend page with the pack UI.
4. Extend \`heroui-web/src/lib/${slug}-pack-client.ts\` instead of adding methods to the monolithic frontend \`api\` object.
5. Add a focused SDK slice if \`${sdk}\` does not exist yet.
6. Run \`node scripts/check-pack-contract.mjs\` and \`npm run test --prefix heroui-web -- src/lib/__tests__/${slug}-pack-client.test.ts\`.
`;

const files = [
  { path: resolve(packDir, "pack.json"), content: `${JSON.stringify(manifest, null, 2)}\n` },
  { path: resolve(packDir, "README.md"), content: readme },
  { path: resolve(handlerDir, "handler.go"), content: handler },
  { path: resolve(pageDir, "page.tsx"), content: page },
  { path: frontendClient, content: client },
  { path: frontendClientTest, content: clientTest },
];
const directories = [packDir, handlerDir, pageDir];
const result = {
  slug,
  packId: packID,
  dryRun,
  manifest,
  directories: directories.map((path) => relative(repoRoot, path).replaceAll("\\", "/")),
  files: files.map((file) => relative(repoRoot, file.path).replaceAll("\\", "/")),
};

if (!dryRun) {
  for (const dir of directories) mkdirSync(dir, { recursive: true });
  for (const file of files) writeFileSync(file.path, file.content, "utf8");
}

if (jsonOutput) {
  console.log(JSON.stringify(result, null, 2));
} else if (dryRun) {
  console.log(`Pack scaffold dry run: ${slug}`);
  for (const file of result.files) console.log(`- ${file}`);
} else {
  console.log(`Pack scaffold created: ${slug}`);
  for (const dir of result.directories) console.log(`- ${dir}`);
}
