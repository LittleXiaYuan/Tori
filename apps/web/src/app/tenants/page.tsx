"use client";

import { useState } from "react";
import { Card, Button, Spinner, Chip, Tooltip, Avatar, TextField, Input, Label } from "@heroui/react";
import { api, TenantInfo } from "@/lib/api";
import { useApiData } from "@/lib/use-api-data";
import { Users, Plus, Copy, Check, Trash2, RefreshCw, Key, Calendar } from "lucide-react";
import { showToast } from "@/components/toast-provider";
import PageHeader from "@/components/page-header";

export default function TenantsPage() {
  const { data: tenants, setData: setTenants, loading, refresh } = useApiData(
    async () => { const res = await api.tenants(); return res.tenants || []; },
    [] as TenantInfo[],
  );
  const [creating, setCreating] = useState(false);
  const [newName, setNewName] = useState("");
  const [copiedId, setCopiedId] = useState<string | null>(null);

  const handleCreate = async () => {
    if (!newName.trim()) return;
    setCreating(true);
    try {
      const t = await api.createTenant(newName.trim());
      setTenants((prev) => [...prev, t]);
      setNewName("");
    } catch (e) { showToast(e instanceof Error ? e.message : "创建失败", "error"); }
    setCreating(false);
  };

  const handleCopyKey = (id: string, key: string) => {
    navigator.clipboard.writeText(key);
    setCopiedId(id);
    setTimeout(() => setCopiedId(null), 2000);
  };

  if (loading) {
    return <div className="flex items-center justify-center h-[60vh]"><Spinner size="lg" /></div>;
  }

  return (
    <div className="page-root animate-fade-in-up">
      <PageHeader icon={<Users size={20} />} title="租户管理" onRefresh={refresh} />

      {/* Create form */}
      <Card className="section-card p-4 mb-6">
        <div className="flex items-center gap-3">
          <Plus size={16} style={{ color: "var(--yunque-accent)" }} />
          <TextField className="flex-1" value={newName} onChange={setNewName}>
            <Label className="sr-only">租户名称</Label>
            <Input placeholder="输入租户名称..."
              onKeyDown={(e: React.KeyboardEvent) => { if (e.key === "Enter") handleCreate(); }} />
          </TextField>
          <Button
            size="sm"
            isPending={creating}
            isDisabled={!newName.trim()}
            onPress={handleCreate}
            className="btn-accent"
          >
            {"创建"}
          </Button>
        </div>
      </Card>

      {/* Tenant list */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 stagger-children">
        {tenants.map((t) => (
          <Card key={t.id} className="section-card p-5 hover-lift">
            <div className="flex items-center gap-4">
              <Avatar size="md" style={{ background: `hsl(${t.name.charCodeAt(0) * 7 % 360}, 50%, 30%)` }}>
                <Avatar.Fallback className="text-white text-sm font-bold">
                  {t.name.charAt(0).toUpperCase()}
                </Avatar.Fallback>
              </Avatar>
              <div className="flex-1 min-w-0">
                <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>{t.name}</div>
                <div className="flex items-center gap-3 mt-1.5">
                  <div className="flex items-center gap-1 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                    <Key size={11} />
                    <code className="font-mono">{t.api_key.slice(0, 12)}...</code>
                    <Button isIconOnly aria-label="确认" variant="ghost" size="sm" onPress={() => handleCopyKey(t.id, t.api_key)}>
                      {copiedId === t.id ? (
                        <Check size={11} className="text-green-400" />
                      ) : (
                        <Copy size={11} className="opacity-60" />
                      )}
                    </Button>
                  </div>
                  <div className="flex items-center gap-1 text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
                    <Calendar size={10} />
                    {new Date(t.created_at).toLocaleDateString()}
                  </div>
                </div>
              </div>
              <Chip size="sm" style={{ background: "rgba(34,197,94,0.12)", color: "#22c55e", fontSize: 10 }}>{"活跃"}</Chip>
            </div>
          </Card>
        ))}
        {tenants.length === 0 && (
          <Card className="section-card p-12 text-center">
            <Users size={40} className="mx-auto mb-3" style={{ color: "var(--yunque-text-muted)" }} />
            <div className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>{"暂无租户，点击上方创建"}</div>
          </Card>
        )}
      </div>
    </div>
  );
}
