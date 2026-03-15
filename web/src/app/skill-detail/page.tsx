"use client";

import { Suspense, useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import { api, type SkillHubDetail, type AuditReport, type SkillUpdateInfo, type SkillVersionInfo } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { BlurFade } from "@/components/ui/blur-fade";
import {
  ArrowLeft, Star, Download, Shield, ShieldCheck, ShieldAlert,
  Tag, Clock, User, FileText, Package, ChevronDown, ChevronRight,
  Trash2, Lock, RefreshCw, History, ArrowDownToLine,
} from "lucide-react";
import Link from "next/link";
import PermissionApproval from "@/components/permission-approval";

const sevColors = ["text-blue-400", "text-amber-400", "text-red-400"];
const sevLabels = ["Info", "Warning", "Critical"];

function ScoreBadge({ score }: { score: number }) {
  const color = score >= 80 ? "text-green-400" : score >= 60 ? "text-amber-400" : "text-red-400";
  const Icon = score >= 80 ? ShieldCheck : score >= 60 ? Shield : ShieldAlert;
  return (
    <span className={`flex items-center gap-1 text-sm font-semibold ${color}`}>
      <Icon size={16} /> {score}/100
    </span>
  );
}

function AuditSection({ report }: { report: AuditReport }) {
  const [expanded, setExpanded] = useState(false);
  return (
    <div className="rounded-xl border p-4" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
      <div className="flex items-center justify-between cursor-pointer" onClick={() => setExpanded(!expanded)}>
        <div className="flex items-center gap-2">
          <Shield size={16} style={{ color: "var(--accent)" }} />
          <span className="text-sm font-medium">安全审计报告</span>
        </div>
        <div className="flex items-center gap-3">
          <ScoreBadge score={report.score} />
          {expanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
        </div>
      </div>
      {expanded && (
        <div className="mt-4 space-y-3">
          <div className="grid grid-cols-3 gap-3">
            {[
              { label: "静态分析", score: report.static_score, max: 40 },
              { label: "权限审计", score: report.perm_score, max: 30 },
              { label: "沙箱测试", score: report.sandbox_score, max: 30 },
            ].map((item) => (
              <div key={item.label} className="rounded-lg p-3 text-center" style={{ background: "var(--bg-hover)" }}>
                <div className="text-xs" style={{ color: "var(--text-muted)" }}>{item.label}</div>
                <div className="text-lg font-bold mt-1">{item.score}/{item.max}</div>
              </div>
            ))}
          </div>
          {report.findings?.length > 0 && (
            <div className="space-y-1">
              <div className="text-xs font-medium mb-2" style={{ color: "var(--text-muted)" }}>发现 ({report.findings.length})</div>
              {report.findings.map((f, i) => (
                <div key={i} className="flex items-start gap-2 text-xs py-1.5 px-2 rounded" style={{ background: "var(--bg)" }}>
                  <span className={`shrink-0 ${sevColors[f.severity] || "text-gray-400"}`}>[{sevLabels[f.severity] || "?"}]</span>
                  <span style={{ color: "var(--text-secondary)" }}><strong>{f.rule}:</strong> {f.detail}</span>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

export default function SkillDetailPage() {
  return (
    <Suspense fallback={<div className="animate-in max-w-3xl mx-auto"><div className="skeleton h-40 w-full" /></div>}>
      <SkillDetailContent />
    </Suspense>
  );
}

function SkillDetailContent() {
  const { t } = useI18n();
  const params = useSearchParams();
  const slug = params.get("slug") || "";
  const [detail, setDetail] = useState<SkillHubDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [installing, setInstalling] = useState(false);
  const [showApproval, setShowApproval] = useState(false);
  const [updateInfo, setUpdateInfo] = useState<SkillUpdateInfo | null>(null);
  const [versions, setVersions] = useState<SkillVersionInfo[]>([]);
  const [updating, setUpdating] = useState(false);
  const [rollingBack, setRollingBack] = useState<string | null>(null);
  const [showVersions, setShowVersions] = useState(false);

  useEffect(() => {
    if (!slug) return;
    api.skillHubDetail(slug).then(setDetail).catch(() => {}).finally(() => setLoading(false));
  }, [slug]);

  // Check for updates when detail is loaded and installed
  useEffect(() => {
    if (!detail?.installed || !slug) return;
    api.skillHubCheckUpdates().then((r) => {
      const info = r.updates?.find((u) => u.slug === slug);
      if (info) setUpdateInfo(info);
    }).catch(() => {});
    api.skillHubVersions(slug).then((r) => setVersions(r.versions || [])).catch(() => {});
  }, [detail?.installed, slug]);

  const doInstall = async () => {
    if (!slug) return;
    setShowApproval(false);
    setInstalling(true);
    try {
      await api.skillHubInstall(slug);
      const updated = await api.skillHubDetail(slug);
      setDetail(updated);
    } catch { /* ignore */ }
    setInstalling(false);
  };

  const requestInstall = () => setShowApproval(true);

  const doUpdate = async () => {
    if (!slug) return;
    setUpdating(true);
    try {
      await api.skillHubUpdate(slug);
      const updated = await api.skillHubDetail(slug);
      setDetail(updated);
      setUpdateInfo(null);
      api.skillHubVersions(slug).then((r) => setVersions(r.versions || [])).catch(() => {});
    } catch { /* ignore */ }
    setUpdating(false);
  };

  const doRollback = async (version: string) => {
    if (!slug) return;
    setRollingBack(version);
    try {
      await api.skillHubRollback(slug, version);
      const updated = await api.skillHubDetail(slug);
      setDetail(updated);
      api.skillHubVersions(slug).then((r) => setVersions(r.versions || [])).catch(() => {});
    } catch { /* ignore */ }
    setRollingBack(null);
  };

  const doUninstall = async () => {
    if (!slug) return;
    try {
      await api.skillHubUninstall(slug);
      const updated = await api.skillHubDetail(slug);
      setDetail(updated);
    } catch { /* ignore */ }
  };

  if (loading) {
    return (
      <div className="animate-in max-w-3xl mx-auto">
        <div className="skeleton h-8 w-48 mb-4" />
        <div className="skeleton h-40 w-full mb-4" />
        <div className="skeleton h-60 w-full" />
      </div>
    );
  }

  if (!detail) {
    return (
      <div className="text-center py-20" style={{ color: "var(--text-muted)" }}>
        <Package size={48} className="mx-auto mb-3 opacity-30" />
        <p>技能未找到: {slug}</p>
        <Link href="/skills" className="text-xs mt-2 inline-block" style={{ color: "var(--accent)" }}>← 返回市场</Link>
      </div>
    );
  }

  return (
    <div className="animate-in max-w-3xl mx-auto">
      {/* Back link */}
      <BlurFade delay={0.05}>
        <Link href="/skills" className="inline-flex items-center gap-1 text-xs mb-6" style={{ color: "var(--accent)" }}>
          <ArrowLeft size={14} /> {t("nav.skills")}
        </Link>
      </BlurFade>

      {/* Header */}
      <BlurFade delay={0.1}>
        <div className="flex items-start justify-between mb-6">
          <div>
            <h1 className="text-2xl font-bold">{detail.name}</h1>
            <p className="text-sm mt-1" style={{ color: "var(--text-muted)" }}>{detail.description}</p>
          </div>
          <div className="flex items-center gap-2">
            {detail.installed ? (
              <>
                {updateInfo?.has_update && (
                  <button onClick={doUpdate} disabled={updating}
                    className="flex items-center gap-1.5 px-4 py-2 rounded-lg text-xs font-medium disabled:opacity-50"
                    style={{ background: "var(--accent)", color: "#000" }}>
                    <RefreshCw size={13} className={updating ? "animate-spin" : ""} />
                    {updating ? "更新中..." : `更新到 v${updateInfo.latest_version}`}
                  </button>
                )}
                <button onClick={doUninstall} className="flex items-center gap-1.5 px-4 py-2 rounded-lg text-xs font-medium border"
                  style={{ borderColor: "var(--border)", color: "var(--text-muted)" }}>
                  <Trash2 size={13} /> 卸载
                </button>
              </>
            ) : (
              <button onClick={requestInstall} disabled={installing}
                className="flex items-center gap-1.5 px-4 py-2 rounded-lg text-xs font-medium disabled:opacity-50"
                style={{ background: "var(--accent)", color: "#000" }}>
                <Download size={13} /> {installing ? "安装中..." : "安装"}
              </button>
            )}
          </div>
        </div>
      </BlurFade>

      {/* Meta cards */}
      <BlurFade delay={0.15}>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-6">
          <div className="rounded-xl border p-3" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <div className="flex items-center gap-1.5 text-xs" style={{ color: "var(--text-muted)" }}><Tag size={12} /> 版本</div>
            <div className="text-sm font-semibold mt-1">{detail.version || "—"}</div>
          </div>
          <div className="rounded-xl border p-3" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <div className="flex items-center gap-1.5 text-xs" style={{ color: "var(--text-muted)" }}><User size={12} /> 作者</div>
            <div className="text-sm font-semibold mt-1">{detail.author || "—"}</div>
          </div>
          <div className="rounded-xl border p-3" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <div className="flex items-center gap-1.5 text-xs" style={{ color: "var(--text-muted)" }}><Star size={12} /> 评分</div>
            <div className="text-sm font-semibold mt-1">{detail.rating > 0 ? `${detail.rating.toFixed(1)} (${detail.rating_count})` : "—"}</div>
          </div>
          <div className="rounded-xl border p-3" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <div className="flex items-center gap-1.5 text-xs" style={{ color: "var(--text-muted)" }}><Download size={12} /> 安装量</div>
            <div className="text-sm font-semibold mt-1">{detail.installs > 0 ? detail.installs : "—"}</div>
          </div>
        </div>
      </BlurFade>

      {/* Info sections */}
      <div className="space-y-4">
        {/* Tags & Category */}
        {(detail.category || (detail.tags && detail.tags.length > 0)) && (
          <BlurFade delay={0.2}>
            <div className="rounded-xl border p-4" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
              <div className="flex items-center gap-2 flex-wrap">
                {detail.category && (
                  <span className="px-2 py-0.5 rounded text-xs" style={{ background: "var(--accent-subtle)", color: "var(--accent)" }}>
                    {detail.category}
                  </span>
                )}
                {detail.tags?.map((tag) => (
                  <span key={tag} className="px-2 py-0.5 rounded text-xs" style={{ background: "var(--bg-hover)", color: "var(--text-muted)" }}>
                    {tag}
                  </span>
                ))}
                {detail.source && (
                  <span className="px-2 py-0.5 rounded text-xs" style={{ background: "var(--bg-hover)", color: "var(--text-muted)" }}>
                    来源: {detail.source}
                  </span>
                )}
                {detail.license && (
                  <span className="px-2 py-0.5 rounded text-xs" style={{ background: "var(--bg-hover)", color: "var(--text-muted)" }}>
                    📄 {detail.license}
                  </span>
                )}
              </div>
            </div>
          </BlurFade>
        )}

        {/* Permissions */}
        {detail.permissions && detail.permissions.length > 0 && (
          <BlurFade delay={0.25}>
            <div className="rounded-xl border p-4" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
              <div className="flex items-center gap-2 mb-3">
                <Lock size={16} style={{ color: "var(--accent)" }} />
                <span className="text-sm font-medium">权限要求</span>
              </div>
              <div className="flex flex-wrap gap-2">
                {detail.permissions.map((perm) => {
                  const isDangerous = ["shell", "network", "write"].some((w) => perm.toLowerCase().includes(w));
                  return (
                    <span key={perm} className={`px-2.5 py-1 rounded text-xs ${isDangerous ? "text-red-400" : ""}`}
                      style={{ background: isDangerous ? "rgba(239,68,68,0.1)" : "var(--bg-hover)" }}>
                      {perm}
                    </span>
                  );
                })}
              </div>
            </div>
          </BlurFade>
        )}

        {/* Audit Report */}
        {detail.audit_report && (
          <BlurFade delay={0.3}>
            <AuditSection report={detail.audit_report} />
          </BlurFade>
        )}

        {/* Install info */}
        {detail.installed && (
          <BlurFade delay={0.35}>
            <div className="rounded-xl border p-4" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
              <div className="flex items-center gap-2 mb-2">
                <Clock size={16} style={{ color: "var(--accent)" }} />
                <span className="text-sm font-medium">安装信息</span>
              </div>
              <div className="text-xs space-y-1" style={{ color: "var(--text-muted)" }}>
                {detail.installed_at && <div>安装于: {new Date(detail.installed_at).toLocaleString("zh-CN")}</div>}
                {detail.updated_at && <div>更新于: {new Date(detail.updated_at).toLocaleString("zh-CN")}</div>}
                <div>安全评分: <ScoreBadge score={detail.security_score} /></div>
              </div>
            </div>
          </BlurFade>
        )}

        {/* Version History */}
        {detail.installed && versions.length > 1 && (
          <BlurFade delay={0.37}>
            <div className="rounded-xl border p-4" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
              <div className="flex items-center justify-between cursor-pointer" onClick={() => setShowVersions(!showVersions)}>
                <div className="flex items-center gap-2">
                  <History size={16} style={{ color: "var(--accent)" }} />
                  <span className="text-sm font-medium">版本历史 ({versions.length})</span>
                </div>
                {showVersions ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
              </div>
              {showVersions && (
                <div className="mt-3 space-y-2">
                  {versions.map((v) => (
                    <div key={v.version} className="flex items-center justify-between px-3 py-2 rounded-lg text-xs"
                      style={{ background: v.current ? "rgba(34,197,94,0.06)" : "var(--bg-hover)" }}>
                      <div className="flex items-center gap-2">
                        <Tag size={12} style={{ color: v.current ? "var(--accent)" : "var(--text-muted)" }} />
                        <span className={v.current ? "font-medium" : ""}>{v.version}</span>
                        {v.current && <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-green-500/10 text-green-400">当前</span>}
                        {v.installed_at && (
                          <span style={{ color: "var(--text-muted)" }}>
                            {new Date(v.installed_at).toLocaleDateString("zh-CN")}
                          </span>
                        )}
                      </div>
                      {!v.current && (
                        <button onClick={() => doRollback(v.version)} disabled={rollingBack === v.version}
                          className="flex items-center gap-1 px-2 py-1 rounded text-[11px] font-medium border disabled:opacity-50"
                          style={{ borderColor: "var(--border)", color: "var(--text-muted)" }}>
                          <ArrowDownToLine size={10} />
                          {rollingBack === v.version ? "回滚中..." : "回滚"}
                        </button>
                      )}
                    </div>
                  ))}
                </div>
              )}
            </div>
          </BlurFade>
        )}

        {/* SKILL.md Content */}
        {detail.content && (
          <BlurFade delay={0.4}>
            <div className="rounded-xl border p-4" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
              <div className="flex items-center gap-2 mb-3">
                <FileText size={16} style={{ color: "var(--accent)" }} />
                <span className="text-sm font-medium">SKILL.md</span>
              </div>
              <pre className="text-xs font-mono whitespace-pre-wrap max-h-96 overflow-auto"
                style={{ color: "var(--text-secondary)" }}>
                {detail.content}
              </pre>
            </div>
          </BlurFade>
        )}
      </div>

      {showApproval && (
        <PermissionApproval
          slug={slug}
          onApprove={doInstall}
          onCancel={() => setShowApproval(false)}
        />
      )}
    </div>
  );
}
