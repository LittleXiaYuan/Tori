"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { Button, Card, Chip, Input, Label, Spinner, TextArea, TextField } from "@heroui/react";
import { ArrowRight, Boxes, Copy, FileSearch, RefreshCw, ShieldCheck, Sparkles, Wrench } from "lucide-react";
import PageHeader from "@/components/page-header";
import { showToast } from "@/components/toast-provider";
import { createYunqueSDKClientOptions } from "@/lib/sdk-client";
import { useApiData } from "@/lib/use-api-data";
import {
  capabilitySurfaceLabels,
  groupPackPermissions,
  packFeatureFlags,
  packUsability,
  riskProfileForPack,
} from "@/lib/pack-presentation";
import { createPacksClient, type PackManifest } from "yunque-client/packs";

const packsClient = createPacksClient(createYunqueSDKClientOptions());

type PackCandidate = {
  manifest: PackManifest;
  source: "installed" | "catalog";
  enabled: boolean;
  installed: boolean;
};

function packPaths(manifest: PackManifest): string[] {
  return Array.from(new Set([
    ...(manifest.frontend?.menus || []).map((menu) => menu.path),
    ...(manifest.frontend?.routes || []).map((route) => route.path),
  ].filter(Boolean))).sort();
}

function packRoutes(manifest: PackManifest): string[] {
  const specs = manifest.backend?.routeSpecs || [];
  if (specs.length > 0) return specs.map((route) => `${route.method} ${route.path}`);
  return manifest.backend?.routes || [];
}

function sourceLabel(candidate: PackCandidate): string {
  if (candidate.installed && candidate.enabled) return "已启用";
  if (candidate.installed) return "已安装";
  return "源内可安装";
}

function buildStudioAnalysis(manifest: PackManifest): {
  editable: string[];
  guarded: string[];
  tests: string[];
  warnings: string[];
} {
  const flags = packFeatureFlags(manifest);
  const risk = riskProfileForPack(manifest);
  const usability = packUsability(manifest);
  const routes = packRoutes(manifest);
  const paths = packPaths(manifest);
  const permissions = manifest.backend?.permissions || [];
  const capabilities = manifest.backend?.capabilities || [];

  const editable = [
    "用途说明、起手示例、入口文案、可用度分层和权限解释可以从 manifest/前端展示层优化。",
  ];
  if (paths.length > 0) editable.push("已有前端入口，可优先改页面文案、交互提示、空态、结果区和任务入口。");
  if (flags.isIframeBundle) editable.push("这是独立界面包；若 yqpack 内含 iframe 静态资源，可在沙箱边界内优化界面。");
  if (flags.hasWasm) editable.push("WASM 能力可以扩展 host 调用说明、输入输出 schema 和审计提示；改二进制逻辑需要源码。");
  if (capabilities.length > 0) editable.push("能力声明可用于生成更清楚的 Cogni/Planner 使用说明，但第一阶段不改决策算法。");

  const guarded = [
    "不直接修改已签名或已安装包；先生成 diff 方案，用户确认后再打包为新 yqpack。",
    "不扩大权限、不新增高风险 route，除非用户明确授权并更新权限说明。",
  ];
  if (routes.length > 0) guarded.push("后端路由逻辑属于运行时能力，改行为需要对应源码和 Go/Pack 测试。");
  if (flags.hasWasm) guarded.push("不要反编译后硬改 WASM；需要源码、ABI 说明和 wasm-plugin 回归测试。");
  if (flags.isIframeBundle) guarded.push("iframe 仍无宿主 token，只能调用 manifest 声明的 route。");
  if (risk.level === "high") guarded.push("高风险能力必须保留授权说明、审计线索和可回滚路径。");

  const tests = [
    "npm run typecheck",
    "npm test -- packs-page.test.tsx",
    "node scripts\\check-pack-usability.mjs --strict",
  ];
  if (routes.length > 0) tests.push("go test ./pkg/packruntime ./internal/controlplane/gateway ./internal/packs/... ./cmd/agent -count=1");
  if (flags.hasWasm) tests.push("go test ./internal/packs/wasmplugin ./internal/controlplane/gateway -run WASM -count=1");

  const warnings = [];
  if (usability.kind === "infrastructure") warnings.push("这个包主要是基础能力，改造目标应落到 Chat/任务/知识等实际使用面。");
  if (usability.kind === "experimental") warnings.push("这个包仍是实验能力，改造时不要把它包装成稳定承诺。");
  if (permissions.length === 0) warnings.push("manifest 未声明权限，若新增能力必须先补权限与风险说明。");

  return { editable, guarded, tests, warnings };
}

function buildXiaoyuPrompt(manifest: PackManifest, goal: string): string {
  const routes = packRoutes(manifest);
  const paths = packPaths(manifest);
  const flags = packFeatureFlags(manifest);
  const permissions = manifest.backend?.permissions || [];
  const capabilities = manifest.backend?.capabilities || [];
  const analysis = buildStudioAnalysis(manifest);
  return [
    `请以“小羽改包”的方式改造能力包 ${manifest.name} (${manifest.id}) v${manifest.version}。`,
    "",
    `用户目标：${goal.trim() || "让这个能力包变得更有用、更可感知，并补齐用户能理解的入口、说明和结果反馈。"}`,
    "",
    "当前包信息：",
    `- 状态：${manifest.status || "unknown"}`,
    `- 前端入口：${paths.length > 0 ? paths.join(", ") : "无"}`,
    `- 后端路由：${routes.length > 0 ? routes.join(", ") : "无"}`,
    `- 能力声明：${capabilities.length > 0 ? capabilities.join(", ") : "无"}`,
    `- 权限声明：${permissions.length > 0 ? permissions.join(", ") : "无"}`,
    `- 形态：${flags.isIframeBundle ? "iframe-bundle " : ""}${flags.hasWasm ? "WASM " : ""}${flags.hasBackend ? "backend " : ""}${flags.hasFrontend ? "frontend" : ""}`.trim(),
    "",
    "请按这个安全流程执行：",
    "1. 先只读检查 yqpack/pack.json/前端入口/SDK/后端 routeSpecs，列出真实可改文件。",
    "2. 明确哪些能直接改，哪些需要源码，哪些属于已编译 WASM/native Go 不能硬改。",
    "3. 先给 diff 预览和风险说明，不要直接扩大权限或绕过签名。",
    "4. 用户确认后再修改、跑测试、重新打包为新的 yqpack，并保留旧版本回滚。",
    "",
    "本包建议优先改：",
    ...analysis.editable.map((item) => `- ${item}`),
    "",
    "必须遵守：",
    ...analysis.guarded.map((item) => `- ${item}`),
    "",
    "建议门禁：",
    ...analysis.tests.map((item) => `- ${item}`),
  ].join("\n");
}

export default function PackStudioPage() {
  const { data, loading, refresh } = useApiData(async () => {
    const [installed, catalog] = await Promise.all([packsClient.installed(), packsClient.catalog()]);
    const map = new Map<string, PackCandidate>();
    for (const pack of installed.packs || []) {
      map.set(pack.manifest.id, {
        manifest: pack.manifest,
        source: "installed",
        enabled: pack.status === "enabled",
        installed: true,
      });
    }
    for (const entry of catalog.entries || []) {
      if (!map.has(entry.manifest.id)) {
        map.set(entry.manifest.id, {
          manifest: entry.manifest,
          source: "catalog",
          enabled: Boolean(entry.enabled),
          installed: Boolean(entry.installed),
        });
      }
    }
    return { packs: [...map.values()].sort((a, b) => a.manifest.name.localeCompare(b.manifest.name)) };
  }, { packs: [] as PackCandidate[] });
  const [selectedId, setSelectedId] = useState("");
  const [goal, setGoal] = useState("让这个能力包更像一个用户能直接理解和使用的功能，而不是只看到存在。");

  const candidates = data?.packs || [];
  const selected = useMemo(
    () => candidates.find((item) => item.manifest.id === selectedId) || candidates[0],
    [candidates, selectedId],
  );
  const manifest = selected?.manifest;
  const analysis = manifest ? buildStudioAnalysis(manifest) : null;
  const prompt = manifest ? buildXiaoyuPrompt(manifest, goal) : "";
  const chatHref = `/chat?q=${encodeURIComponent(prompt)}`;

  const copyPrompt = async () => {
    if (!prompt) return;
    await navigator.clipboard?.writeText(prompt);
    showToast("已复制小羽改包任务", "success");
  };

  if (loading) {
    return <div className="flex items-center justify-center h-[60vh]"><Spinner size="lg" /></div>;
  }

  return (
    <div className="flex flex-col h-screen overflow-hidden">
      <div className="flex-shrink-0 p-5 border-b" style={{ borderColor: "var(--yunque-border)" }}>
        <PageHeader
          icon={<Wrench size={20} />}
          title="Pack Studio"
          description="让小羽先读懂能力包，再生成可审计、可测试、可回滚的改包方案"
          onRefresh={refresh}
        />
        <div className="mt-4 grid gap-3 md:grid-cols-3">
          <Card className="section-card p-4">
            <div className="kpi-label">可分析</div>
            <div className="kpi-value">{candidates.length}</div>
          </Card>
          <Card className="section-card p-4">
            <div className="kpi-label">执行模式</div>
            <div className="kpi-value text-base">只读规划</div>
          </Card>
          <Card className="section-card p-4">
            <div className="kpi-label">交付物</div>
            <div className="kpi-value text-base">diff 方案</div>
          </Card>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto p-5 space-y-4">
        <Card className="section-card p-4">
          <div className="flex items-center gap-2 mb-3">
            <FileSearch size={16} style={{ color: "var(--yunque-accent)" }} />
            <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>选择要分析的能力包</div>
          </div>
          {candidates.length === 0 ? (
            <div className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>还没有可分析的能力包。</div>
          ) : (
            <div className="grid gap-2 md:grid-cols-2 xl:grid-cols-3">
              {candidates.map((candidate) => {
                const usability = packUsability(candidate.manifest);
                const active = candidate.manifest.id === (manifest?.id || "");
                return (
                  <button
                    key={candidate.manifest.id}
                    type="button"
                    onClick={() => setSelectedId(candidate.manifest.id)}
                    className="rounded-md border p-3 text-left transition-colors"
                    style={{
                      borderColor: active ? "var(--yunque-accent)" : "var(--yunque-border)",
                      background: active ? "rgba(59,130,246,0.08)" : "var(--yunque-surface)",
                    }}
                  >
                    <div className="flex items-center justify-between gap-2">
                      <div className="truncate text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{candidate.manifest.name}</div>
                      <Chip size="sm" variant="soft">{sourceLabel(candidate)}</Chip>
                    </div>
                    <div className="mt-1 truncate text-xs" style={{ color: "var(--yunque-text-muted)" }}>{candidate.manifest.id}</div>
                    <div className="mt-2 flex flex-wrap gap-1">
                      <Chip size="sm" style={{ background: "rgba(59,130,246,0.08)", color: "var(--yunque-primary)" }}>{usability.label}</Chip>
                      <Chip size="sm" variant="soft">v{candidate.manifest.version}</Chip>
                    </div>
                  </button>
                );
              })}
            </div>
          )}
        </Card>

        {manifest && analysis && (
          <>
            <Card className="section-card p-4">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <div className="flex items-center gap-2 text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
                    <Boxes size={16} style={{ color: "var(--yunque-accent)" }} />
                    {manifest.name}
                  </div>
                  <div className="mt-1 text-xs font-mono" style={{ color: "var(--yunque-text-muted)" }}>{manifest.id}</div>
                </div>
                <div className="flex flex-wrap gap-1.5">
                  {capabilitySurfaceLabels(manifest).map((label) => (
                    <Chip key={label} size="sm" variant="soft">{label}</Chip>
                  ))}
                </div>
              </div>
              <div className="mt-3 grid gap-3 md:grid-cols-2">
                <div className="rounded-md border p-3" style={{ borderColor: "var(--yunque-border)" }}>
                  <div className="text-xs font-medium" style={{ color: "var(--yunque-text)" }}>可以让小羽优先优化</div>
                  <div className="mt-2 space-y-2">
                    {analysis.editable.map((item) => (
                      <div key={item} className="flex items-start gap-2 text-xs" style={{ color: "var(--yunque-text-secondary)" }}>
                        <span style={{ color: "var(--yunque-accent)" }}>•</span><span>{item}</span>
                      </div>
                    ))}
                  </div>
                </div>
                <div className="rounded-md border p-3" style={{ borderColor: "var(--yunque-border)" }}>
                  <div className="text-xs font-medium" style={{ color: "var(--yunque-text)" }}>需要守住的边界</div>
                  <div className="mt-2 space-y-2">
                    {analysis.guarded.map((item) => (
                      <div key={item} className="flex items-start gap-2 text-xs" style={{ color: "var(--yunque-text-secondary)" }}>
                        <ShieldCheck size={12} className="mt-0.5" style={{ color: "var(--yunque-success)" }} /><span>{item}</span>
                      </div>
                    ))}
                  </div>
                </div>
              </div>
              {analysis.warnings.length > 0 && (
                <div className="mt-3 rounded-md p-3 text-xs" style={{ background: "rgba(245,158,11,0.10)", color: "var(--yunque-warning)" }}>
                  {analysis.warnings.join(" ")}
                </div>
              )}
            </Card>

            <Card className="section-card p-4">
              <div className="flex items-center gap-2 mb-3">
                <Sparkles size={16} style={{ color: "var(--yunque-primary)" }} />
                <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>交给小羽的改包目标</div>
              </div>
              <TextField value={goal} onChange={setGoal}>
                <Label>这次想补强什么</Label>
                <Input placeholder="例如：增加结果查看入口、让能力更像一个可用工具、补齐权限说明" />
              </TextField>
              <div className="mt-3">
                <TextArea aria-label="小羽改包任务" value={prompt} readOnly rows={14} className="font-mono text-xs" />
              </div>
              <div className="mt-3 flex flex-wrap gap-2">
                <Button variant="outline" onPress={copyPrompt}>
                  <Copy size={14} /> 复制任务
                </Button>
                <Link href={chatHref}>
                  <Button className="btn-accent">
                    交给 Chat 里的小羽 <ArrowRight size={14} />
                  </Button>
                </Link>
                <Button variant="ghost" onPress={refresh}>
                  <RefreshCw size={14} /> 重新读取
                </Button>
              </div>
            </Card>

            <Card className="section-card p-4">
              <div className="text-sm font-semibold mb-3" style={{ color: "var(--yunque-text)" }}>建议门禁</div>
              <div className="space-y-1">
                {analysis.tests.map((command) => (
                  <div key={command} className="rounded px-2 py-1 font-mono text-xs" style={{ background: "var(--yunque-bg-hover)", color: "var(--yunque-text-secondary)" }}>
                    {command}
                  </div>
                ))}
              </div>
              <div className="mt-3 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                第一阶段只生成方案和任务，不直接改已安装包。真正写入、重新打包、安装 fork 版本和回滚会在后续闭环接入。
              </div>
            </Card>
          </>
        )}
      </div>
    </div>
  );
}
