"use client";

import { useEffect, useState } from "react";
import { api, type SkillPolicy } from "@/lib/api";
import { BlurFade } from "@/components/ui/blur-fade";
import {
  Shield, ShieldCheck, ShieldAlert, Save, Plus, X, ArrowLeft,
  UserCheck, UserX, Lock, ListCheck, Ban,
} from "lucide-react";
import Link from "next/link";

const defaultPolicy: SkillPolicy = {
  min_score: 60,
  trusted_authors: [],
  blocked_authors: [],
  allowed_slugs: [],
  blocked_slugs: [],
  max_perm_level: "shell",
  require_audit: false,
  auto_approve_min: 80,
};

const permLevels = [
  { value: "read-only", label: "只读", desc: "仅允许只读技能" },
  { value: "write", label: "读写", desc: "允许文件写入" },
  { value: "network", label: "网络", desc: "允许网络访问" },
  { value: "shell", label: "Shell", desc: "允许所有权限 (包括 shell 执行)" },
];

export default function SkillPolicyPage() {
  const [policy, setPolicy] = useState<SkillPolicy>(defaultPolicy);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);

  // Input states for adding items to lists
  const [newTrusted, setNewTrusted] = useState("");
  const [newBlocked, setNewBlocked] = useState("");
  const [newAllowed, setNewAllowed] = useState("");
  const [newBlockedSlug, setNewBlockedSlug] = useState("");

  useEffect(() => {
    api.skillHubGetPolicy().then((p) => setPolicy({
      ...defaultPolicy,
      ...p,
      trusted_authors: p.trusted_authors ?? [],
      blocked_authors: p.blocked_authors ?? [],
      allowed_slugs: p.allowed_slugs ?? [],
      blocked_slugs: p.blocked_slugs ?? [],
    })).catch(() => {}).finally(() => setLoading(false));
  }, []);

  const doSave = async () => {
    setSaving(true);
    try {
      await api.skillHubSetPolicy(policy);
      setSaved(true);
      setTimeout(() => setSaved(false), 2000);
    } catch { /* ignore */ }
    setSaving(false);
  };

  const addToList = (field: keyof SkillPolicy, value: string, setter: (v: string) => void) => {
    if (!value.trim()) return;
    const list = (policy[field] as string[]) ?? [];
    if (!list.includes(value.trim())) {
      setPolicy({ ...policy, [field]: [...list, value.trim()] });
    }
    setter("");
  };

  const removeFromList = (field: keyof SkillPolicy, value: string) => {
    const list = (policy[field] as string[]) ?? [];
    setPolicy({ ...policy, [field]: list.filter((v) => v !== value) });
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-[60vh]">
        <div className="w-5 h-5 border-2 border-t-transparent rounded-full animate-spin"
          style={{ borderColor: "var(--text-muted)", borderTopColor: "transparent" }} />
      </div>
    );
  }

  return (
    <div className="max-w-3xl">
      <BlurFade delay={0}>
        <Link href="/skills" className="inline-flex items-center gap-1 text-xs mb-6" style={{ color: "var(--accent)" }}>
          <ArrowLeft size={14} /> 返回技能市场
        </Link>
      </BlurFade>

      <BlurFade delay={0.05}>
        <div className="flex items-center justify-between mb-6">
          <div className="flex items-center gap-3">
            <Shield size={20} />
            <h1 className="text-xl font-semibold tracking-tight">安全策略</h1>
          </div>
          <button onClick={doSave} disabled={saving}
            className="flex items-center gap-1.5 px-4 py-2 rounded-lg text-xs font-medium disabled:opacity-50"
            style={{ background: saved ? "rgb(34,197,94)" : "var(--accent)", color: "#000" }}>
            {saved ? <ShieldCheck size={13} /> : <Save size={13} />}
            {saving ? "保存中..." : saved ? "已保存" : "保存策略"}
          </button>
        </div>
      </BlurFade>

      <div className="space-y-4">
        {/* Score thresholds */}
        <BlurFade delay={0.1}>
          <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <div className="flex items-center gap-2 mb-4">
              <ShieldAlert size={16} style={{ color: "var(--accent)" }} />
              <span className="text-sm font-medium">评分阈值</span>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <label className="text-xs block mb-1.5" style={{ color: "var(--text-muted)" }}>最低安全评分 (0-100)</label>
                <input type="number" min={0} max={100} value={policy.min_score}
                  onChange={(e) => setPolicy({ ...policy, min_score: parseInt(e.target.value) || 0 })}
                  className="w-full px-3 py-2 rounded-lg border text-sm bg-transparent"
                  style={{ borderColor: "var(--border)" }} />
                <div className="text-[10px] mt-1" style={{ color: "var(--text-muted)" }}>低于此分数的技能将被阻止安装</div>
              </div>
              <div>
                <label className="text-xs block mb-1.5" style={{ color: "var(--text-muted)" }}>自动审批阈值 (0-100)</label>
                <input type="number" min={0} max={100} value={policy.auto_approve_min}
                  onChange={(e) => setPolicy({ ...policy, auto_approve_min: parseInt(e.target.value) || 0 })}
                  className="w-full px-3 py-2 rounded-lg border text-sm bg-transparent"
                  style={{ borderColor: "var(--border)" }} />
                <div className="text-[10px] mt-1" style={{ color: "var(--text-muted)" }}>高于此分数的技能无需人工确认</div>
              </div>
            </div>
          </div>
        </BlurFade>

        {/* Permission level */}
        <BlurFade delay={0.15}>
          <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <div className="flex items-center gap-2 mb-4">
              <Lock size={16} style={{ color: "var(--accent)" }} />
              <span className="text-sm font-medium">最大权限级别</span>
            </div>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
              {permLevels.map((level) => (
                <button key={level.value}
                  onClick={() => setPolicy({ ...policy, max_perm_level: level.value })}
                  className="px-3 py-2.5 rounded-lg border text-xs text-left transition-all"
                  style={{
                    borderColor: policy.max_perm_level === level.value ? "var(--accent)" : "var(--border)",
                    background: policy.max_perm_level === level.value ? "rgba(var(--accent-rgb),0.08)" : "transparent",
                  }}>
                  <div className="font-medium">{level.label}</div>
                  <div className="mt-0.5" style={{ color: "var(--text-muted)", fontSize: "10px" }}>{level.desc}</div>
                </button>
              ))}
            </div>
          </div>
        </BlurFade>

        {/* Audit requirement */}
        <BlurFade delay={0.18}>
          <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <label className="flex items-center gap-3 cursor-pointer">
              <input type="checkbox" checked={policy.require_audit}
                onChange={(e) => setPolicy({ ...policy, require_audit: e.target.checked })}
                className="rounded"
              />
              <div>
                <div className="text-sm font-medium">强制审计</div>
                <div className="text-[10px]" style={{ color: "var(--text-muted)" }}>如未完成安全审计，拒绝安装</div>
              </div>
            </label>
          </div>
        </BlurFade>

        {/* Trusted Authors */}
        <BlurFade delay={0.2}>
          <ListSection
            icon={<UserCheck size={16} className="text-green-400" />}
            title="信任作者"
            desc="信任作者的技能自动审批"
            items={policy.trusted_authors}
            inputValue={newTrusted}
            onInputChange={setNewTrusted}
            onAdd={() => addToList("trusted_authors", newTrusted, setNewTrusted)}
            onRemove={(v) => removeFromList("trusted_authors", v)}
            colorClass="text-green-400"
          />
        </BlurFade>

        {/* Blocked Authors */}
        <BlurFade delay={0.25}>
          <ListSection
            icon={<UserX size={16} className="text-red-400" />}
            title="黑名单作者"
            desc="禁止安装此作者的所有技能"
            items={policy.blocked_authors}
            inputValue={newBlocked}
            onInputChange={setNewBlocked}
            onAdd={() => addToList("blocked_authors", newBlocked, setNewBlocked)}
            onRemove={(v) => removeFromList("blocked_authors", v)}
            colorClass="text-red-400"
          />
        </BlurFade>

        {/* Allowed Slugs */}
        <BlurFade delay={0.3}>
          <ListSection
            icon={<ListCheck size={16} className="text-green-400" />}
            title="白名单技能"
            desc="直接放行，跳过评分检查"
            items={policy.allowed_slugs}
            inputValue={newAllowed}
            onInputChange={setNewAllowed}
            onAdd={() => addToList("allowed_slugs", newAllowed, setNewAllowed)}
            onRemove={(v) => removeFromList("allowed_slugs", v)}
            colorClass="text-green-400"
          />
        </BlurFade>

        {/* Blocked Slugs */}
        <BlurFade delay={0.35}>
          <ListSection
            icon={<Ban size={16} className="text-red-400" />}
            title="黑名单技能"
            desc="禁止安装特定技能"
            items={policy.blocked_slugs}
            inputValue={newBlockedSlug}
            onInputChange={setNewBlockedSlug}
            onAdd={() => addToList("blocked_slugs", newBlockedSlug, setNewBlockedSlug)}
            onRemove={(v) => removeFromList("blocked_slugs", v)}
            colorClass="text-red-400"
          />
        </BlurFade>
      </div>
    </div>
  );
}

function ListSection({
  icon, title, desc, items, inputValue, onInputChange, onAdd, onRemove, colorClass,
}: {
  icon: React.ReactNode;
  title: string;
  desc: string;
  items: string[];
  inputValue: string;
  onInputChange: (v: string) => void;
  onAdd: () => void;
  onRemove: (v: string) => void;
  colorClass: string;
}) {
  return (
    <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
      <div className="flex items-center gap-2 mb-1">{icon}<span className="text-sm font-medium">{title}</span></div>
      <div className="text-[10px] mb-3" style={{ color: "var(--text-muted)" }}>{desc}</div>

      <div className="flex gap-2 mb-3">
        <input value={inputValue} onChange={(e) => onInputChange(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && onAdd()}
          placeholder="输入后回车添加..."
          className="flex-1 px-3 py-1.5 rounded-lg border text-xs bg-transparent"
          style={{ borderColor: "var(--border)" }} />
        <button onClick={onAdd} disabled={!inputValue.trim()}
          className="px-3 py-1.5 rounded-lg text-xs font-medium border disabled:opacity-30"
          style={{ borderColor: "var(--border)" }}>
          <Plus size={12} />
        </button>
      </div>

      {items && items.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {items.map((item) => (
            <span key={item} className={`flex items-center gap-1 px-2 py-1 rounded text-xs ${colorClass}`}
              style={{ background: "var(--bg-hover)" }}>
              {item}
              <button onClick={() => onRemove(item)} className="ml-0.5 hover:opacity-70"><X size={10} /></button>
            </span>
          ))}
        </div>
      )}
    </div>
  );
}
