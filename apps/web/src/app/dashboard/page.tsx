"use client";

import { useEffect, useState, useCallback, useMemo } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Button } from "@heroui/react";
import { api, type VersionInfo } from "@/lib/api";
import {
  MessageCircle, ArrowRight, Rocket, Zap,
  BookOpen, Brain, CheckCircle2, XCircle, Plus,
  Sparkles, Settings,
} from "lucide-react";
import { usePolling } from "@/lib/use-polling";
import { DashboardSkeleton } from "@/components/skeleton-loader";
import { DASHBOARD_SCENARIOS, scenarioChatHref } from "@/lib/product-scenarios";
import { createCogniKernelPackClient } from "@/lib/cogni-kernel-pack-client";
import type {
  CogniEntryStatus,
  CogniHealthMetrics,
} from "@/lib/api-types/cogni";

const cogniPack = createCogniKernelPackClient();

const COGNI_GRID_LIMIT = 7; // 7 cogni cards + 1 "create" tile = 8 cells (fits 4x2 grid cleanly)

type CogniState =
  | { kind: "loading" }
  | { kind: "unavailable" }
  | { kind: "ready"; entries: CogniEntryStatus[]; health: Record<string, CogniHealthMetrics> };

function statusDotColor(entry: CogniEntryStatus, health: CogniHealthMetrics | undefined): string {
  if (!entry.enabled) return "var(--yunque-text-muted)";
  if (entry.load_error) return "var(--yunque-danger)";
  switch (health?.status) {
    case "healthy": return "var(--yunque-success)";
    case "warn": return "var(--yunque-warning)";
    case "unhealthy": return "var(--yunque-danger)";
    case "idle":
    default:
      return "var(--yunque-text-muted)";
  }
}

function statusLabel(entry: CogniEntryStatus, health: CogniHealthMetrics | undefined): string {
  if (!entry.enabled) return "未启用";
  if (entry.load_error) return "加载失败";
  if (!health) return "等待运行数据";
  const activations = health.activations || 0;
  const last = health.last_seen_at ? timeAgo(health.last_seen_at) : "尚未激活";
  return `${activations} 次激活 · ${last}`;
}

function timeAgo(iso: string): string {
  const t = Date.parse(iso);
  if (Number.isNaN(t)) return "—";
  const seconds = Math.max(1, Math.round((Date.now() - t) / 1000));
  if (seconds < 60) return `${seconds}s 前`;
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return `${minutes}min 前`;
  const hours = Math.round(minutes / 60);
  if (hours < 48) return `${hours}h 前`;
  const days = Math.round(hours / 24);
  return `${days}d 前`;
}

export default function DashboardPage() {
  const router = useRouter();
  const [version, setVersion] = useState<VersionInfo | null>(null);
  const [serviceOnline, setServiceOnline] = useState(false);
  const [loading, setLoading] = useState(true);
  const [setupNeeded, setSetupNeeded] = useState(false);
  const [cogniState, setCogniState] = useState<CogniState>({ kind: "loading" });

  const load = useCallback(async () => {
    try {
      const health = await api.healthz();
      setServiceOnline(true);
      setVersion(prev => prev || {
        version: health.version || "",
        git_commit: "",
        build_date: "",
        go_version: "",
        os: "",
        arch: "",
      });
    } catch {
      setServiceOnline(false);
    }
    try {
      const v = await api.version().catch(() => null);
      if (v) setVersion(v);
    } catch { /* ignore */ }
    try {
      const chk = await api.checkSetup();
      setSetupNeeded(chk.setup_needed);
    } catch { /* ignore */ }
    // The /v1/cognis routes are gated by the cogni-kernel pack; absence is the
    // expected fallback, not an error — degrade to scenarios silently.
    try {
      const list = await cogniPack.list();
      setCogniState({
        kind: "ready",
        entries: list.cognis || [],
        health: list.health || {},
      });
    } catch {
      setCogniState({ kind: "unavailable" });
    }
    setLoading(false);
  }, []);

  useEffect(() => { load(); }, [load]);
  usePolling(load, 15000);

  const cogniSummary = useMemo(() => {
    if (cogniState.kind !== "ready") return null;
    const total = cogniState.entries.length;
    const enabled = cogniState.entries.filter(e => e.enabled).length;
    return { total, enabled };
  }, [cogniState]);

  if (loading) return <DashboardSkeleton />;

  return (
    <div className="page-root animate-fade-in-up">

      {/* ── Greeting ── */}
      <div style={{ textAlign: "center", padding: "var(--sp-8) 0 var(--sp-4)" }}>
        <h1 className="page-title" style={{ fontSize: "var(--text-2xl)", textAlign: "center" }}>
          {cogniSummary && cogniSummary.total > 0 ? "云雀正在为你工作" : "你好"}
        </h1>
        <p style={{
          fontSize: "var(--text-base)", color: "var(--yunque-text-muted)",
          marginTop: "var(--sp-3)", lineHeight: 1.6,
        }}>
          {cogniSummary && cogniSummary.total > 0
            ? `${cogniSummary.enabled} 个 Cogni 启用，共 ${cogniSummary.total} 个声明`
            : "说一句话，云雀帮你规划和执行。"}
          <span style={{ marginLeft: 8, display: "inline-flex", alignItems: "center", gap: 4 }}>
            {serviceOnline
              ? <><CheckCircle2 size={13} style={{ color: "var(--yunque-success)" }} /> <span style={{ fontSize: "var(--text-sm)", color: "var(--yunque-text-muted)" }}>服务在线{version ? ` · v${version.version}` : ""}</span></>
              : <><XCircle size={13} style={{ color: "var(--yunque-danger)" }} /> <span style={{ fontSize: "var(--text-sm)", color: "var(--yunque-text-muted)" }}>服务离线</span></>
            }
          </span>
        </p>
      </div>

      {/* ── Setup Banner ── */}
      {setupNeeded && (
        <div
          className="section-card"
          style={{
            borderLeft: "3px solid var(--yunque-warning)",
            background: "var(--yunque-warning-muted)",
            display: "flex", alignItems: "center", gap: "var(--sp-4)",
            padding: "var(--card-pad-sm) var(--card-pad)",
          }}
        >
          <Rocket size={20} style={{ color: "var(--yunque-warning)", flexShrink: 0 }} />
          <div style={{ flex: 1 }}>
            <div style={{ fontSize: "var(--text-md)", fontWeight: 600, color: "var(--yunque-text)" }}>
              请先配置大模型接入
            </div>
            <p style={{ fontSize: "var(--text-sm)", color: "var(--yunque-text-secondary)", marginTop: "var(--sp-1)" }}>
              配置后才能正常对话和执行任务。
            </p>
          </div>
          <Button size="sm" onPress={() => router.push("/setup")} style={{ background: "var(--yunque-warning)", color: "#000", fontWeight: 600 }}>
            去配置
          </Button>
        </div>
      )}

      {/* ── Cogni instances (or fallback) ── */}
      {cogniState.kind === "ready" && cogniState.entries.length > 0 ? (
        <CogniGridSection
          entries={cogniState.entries}
          health={cogniState.health}
          onOpen={(id) => router.push(`/cognis?id=${encodeURIComponent(id)}`)}
          onCreate={() => router.push("/cognis")}
        />
      ) : cogniState.kind === "ready" ? (
        <CogniEmptySection onCreate={() => router.push("/cognis")} />
      ) : (
        <ScenariosFallbackSection
          cogniPackAvailable={cogniState.kind !== "unavailable"}
          onScenario={(prompt) => router.push(scenarioChatHref(prompt))}
        />
      )}

      {/* ── Primary action ── */}
      <div style={{ display: "flex", justifyContent: "center", padding: "var(--sp-2) 0" }}>
        <Button
          size="lg"
          onPress={() => router.push("/chat")}
          style={{
            paddingInline: 32,
            fontSize: "var(--text-md)",
            fontWeight: 600,
            gap: 8,
          }}
        >
          <MessageCircle size={18} /> 开始对话
        </Button>
      </div>

      {/* ── Quick links ── */}
      <nav
        aria-label="快捷入口"
        style={{
          display: "flex", justifyContent: "center", gap: "var(--sp-3)",
          padding: "var(--sp-4) 0 var(--sp-8)",
          flexWrap: "wrap",
        }}
      >
        <Button variant="ghost" size="sm" onPress={() => router.push("/missions")} style={{ gap: 6 }}>
          <Zap size={14} /> 任务中心
        </Button>
        <Button variant="ghost" size="sm" onPress={() => router.push("/knowledge")} style={{ gap: 6 }}>
          <BookOpen size={14} /> 知识库
        </Button>
        <Button variant="ghost" size="sm" onPress={() => router.push("/memory")} style={{ gap: 6 }}>
          <Brain size={14} /> 记忆
        </Button>
        <Button variant="ghost" size="sm" onPress={() => window.dispatchEvent(new CustomEvent("yunque:open-settings"))} style={{ gap: 6 }}>
          <Settings size={14} /> 设置
        </Button>
      </nav>
    </div>
  );
}

interface CogniGridSectionProps {
  entries: CogniEntryStatus[];
  health: Record<string, CogniHealthMetrics>;
  onOpen: (id: string) => void;
  onCreate: () => void;
}

function CogniGridSection({ entries, health, onOpen, onCreate }: CogniGridSectionProps) {
  // Sort: enabled first, then by recent activation (healthy → idle), then by display name.
  const sorted = useMemo(() => {
    const arr = [...entries];
    arr.sort((a, b) => {
      if (a.enabled !== b.enabled) return a.enabled ? -1 : 1;
      const ha = health[a.id];
      const hb = health[b.id];
      const ta = ha?.last_seen_at ? Date.parse(ha.last_seen_at) : 0;
      const tb = hb?.last_seen_at ? Date.parse(hb.last_seen_at) : 0;
      if (ta !== tb) return tb - ta;
      return (a.display_name || a.id).localeCompare(b.display_name || b.id);
    });
    return arr;
  }, [entries, health]);

  const visible = sorted.slice(0, COGNI_GRID_LIMIT);
  const hidden = sorted.length - visible.length;

  return (
    <section aria-labelledby="dashboard-cogni-title" style={{ marginTop: "var(--sp-4)" }}>
      <div style={{
        display: "flex", alignItems: "baseline", justifyContent: "space-between",
        marginBottom: "var(--sp-3)", gap: "var(--sp-2)",
      }}>
        <h2
          id="dashboard-cogni-title"
          style={{
            fontSize: "var(--text-sm)", fontWeight: 600, color: "var(--yunque-text-muted)",
            textTransform: "uppercase", letterSpacing: "0.06em",
          }}
        >
          你的 Cogni
        </h2>
        <Link
          href="/cognis"
          style={{
            fontSize: "var(--text-sm)", color: "var(--yunque-text-muted)",
            display: "inline-flex", alignItems: "center", gap: 4,
          }}
        >
          全部 {hidden > 0 ? `(+${hidden})` : ""} <ArrowRight size={12} />
        </Link>
      </div>
      <ul
        className="dashboard-scenario-grid"
        style={{ gridTemplateColumns: "repeat(auto-fill, minmax(220px, 1fr))" }}
      >
        {visible.map((entry) => (
          <li key={entry.id}>
            <CogniCard
              entry={entry}
              health={health[entry.id]}
              onOpen={() => onOpen(entry.id)}
            />
          </li>
        ))}
        <li>
          <button
            type="button"
            onClick={onCreate}
            className="dashboard-scenario-card"
            style={{
              borderStyle: "dashed",
              alignItems: "center",
              justifyContent: "center",
              minHeight: 96,
            }}
          >
            <span className="dashboard-scenario-card__icon" aria-hidden>
              <Plus size={18} />
            </span>
            <span className="dashboard-scenario-card__body" style={{ textAlign: "left" }}>
              <span className="dashboard-scenario-card__title">新建 Cogni</span>
              <span className="dashboard-scenario-card__desc">用一句话描述助手要做什么</span>
            </span>
          </button>
        </li>
      </ul>
    </section>
  );
}

interface CogniCardProps {
  entry: CogniEntryStatus;
  health: CogniHealthMetrics | undefined;
  onOpen: () => void;
}

function CogniCard({ entry, health, onOpen }: CogniCardProps) {
  const dotColor = statusDotColor(entry, health);
  const meta = statusLabel(entry, health);
  const dim = !entry.enabled;
  return (
    <button
      type="button"
      onClick={onOpen}
      className="dashboard-scenario-card"
      style={{ opacity: dim ? 0.65 : 1, minHeight: 96 }}
      aria-label={`打开 ${entry.display_name || entry.id}`}
    >
      <span
        aria-hidden
        style={{
          display: "inline-flex",
          alignItems: "center",
          justifyContent: "center",
          width: 32, height: 32, flexShrink: 0,
          borderRadius: "var(--radius-md)",
          background: "var(--yunque-bg-subtle, rgba(255,255,255,0.04))",
          position: "relative",
        }}
      >
        <Sparkles size={14} style={{ color: "var(--yunque-text-muted)" }} />
        <span
          aria-hidden
          style={{
            position: "absolute", right: -2, top: -2,
            width: 8, height: 8, borderRadius: "50%",
            background: dotColor,
            boxShadow: "0 0 0 2px var(--yunque-card)",
          }}
        />
      </span>
      <span className="dashboard-scenario-card__body">
        <span
          className="dashboard-scenario-card__title"
          style={{ display: "block", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}
        >
          {entry.display_name || entry.id}
        </span>
        <span
          className="dashboard-scenario-card__desc"
          style={{
            display: "-webkit-box",
            WebkitLineClamp: 2,
            WebkitBoxOrient: "vertical",
            overflow: "hidden",
          }}
        >
          {entry.description || meta}
        </span>
        {entry.description && (
          <span
            style={{
              display: "block",
              marginTop: 4,
              fontSize: "var(--text-xs)",
              color: "var(--yunque-text-muted)",
            }}
          >
            {meta}
          </span>
        )}
      </span>
      <ArrowRight size={12} aria-hidden className="dashboard-scenario-card__arrow" />
    </button>
  );
}

function CogniEmptySection({ onCreate }: { onCreate: () => void }) {
  return (
    <section style={{ marginTop: "var(--sp-4)" }}>
      <div
        className="section-card"
        style={{
          display: "flex", flexDirection: "column", alignItems: "center",
          gap: "var(--sp-3)", padding: "var(--sp-6) var(--sp-4)", textAlign: "center",
        }}
      >
        <Sparkles size={28} style={{ color: "var(--yunque-text-muted)" }} />
        <div style={{ fontSize: "var(--text-md)", fontWeight: 600, color: "var(--yunque-text)" }}>
          你还没有 Cogni 实例
        </div>
        <p style={{ fontSize: "var(--text-sm)", color: "var(--yunque-text-muted)", maxWidth: 420 }}>
          Cogni 把"何时激活、注入什么上下文、暴露哪些工具"写成可验证的声明。
          用一句话描述目标，云雀会帮你生成第一个。
        </p>
        <Button onPress={onCreate} style={{ gap: 6 }}>
          <Plus size={14} /> 新建 Cogni
        </Button>
      </div>
    </section>
  );
}

interface ScenariosFallbackProps {
  cogniPackAvailable: boolean;
  onScenario: (prompt: string) => void;
}

function ScenariosFallbackSection({ cogniPackAvailable, onScenario }: ScenariosFallbackProps) {
  return (
    <section aria-labelledby="dashboard-scenarios-title" style={{ marginTop: "var(--sp-4)" }}>
      <h2
        id="dashboard-scenarios-title"
        style={{
          fontSize: "var(--text-sm)", fontWeight: 600, color: "var(--yunque-text-muted)",
          textTransform: "uppercase", letterSpacing: "0.06em",
          marginBottom: "var(--sp-4)", textAlign: "center",
        }}
      >
        常用场景
      </h2>
      <ul className="dashboard-scenario-grid" style={{ gridTemplateColumns: "repeat(2, minmax(0, 1fr))" }}>
        {DASHBOARD_SCENARIOS.map((a) => (
          <li key={a.id}>
            <button
              type="button"
              onClick={() => onScenario(a.prompt)}
              className="dashboard-scenario-card"
            >
              <span className="dashboard-scenario-card__icon" aria-hidden>{a.icon}</span>
              <span className="dashboard-scenario-card__body">
                <span className="dashboard-scenario-card__title">{a.label}</span>
                <span className="dashboard-scenario-card__desc">{a.description}</span>
              </span>
              <ArrowRight size={12} aria-hidden className="dashboard-scenario-card__arrow" />
            </button>
          </li>
        ))}
      </ul>
      {!cogniPackAvailable && (
        <p style={{
          marginTop: "var(--sp-4)", textAlign: "center",
          fontSize: "var(--text-xs)", color: "var(--yunque-text-muted)",
        }}>
          想让云雀按声明运行而不是单次回答？<Link href="/packs?focus=cogni-kernel" style={{ color: "var(--yunque-accent)" }}>启用 Cogni 内核 →</Link>
        </p>
      )}
    </section>
  );
}
