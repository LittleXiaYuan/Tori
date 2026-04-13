"use client";

import { useEffect, useState } from "react";
import { api, type SkillPolicy } from "@/lib/api";
import { Card, Button, Switch, TextField, Input, Label } from "@heroui/react";
import { Shield, ShieldCheck, ShieldAlert, Save, Plus, X, ArrowLeft, UserCheck, UserX, Lock, ListCheck, Ban } from "lucide-react";
import Link from "next/link";
import { showToast } from "@/components/toast-provider";

const defaultPolicy: SkillPolicy = {
  min_score: 60, trusted_authors: [], blocked_authors: [], allowed_slugs: [], blocked_slugs: [],
  max_perm_level: "shell", require_audit: false, auto_approve_min: 80,
};
const permLevels = [
  { value: "read-only", label: "只读", desc: "仅允许只读技能" },
  { value: "write", label: "读写", desc: "允许文件写入" },
  { value: "network", label: "网络", desc: "允许网络访问" },
  { value: "shell", label: "Shell", desc: "允许所有权限" },
];

function ListSection({ icon, title, desc, items, inputValue, onInputChange, onAdd, onRemove, colorClass }: {
  icon: React.ReactNode; title: string; desc: string; items: string[]; inputValue: string;
  onInputChange: (v: string) => void; onAdd: () => void; onRemove: (v: string) => void; colorClass: string;
}) {
  return (
    <Card className="section-card p-5">
      <div className="flex items-center gap-2 mb-1">{icon}<span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{title}</span></div>
      <div className="text-[10px] mb-3" style={{ color: "var(--yunque-text-muted)" }}>{desc}</div>
      <div className="flex gap-2 mb-3">
        <Input value={inputValue} onChange={(e) => onInputChange(e.target.value)} onKeyDown={(e) => e.key === "Enter" && onAdd()}
          placeholder="输入后回车添加..." />
        <Button isIconOnly aria-label="添加" size="sm" variant="outline" onPress={onAdd} isDisabled={!inputValue.trim()}><Plus size={12} /></Button>
      </div>
      {items && items.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {items.map((item) => (
            <span key={item} className="inline-flex items-center gap-1 px-2 py-1 rounded text-xs"
              style={{ background: "rgba(255,255,255,0.04)", color: colorClass === "text-green-400" ? "#17c964" : "#f31260" }}>
              {item}
              <Button isIconOnly aria-label="关闭" variant="ghost" size="sm" onPress={() => onRemove(item)} className="ml-0.5"><X size={10} /></Button>
            </span>
          ))}
        </div>
      )}
    </Card>
  );
}

export default function SkillPolicyPage() {
  const [policy, setPolicy] = useState<SkillPolicy>(defaultPolicy);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [newTrusted, setNewTrusted] = useState("");
  const [newBlocked, setNewBlocked] = useState("");
  const [newAllowed, setNewAllowed] = useState("");
  const [newBlockedSlug, setNewBlockedSlug] = useState("");

  useEffect(() => {
    api.skillHubGetPolicy().then((p) => setPolicy({
      ...defaultPolicy, ...p,
      trusted_authors: p.trusted_authors ?? [], blocked_authors: p.blocked_authors ?? [],
      allowed_slugs: p.allowed_slugs ?? [], blocked_slugs: p.blocked_slugs ?? [],
    })).catch(() => {}).finally(() => setLoading(false));
  }, []);

  const doSave = async () => {
    setSaving(true);
    try { await api.skillHubSetPolicy(policy); setSaved(true); setTimeout(() => setSaved(false), 2000); showToast("策略已保存", "success"); }
    catch (e) { showToast(e instanceof Error ? e.message : "保存失败", "error"); }
    setSaving(false);
  };

  const addToList = (field: keyof SkillPolicy, value: string, setter: (v: string) => void) => {
    if (!value.trim()) return;
    const list = (policy[field] as string[]) ?? [];
    if (!list.includes(value.trim())) setPolicy({ ...policy, [field]: [...list, value.trim()] });
    setter("");
  };
  const removeFromList = (field: keyof SkillPolicy, value: string) => {
    const list = (policy[field] as string[]) ?? [];
    setPolicy({ ...policy, [field]: list.filter((v) => v !== value) });
  };

  if (loading) return (
    <div className="flex items-center justify-center h-[60vh]">
      <div className="w-5 h-5 border-2 border-t-transparent rounded-full animate-spin" style={{ borderColor: "var(--yunque-text-muted)", borderTopColor: "transparent" }} />
    </div>
  );

  return (
    <div className="page-root space-y-5 animate-fade-in-up">
      <Link href="/skills" className="inline-flex items-center gap-1 text-xs" style={{ color: "var(--yunque-accent)" }}>
        <ArrowLeft size={14} /> 返回技能市场
      </Link>

      <div className="page-header">
        <h1 className="page-title" style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <span style={{ color: "var(--yunque-accent)", display: "flex" }}><Shield size={20} /></span>
          安全策略
        </h1>
        <Button size="sm" onPress={doSave} isPending={saving} style={{ background: saved ? "#17c964" : "var(--yunque-accent)", color: "#fff" }}>
          {saved ? <><ShieldCheck size={13} /> 已保存</> : <><Save size={13} /> 保存策略</>}
        </Button>
      </div>

      {/* Score Thresholds */}
      <Card className="section-card p-5">
        <div className="flex items-center gap-2 mb-4">
          <ShieldAlert size={16} style={{ color: "var(--yunque-accent)" }} />
          <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>评分阈值</span>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div>
            <TextField>
              <Label>最低安全评分 (0-100)</Label>
              <Input type="number" min={0} max={100} value={String(policy.min_score)}
                onChange={(e) => setPolicy({ ...policy, min_score: parseInt(e.target.value) || 0 })} />
            </TextField>
            <div className="text-[10px] mt-1" style={{ color: "var(--yunque-text-muted)" }}>低于此分数的技能将被阻止安装</div>
          </div>
          <div>
            <TextField>
              <Label>自动审批阈值 (0-100)</Label>
              <Input type="number" min={0} max={100} value={String(policy.auto_approve_min)}
                onChange={(e) => setPolicy({ ...policy, auto_approve_min: parseInt(e.target.value) || 0 })} />
            </TextField>
            <div className="text-[10px] mt-1" style={{ color: "var(--yunque-text-muted)" }}>高于此分数的技能无需人工确认</div>
          </div>
        </div>
      </Card>

      {/* Permission Level */}
      <Card className="section-card p-5">
        <div className="flex items-center gap-2 mb-4">
          <Lock size={16} style={{ color: "var(--yunque-accent)" }} />
          <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>最大权限级别</span>
        </div>
        <div className="kpi-grid">
          {permLevels.map((level) => (
            <Button key={level.value} variant="ghost" onPress={() => setPolicy({ ...policy, max_perm_level: level.value })}
              className="px-3 py-2.5 rounded-lg border text-xs text-left h-auto"
              style={{
                borderColor: policy.max_perm_level === level.value ? "var(--yunque-accent)" : "var(--yunque-border)",
                background: policy.max_perm_level === level.value ? "rgba(0,111,238,0.08)" : "transparent",
                color: "var(--yunque-text)",
              }}>
              <div className="font-medium">{level.label}</div>
              <div className="mt-0.5" style={{ color: "var(--yunque-text-muted)", fontSize: "10px" }}>{level.desc}</div>
            </Button>
          ))}
        </div>
      </Card>

      {/* Audit Requirement */}
      <Card className="section-card p-5">
        <div className="flex items-center gap-3">
          <Switch isSelected={policy.require_audit} onChange={(val) => setPolicy({ ...policy, require_audit: val })} size="sm">
            <Switch.Control><Switch.Thumb /></Switch.Control>
          </Switch>
          <div>
            <div className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>强制审计</div>
            <div className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>如未完成安全审计，拒绝安装</div>
          </div>
        </div>
      </Card>

      {/* Lists */}
      <ListSection icon={<UserCheck size={16} style={{ color: "#17c964" }} />} title="信任作者" desc="信任作者的技能自动审批"
        items={policy.trusted_authors} inputValue={newTrusted} onInputChange={setNewTrusted}
        onAdd={() => addToList("trusted_authors", newTrusted, setNewTrusted)} onRemove={(v) => removeFromList("trusted_authors", v)} colorClass="text-green-400" />
      <ListSection icon={<UserX size={16} style={{ color: "#f31260" }} />} title="黑名单作者" desc="禁止安装此作者的所有技能"
        items={policy.blocked_authors} inputValue={newBlocked} onInputChange={setNewBlocked}
        onAdd={() => addToList("blocked_authors", newBlocked, setNewBlocked)} onRemove={(v) => removeFromList("blocked_authors", v)} colorClass="text-red-400" />
      <ListSection icon={<ListCheck size={16} style={{ color: "#17c964" }} />} title="白名单技能" desc="直接放行，跳过评分检查"
        items={policy.allowed_slugs} inputValue={newAllowed} onInputChange={setNewAllowed}
        onAdd={() => addToList("allowed_slugs", newAllowed, setNewAllowed)} onRemove={(v) => removeFromList("allowed_slugs", v)} colorClass="text-green-400" />
      <ListSection icon={<Ban size={16} style={{ color: "#f31260" }} />} title="黑名单技能" desc="禁止安装特定技能"
        items={policy.blocked_slugs} inputValue={newBlockedSlug} onInputChange={setNewBlockedSlug}
        onAdd={() => addToList("blocked_slugs", newBlockedSlug, setNewBlockedSlug)} onRemove={(v) => removeFromList("blocked_slugs", v)} colorClass="text-red-400" />
    </div>
  );
}
