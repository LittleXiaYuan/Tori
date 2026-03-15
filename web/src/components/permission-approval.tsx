"use client";

import { useEffect, useState } from "react";
import { api, type SkillHubDetail } from "@/lib/api";
import { Shield, ShieldAlert, ShieldCheck, AlertTriangle, X, Download } from "lucide-react";

interface PermissionApprovalProps {
  slug: string;
  onApprove: () => void;
  onCancel: () => void;
}

const permDescriptions: Record<string, { label: string; danger: boolean }> = {
  "read-only": { label: "只读文件访问", danger: false },
  "read": { label: "读取文件", danger: false },
  "write": { label: "写入文件系统", danger: true },
  "network": { label: "网络访问", danger: true },
  "shell": { label: "执行 Shell 命令", danger: true },
  "exec": { label: "执行外部程序", danger: true },
  "env": { label: "访问环境变量", danger: false },
  "memory": { label: "访问 Agent 记忆", danger: false },
};

function getPermInfo(perm: string) {
  const lower = perm.toLowerCase();
  for (const [key, info] of Object.entries(permDescriptions)) {
    if (lower.includes(key)) return info;
  }
  return { label: perm, danger: lower.includes("shell") || lower.includes("write") || lower.includes("network") };
}

export default function PermissionApproval({ slug, onApprove, onCancel }: PermissionApprovalProps) {
  const [detail, setDetail] = useState<SkillHubDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [confirming, setConfirming] = useState(false);

  useEffect(() => {
    api.skillHubDetail(slug).then(setDetail).catch(() => {}).finally(() => setLoading(false));
  }, [slug]);

  const hasDangerousPerms = detail?.permissions?.some((p) => getPermInfo(p).danger) ?? false;

  const handleApprove = async () => {
    if (hasDangerousPerms && !confirming) {
      setConfirming(true);
      return;
    }
    onApprove();
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center" style={{ background: "rgba(0,0,0,0.6)" }}>
      <div className="w-full max-w-md rounded-2xl border p-6 mx-4 animate-in"
        style={{ background: "var(--bg-card)", borderColor: "var(--border)", boxShadow: "var(--shadow-lg)" }}>

        {/* Header */}
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-2">
            <Shield size={20} style={{ color: "var(--accent)" }} />
            <h3 className="text-base font-semibold">安装确认</h3>
          </div>
          <button onClick={onCancel} className="p-1 rounded-lg hover:bg-[var(--bg-hover)]" style={{ color: "var(--text-muted)" }}>
            <X size={16} />
          </button>
        </div>

        {loading ? (
          <div className="py-8 text-center text-sm" style={{ color: "var(--text-muted)" }}>加载中...</div>
        ) : !detail ? (
          <div className="py-8 text-center text-sm" style={{ color: "var(--text-muted)" }}>无法获取技能信息</div>
        ) : (
          <>
            {/* Skill info */}
            <div className="mb-4">
              <div className="text-sm font-medium">{detail.name}</div>
              <div className="text-xs mt-1" style={{ color: "var(--text-muted)" }}>
                {detail.author && `by ${detail.author} · `}v{detail.version}
                {detail.source && ` · ${detail.source}`}
              </div>
            </div>

            {/* Permissions */}
            {detail.permissions && detail.permissions.length > 0 ? (
              <div className="mb-4">
                <div className="text-xs font-medium mb-2" style={{ color: "var(--text-muted)" }}>
                  该技能需要以下权限：
                </div>
                <div className="space-y-1.5">
                  {detail.permissions.map((perm) => {
                    const info = getPermInfo(perm);
                    return (
                      <div key={perm} className="flex items-center gap-2 px-3 py-2 rounded-lg text-xs"
                        style={{ background: info.danger ? "rgba(239,68,68,0.08)" : "var(--bg-hover)" }}>
                        {info.danger ? (
                          <AlertTriangle size={14} className="text-red-400 shrink-0" />
                        ) : (
                          <ShieldCheck size={14} className="text-green-400 shrink-0" />
                        )}
                        <span className={info.danger ? "text-red-400" : ""}>{info.label}</span>
                      </div>
                    );
                  })}
                </div>
              </div>
            ) : (
              <div className="mb-4 px-3 py-2 rounded-lg text-xs" style={{ background: "var(--bg-hover)" }}>
                <div className="flex items-center gap-2">
                  <ShieldCheck size={14} className="text-green-400" />
                  <span>此技能无需特殊权限</span>
                </div>
              </div>
            )}

            {/* Security score */}
            {detail.security_score > 0 && (
              <div className="mb-4 px-3 py-2 rounded-lg text-xs" style={{ background: "var(--bg-hover)" }}>
                <div className="flex items-center gap-2">
                  {detail.security_score >= 80 ? (
                    <ShieldCheck size={14} className="text-green-400" />
                  ) : detail.security_score >= 60 ? (
                    <Shield size={14} className="text-amber-400" />
                  ) : (
                    <ShieldAlert size={14} className="text-red-400" />
                  )}
                  <span>安全评分: {detail.security_score}/100</span>
                </div>
              </div>
            )}

            {/* Danger confirmation */}
            {hasDangerousPerms && confirming && (
              <div className="mb-4 px-3 py-3 rounded-lg border text-xs"
                style={{ background: "rgba(239,68,68,0.05)", borderColor: "rgba(239,68,68,0.3)" }}>
                <div className="flex items-center gap-2 text-red-400 font-medium mb-1">
                  <AlertTriangle size={14} />
                  此技能包含高危权限
                </div>
                <div style={{ color: "var(--text-muted)" }}>
                  包含 shell 执行、网络访问或文件写入等权限。确定要继续安装吗？
                </div>
              </div>
            )}

            {/* Actions */}
            <div className="flex gap-3 mt-5">
              <button onClick={onCancel}
                className="flex-1 px-4 py-2.5 rounded-lg text-xs font-medium border"
                style={{ borderColor: "var(--border)", color: "var(--text-muted)" }}>
                取消
              </button>
              <button onClick={handleApprove}
                className="flex-1 flex items-center justify-center gap-1.5 px-4 py-2.5 rounded-lg text-xs font-medium"
                style={{
                  background: confirming ? "rgba(239,68,68,0.8)" : "var(--accent)",
                  color: confirming ? "#fff" : "#000",
                }}>
                <Download size={13} />
                {confirming ? "确认安装" : hasDangerousPerms ? "查看风险" : "确认安装"}
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
