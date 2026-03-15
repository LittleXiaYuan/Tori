"use client";

import { useEffect, useState } from "react";
import { api, type TenantInfo } from "@/lib/api";
import { Users, Plus, Copy, Check } from "lucide-react";

export default function TenantsPage() {
  const [tenants, setTenants] = useState<TenantInfo[]>([]);
  const [newName, setNewName] = useState("");
  const [copied, setCopied] = useState<string | null>(null);

  useEffect(() => {
    api.tenants().then((res) => setTenants(res.tenants || [])).catch(() => {});
  }, []);

  const create = async () => {
    if (!newName.trim()) return;
    try {
      const t = await api.createTenant(newName.trim());
      setTenants((prev) => [...prev, t]);
      setNewName("");
    } catch {}
  };

  const copyKey = (key: string) => {
    navigator.clipboard.writeText(key);
    setCopied(key);
    setTimeout(() => setCopied(null), 2000);
  };

  return (
    <div>
      <h1 className="text-xl font-semibold mb-6">Tenants</h1>

      <div className="flex gap-3 mb-6">
        <input
          value={newName}
          onChange={(e) => setNewName(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && create()}
          placeholder="New tenant name..."
          className="flex-1 rounded-xl px-4 py-2.5 text-sm outline-none border"
          style={{ background: "var(--bg-card)", borderColor: "var(--border)", color: "var(--text)" }}
        />
        <button onClick={create} disabled={!newName.trim()}
          className="flex items-center gap-2 rounded-xl px-4 py-2.5 text-sm font-medium transition-colors"
          style={{
            background: newName.trim() ? "var(--accent)" : "var(--bg-hover)",
            color: newName.trim() ? "white" : "var(--text-muted)",
          }}>
          <Plus size={16} /> Create
        </button>
      </div>

      <div className="space-y-2">
        {tenants.map((t) => (
          <div key={t.id} className="rounded-xl border p-4 flex items-center gap-4"
            style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <div className="w-10 h-10 rounded-lg flex items-center justify-center"
              style={{ background: "#6366f115", color: "var(--accent)" }}>
              <Users size={18} />
            </div>
            <div className="flex-1 min-w-0">
              <div className="font-medium text-sm">{t.name}</div>
              <div className="text-xs truncate" style={{ color: "var(--text-muted)" }}>
                ID: {t.id}
              </div>
            </div>
            <div className="flex items-center gap-2">
              <code className="text-xs px-2 py-1 rounded"
                style={{ background: "var(--bg-hover)", color: "var(--text-muted)" }}>
                {t.api_key.slice(0, 10)}...
              </code>
              <button onClick={() => copyKey(t.api_key)}
                className="p-1.5 rounded-lg transition-colors"
                style={{ color: copied === t.api_key ? "var(--success)" : "var(--text-muted)" }}>
                {copied === t.api_key ? <Check size={14} /> : <Copy size={14} />}
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
