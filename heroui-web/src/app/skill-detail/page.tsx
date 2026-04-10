"use client";

import { Suspense, useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import { api, type SkillHubDetail, type AuditReport, type SkillUpdateInfo, type SkillVersionInfo } from "@/lib/api";
import { Card, Button, Chip } from "@heroui/react";
import {
  ArrowLeft, Star, Download, Shield, ShieldCheck, ShieldAlert,
  Tag, Clock, User, FileText, Package, ChevronDown, ChevronRight,
  Trash2, Lock, RefreshCw, History, ArrowDownToLine,
} from "lucide-react";
import Link from "next/link";

function ScoreBadge({ score }: { score: number }) {
  const color = score >= 80 ? "#17c964" : score >= 60 ? "#f5a524" : "#f31260";
  const Icon = score >= 80 ? ShieldCheck : score >= 60 ? Shield : ShieldAlert;
  return <span className="flex items-center gap-1 text-sm font-semibold" style={{ color }}><Icon size={16} /> {score}/100</span>;
}

function AuditSection({ report }: { report: AuditReport }) {
  const [expanded, setExpanded] = useState(false);
  const sevColors = ["#006fee", "#f5a524", "#f31260"];
  const sevLabels = ["Info", "Warning", "Critical"];
  return (
    <Card className="section-card p-4">
      <div className="flex items-center justify-between cursor-pointer" onClick={() => setExpanded(!expanded)}>
        <div className="flex items-center gap-2">
          <Shield size={16} style={{ color: "var(--yunque-accent)" }} />
          <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>安全审计报告</span>
        </div>
        <div className="flex items-center gap-3">
          <ScoreBadge score={report.score} />
          {expanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
        </div>
      </div>
      {expanded && (
        <div className="mt-4 space-y-3">
          <div className="grid grid-cols-3 gap-3">
            {[{ label: "静态分析", score: report.static_score, max: 40 }, { label: "权限审计", score: report.perm_score, max: 30 }, { label: "沙箱测试", score: report.sandbox_score, max: 30 }].map((item) => (
              <div key={item.label} className="rounded-lg p-3 text-center" style={{ background: "rgba(255,255,255,0.02)" }}>
                <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>{item.label}</div>
                <div className="text-lg font-bold mt-1" style={{ color: "var(--yunque-text)" }}>{item.score}/{item.max}</div>
              </div>
            ))}
          </div>
          {report.findings?.length > 0 && (
            <div className="space-y-1">
              <div className="text-xs font-medium mb-2" style={{ color: "var(--yunque-text-muted)" }}>发现 ({report.findings.length})</div>
              {report.findings.map((f, i) => (
                <div key={i} className="flex items-start gap-2 text-xs py-1.5 px-2 rounded" style={{ background: "rgba(255,255,255,0.02)" }}>
                  <span style={{ color: sevColors[f.severity] || "#9ca3af" }}>[{sevLabels[f.severity] || "?"}]</span>
                  <span style={{ color: "var(--yunque-text-muted)" }}><strong>{f.rule}:</strong> {f.detail}</span>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </Card>
  );
}

export default function SkillDetailPage() {
  return (
    <Suspense fallback={<div className="max-w-3xl mx-auto"><div className="section-card h-40 w-full rounded-xl animate-pulse" /></div>}>
      <SkillDetailContent />
    </Suspense>
  );
}

function SkillDetailContent() {
  const params = useSearchParams();
  const slug = params.get("slug") || "";
  const [detail, setDetail] = useState<SkillHubDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [installing, setInstalling] = useState(false);
  const [updateInfo, setUpdateInfo] = useState<SkillUpdateInfo | null>(null);
  const [versions, setVersions] = useState<SkillVersionInfo[]>([]);
  const [updating, setUpdating] = useState(false);
  const [rollingBack, setRollingBack] = useState<string | null>(null);
  const [showVersions, setShowVersions] = useState(false);

  useEffect(() => {
    if (!slug) return;
    api.skillHubDetail(slug).then(setDetail).catch(() => {}).finally(() => setLoading(false));
  }, [slug]);

  useEffect(() => {
    if (!detail?.installed || !slug) return;
    api.skillHubCheckUpdates().then((r) => { const info = r.updates?.find((u) => u.slug === slug); if (info) setUpdateInfo(info); }).catch(() => {});
    api.skillHubVersions(slug).then((r) => setVersions(r.versions || [])).catch(() => {});
  }, [detail?.installed, slug]);

  const doInstall = async () => { if (!slug) return; setInstalling(true); try { await api.skillHubInstall(slug); setDetail(await api.skillHubDetail(slug)); } catch {} setInstalling(false); };
  const doUpdate = async () => { if (!slug) return; setUpdating(true); try { await api.skillHubUpdate(slug); setDetail(await api.skillHubDetail(slug)); setUpdateInfo(null); api.skillHubVersions(slug).then((r) => setVersions(r.versions || [])).catch(() => {}); } catch {} setUpdating(false); };
  const doRollback = async (version: string) => { if (!slug) return; setRollingBack(version); try { await api.skillHubRollback(slug, version); setDetail(await api.skillHubDetail(slug)); api.skillHubVersions(slug).then((r) => setVersions(r.versions || [])).catch(() => {}); } catch {} setRollingBack(null); };
  const doUninstall = async () => { if (!slug) return; try { await api.skillHubUninstall(slug); setDetail(await api.skillHubDetail(slug)); } catch {} };

  if (loading) return (
    <div className="page-root space-y-4">
      {[1, 2, 3].map((i) => <div key={i} className="section-card h-24 rounded-xl animate-pulse" />)}
    </div>
  );

  if (!detail) return (
    <div className="page-root text-center py-20" style={{ color: "var(--yunque-text-muted)" }}>
      <Package size={48} className="mx-auto mb-3 opacity-30" />
      <p>技能未找到: {slug}</p>
      <Link href="/skills" className="text-xs mt-2 inline-block" style={{ color: "var(--yunque-accent)" }}>→返回市场</Link>
    </div>
  );

  return (
    <div className="page-root space-y-5 animate-fade-in-up">
      {/* Back + Header */}
      <div className="flex items-start justify-between gap-4">
        <div>
          <Link href="/skills" className="inline-flex items-center gap-1 text-xs mb-2" style={{ color: "var(--yunque-accent)" }}>
            <ArrowLeft size={14} /> 返回技能市场
          </Link>
          <h1 className="text-xl font-bold" style={{ color: "var(--yunque-text)" }}>{detail.name}</h1>
          <p className="text-sm mt-1" style={{ color: "var(--yunque-text-muted)" }}>{detail.description}</p>
        </div>
        <div className="flex items-center gap-2 shrink-0">
          {detail.installed ? (
            <>
              {updateInfo?.has_update && (
                <Button size="sm" onPress={doUpdate} isPending={updating} className="btn-accent">
                  <RefreshCw size={13} /> 更新到 v{updateInfo.latest_version}
                </Button>
              )}
              <Button size="sm" variant="outline" onPress={doUninstall}><Trash2 size={13} /> 卸载</Button>
            </>
          ) : (
            <Button size="sm" onPress={doInstall} isPending={installing} className="btn-accent">
              <Download size={13} /> 安装
            </Button>
          )}
        </div>
      </div>

      {/* Meta stats row */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3 stagger-children">
        {[
          { icon: <Tag size={14} />, label: "版本", value: detail.version || "—", color: "var(--yunque-accent)" },
          { icon: <User size={14} />, label: "作者", value: detail.author || "—", color: "#a78bfa" },
          { icon: <Star size={14} />, label: "评分", value: detail.rating > 0 ? `${detail.rating.toFixed(1)} *` : "—", color: "#f59e0b" },
          { icon: <Download size={14} />, label: "安装量", value: detail.installs > 0 ? String(detail.installs) : "—", color: "#22c55e" },
        ].map((m) => (
          <Card key={m.label} className="section-card p-4 hover-lift">
            <div className="flex items-center gap-1.5 text-xs mb-1" style={{ color: "var(--yunque-text-muted)" }}>{m.icon} {m.label}</div>
            <div className="text-base font-semibold" style={{ color: m.color }}>{m.value}</div>
          </Card>
        ))}
      </div>

      {/* Main 2-col layout */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-5">
        {/* Left: tags + permissions + SKILL.md */}
        <div className="lg:col-span-2 space-y-4">
          {/* Tags */}
          {(detail.category || (detail.tags && detail.tags.length > 0)) && (
            <Card className="section-card p-4">
              <div className="flex items-center gap-2 flex-wrap">
                {detail.category && <Chip size="sm" style={{ background: "rgba(0,111,238,0.1)", color: "var(--yunque-accent)" }}>{detail.category}</Chip>}
                {detail.tags?.map((tag) => <Chip key={tag} size="sm" style={{ background: "rgba(255,255,255,0.04)" }}>{tag}</Chip>)}
                {detail.source && <Chip size="sm" style={{ background: "rgba(255,255,255,0.04)" }}>来源: {detail.source}</Chip>}
                {detail.license && <Chip size="sm" style={{ background: "rgba(255,255,255,0.04)" }}> {detail.license}</Chip>}
              </div>
            </Card>
          )}

          {/* Permissions */}
          {detail.permissions && detail.permissions.length > 0 && (
            <Card className="section-card p-4">
              <div className="flex items-center gap-2 mb-3">
                <Lock size={16} style={{ color: "var(--yunque-accent)" }} />
                <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>权限要求</span>
              </div>
              <div className="flex flex-wrap gap-2">
                {detail.permissions.map((perm) => {
                  const isDangerous = ["shell", "network", "write"].some((w) => perm.toLowerCase().includes(w));
                  return (
                    <Chip key={perm} size="sm" style={{ background: isDangerous ? "rgba(243,18,96,0.1)" : "rgba(255,255,255,0.04)", color: isDangerous ? "#f31260" : "var(--yunque-text-muted)" }}>
                      {perm}
                    </Chip>
                  );
                })}
              </div>
            </Card>
          )}

          {/* SKILL.md Content */}
          {detail.content && (
            <Card className="section-card p-4">
              <div className="flex items-center gap-2 mb-3">
                <FileText size={16} style={{ color: "var(--yunque-accent)" }} />
                <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>SKILL.md</span>
              </div>
              <pre className="text-xs font-mono whitespace-pre-wrap" style={{ color: "var(--yunque-text-muted)", maxHeight: "60vh", overflow: "auto" }}>
                {detail.content}
              </pre>
            </Card>
          )}
        </div>

        {/* Right: audit + install info + version history */}
        <div className="space-y-4">
          {/* Security score badge */}
          <Card className="section-card p-4">
            <div className="flex items-center gap-2 mb-3">
              <Shield size={16} style={{ color: "var(--yunque-accent)" }} />
              <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>安全评分</span>
            </div>
            <div className="text-3xl font-bold mb-1"><ScoreBadge score={detail.security_score} /></div>
            <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
              {detail.security_score >= 80 ? "高安全性，可放心安装" : detail.security_score >= 60 ? "中等风险，建议审查权限" : "高风险，谨慎安装"}
            </div>
          </Card>

          {/* Audit Report */}
          {detail.audit_report && <AuditSection report={detail.audit_report} />}

          {/* Install Info */}
          {detail.installed && (
            <Card className="section-card p-4">
              <div className="flex items-center gap-2 mb-3">
                <Clock size={16} style={{ color: "var(--yunque-accent)" }} />
                <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>安装信息</span>
              </div>
              <div className="text-xs space-y-2" style={{ color: "var(--yunque-text-muted)" }}>
                <div className="flex items-center justify-between">
                  <span>安装时间</span>
                  <span>{detail.installed_at ? new Date(detail.installed_at).toLocaleDateString("zh-CN") : "—"}</span>
                </div>
                <div className="flex items-center justify-between">
                  <span>更新时间</span>
                  <span>{detail.updated_at ? new Date(detail.updated_at).toLocaleDateString("zh-CN") : "—"}</span>
                </div>
              </div>
            </Card>
          )}

          {/* Version History */}
          {detail.installed && versions.length > 1 && (
            <Card className="section-card p-4">
              <div className="flex items-center justify-between cursor-pointer mb-2" onClick={() => setShowVersions(!showVersions)}>
                <div className="flex items-center gap-2">
                  <History size={16} style={{ color: "var(--yunque-accent)" }} />
                  <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>版本历史 ({versions.length})</span>
                </div>
                {showVersions ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
              </div>
              {showVersions && (
                <div className="space-y-1">
                  {versions.map((v) => (
                    <div key={v.version} className="flex items-center justify-between px-2 py-1.5 rounded-lg text-xs"
                      style={{ background: v.current ? "rgba(23,201,100,0.06)" : "rgba(255,255,255,0.02)" }}>
                      <div className="flex items-center gap-1.5">
                        <Tag size={11} style={{ color: v.current ? "var(--yunque-accent)" : "var(--yunque-text-muted)" }} />
                        <span className={v.current ? "font-medium" : ""} style={{ color: "var(--yunque-text)" }}>{v.version}</span>
                        {v.current && <Chip size="sm" style={{ background: "rgba(23,201,100,0.1)", color: "#17c964" }}>当前</Chip>}
                      </div>
                      {!v.current && (
                        <Button size="sm" variant="outline" onPress={() => doRollback(v.version)} isPending={rollingBack === v.version}>
                          <ArrowDownToLine size={10} /> 回滚
                        </Button>
                      )}
                    </div>
                  ))}
                </div>
              )}
            </Card>
          )}
        </div>
      </div>
    </div>
  );
}
