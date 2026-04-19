"use client";

import { useState, useEffect, useCallback } from "react";
import { api } from "@/lib/api";
import type { ProjectInfo } from "@/lib/api-types";
import { Card, Button, Spinner } from "@heroui/react";
import {
  FolderGit2, Plus, Trash2, RefreshCw, Pencil, X, Check, GitBranch,
} from "lucide-react";
import { showToast } from "@/components/toast-provider";
import EmptyState from "@/components/empty-state";

export default function ProjectsPage() {
  const [projects, setProjects] = useState<ProjectInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);

  const [form, setForm] = useState({ name: "", repo_path: "", repo_url: "", description: "" });

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await api.listProjects();
      setProjects(res.projects || []);
    } catch { /* ignore */ }
    setLoading(false);
  }, []);

  useEffect(() => { load(); }, [load]);

  const handleCreate = async () => {
    if (!form.name || !form.repo_path) {
      showToast("名称和仓库路径必填", "warning");
      return;
    }
    try {
      await api.createProject({
        name: form.name,
        repo_path: form.repo_path,
        repo_url: form.repo_url || undefined,
        description: form.description || undefined,
      });
      showToast("项目已创建", "success");
      setShowCreate(false);
      setForm({ name: "", repo_path: "", repo_url: "", description: "" });
      load();
    } catch (e) {
      showToast(`创建失败: ${(e as Error).message}`, "error");
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await api.removeProject(id);
      showToast("已删除", "success");
      load();
    } catch (e) {
      showToast(`删除失败: ${(e as Error).message}`, "error");
    }
  };

  const handleUpdate = async (id: string) => {
    try {
      await api.updateProject(id, {
        name: form.name || undefined,
        repo_path: form.repo_path || undefined,
        repo_url: form.repo_url || undefined,
        description: form.description || undefined,
      });
      showToast("已更新", "success");
      setEditingId(null);
      load();
    } catch (e) {
      showToast(`更新失败: ${(e as Error).message}`, "error");
    }
  };

  const startEdit = (p: ProjectInfo) => {
    setEditingId(p.id);
    setForm({ name: p.name, repo_path: p.repo_path, repo_url: p.repo_url || "", description: p.description || "" });
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <Spinner size="lg" />
      </div>
    );
  }

  return (
    <div className="p-6 max-w-4xl mx-auto space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold" style={{ color: "var(--yunque-text)" }}>
          <FolderGit2 className="inline mr-2" size={22} /> 项目管理
        </h1>
        <div className="flex gap-2">
          <Button size="sm" variant="ghost" onPress={load}>
            <RefreshCw size={14} />
          </Button>
          <Button size="sm" onPress={() => { setShowCreate(!showCreate); setEditingId(null); }}
            style={{ background: "var(--yunque-accent)", color: "#fff" }}>
            <Plus size={14} /> 新建项目
          </Button>
        </div>
      </div>

      {showCreate && (
        <Card style={{ background: "var(--yunque-card)", border: "1px solid var(--yunque-border)" }}>
          <div className="p-4 space-y-3">
            <h3 className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>新建项目</h3>
            <div className="grid grid-cols-2 gap-3">
              <label className="space-y-1">
                <span className="text-xs" style={{ color: "var(--yunque-muted)" }}>项目名称 *</span>
                <input className="w-full rounded-lg px-3 py-2 text-sm outline-none" style={{ background: "var(--yunque-bg)", color: "var(--yunque-text)", border: "1px solid var(--yunque-border)" }}
                  value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
              </label>
              <label className="space-y-1">
                <span className="text-xs" style={{ color: "var(--yunque-muted)" }}>仓库路径 *</span>
                <input className="w-full rounded-lg px-3 py-2 text-sm outline-none" style={{ background: "var(--yunque-bg)", color: "var(--yunque-text)", border: "1px solid var(--yunque-border)" }}
                  value={form.repo_path} onChange={(e) => setForm({ ...form, repo_path: e.target.value })} placeholder="C:\Code\my-project" />
              </label>
              <label className="space-y-1">
                <span className="text-xs" style={{ color: "var(--yunque-muted)" }}>Git URL (可选)</span>
                <input className="w-full rounded-lg px-3 py-2 text-sm outline-none" style={{ background: "var(--yunque-bg)", color: "var(--yunque-text)", border: "1px solid var(--yunque-border)" }}
                  value={form.repo_url} onChange={(e) => setForm({ ...form, repo_url: e.target.value })} placeholder="https://github.com/..." />
              </label>
            </div>
            <label className="space-y-1">
              <span className="text-xs" style={{ color: "var(--yunque-muted)" }}>描述 (可选)</span>
              <textarea className="w-full rounded-lg px-3 py-2 text-sm outline-none resize-none" rows={2} style={{ background: "var(--yunque-bg)", color: "var(--yunque-text)", border: "1px solid var(--yunque-border)" }}
                value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} />
            </label>
            <div className="flex gap-2 justify-end">
              <Button size="sm" variant="ghost" onPress={() => setShowCreate(false)}>取消</Button>
              <Button size="sm" onPress={handleCreate} style={{ background: "var(--yunque-accent)", color: "#fff" }}>
                创建
              </Button>
            </div>
          </div>
        </Card>
      )}

      {projects.length === 0 && !showCreate ? (
        <EmptyState
          icon={<FolderGit2 size={40} style={{ color: "var(--yunque-muted)" }} />}
          title="暂无项目"
          description="创建项目后，云雀编排守护进程可自动派发任务到对应仓库"
        />
      ) : (
        <div className="space-y-3">
          {projects.map((p) => (
            <Card key={p.id} style={{ background: "var(--yunque-card)", border: "1px solid var(--yunque-border)" }}>
              <div className="p-4">
                {editingId === p.id ? (
                  <div className="space-y-3">
                    <div className="grid grid-cols-2 gap-3">
                      <label className="space-y-1">
                        <span className="text-xs" style={{ color: "var(--yunque-muted)" }}>名称</span>
                        <input className="w-full rounded-lg px-3 py-2 text-sm outline-none" style={{ background: "var(--yunque-bg)", color: "var(--yunque-text)", border: "1px solid var(--yunque-border)" }}
                          value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
                      </label>
                      <label className="space-y-1">
                        <span className="text-xs" style={{ color: "var(--yunque-muted)" }}>路径</span>
                        <input className="w-full rounded-lg px-3 py-2 text-sm outline-none" style={{ background: "var(--yunque-bg)", color: "var(--yunque-text)", border: "1px solid var(--yunque-border)" }}
                          value={form.repo_path} onChange={(e) => setForm({ ...form, repo_path: e.target.value })} />
                      </label>
                      <label className="space-y-1">
                        <span className="text-xs" style={{ color: "var(--yunque-muted)" }}>Git URL</span>
                        <input className="w-full rounded-lg px-3 py-2 text-sm outline-none" style={{ background: "var(--yunque-bg)", color: "var(--yunque-text)", border: "1px solid var(--yunque-border)" }}
                          value={form.repo_url} onChange={(e) => setForm({ ...form, repo_url: e.target.value })} />
                      </label>
                    </div>
                    <label className="space-y-1">
                      <span className="text-xs" style={{ color: "var(--yunque-muted)" }}>描述</span>
                      <textarea className="w-full rounded-lg px-3 py-2 text-sm outline-none resize-none" rows={2} style={{ background: "var(--yunque-bg)", color: "var(--yunque-text)", border: "1px solid var(--yunque-border)" }}
                        value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} />
                    </label>
                    <div className="flex gap-2 justify-end">
                      <Button size="sm" variant="ghost" onPress={() => setEditingId(null)}><X size={14} /> 取消</Button>
                      <Button size="sm" onPress={() => handleUpdate(p.id)} style={{ background: "var(--yunque-accent)", color: "#fff" }}><Check size={14} /> 保存</Button>
                    </div>
                  </div>
                ) : (
                  <div className="flex items-start justify-between">
                    <div className="space-y-1">
                      <div className="flex items-center gap-2">
                        <FolderGit2 size={16} style={{ color: "var(--yunque-accent)" }} />
                        <span className="font-medium" style={{ color: "var(--yunque-text)" }}>{p.name}</span>
                      </div>
                      <div className="text-xs flex items-center gap-1" style={{ color: "var(--yunque-muted)" }}>
                        <GitBranch size={12} /> {p.repo_path}
                      </div>
                      {p.repo_url && (
                        <div className="text-xs" style={{ color: "var(--yunque-muted)" }}>
                          {p.repo_url}
                        </div>
                      )}
                      {p.description && (
                        <div className="text-xs mt-1" style={{ color: "var(--yunque-muted)" }}>
                          {p.description}
                        </div>
                      )}
                      {p.default_caps && p.default_caps.length > 0 && (
                        <div className="flex gap-1 mt-1 flex-wrap">
                          {p.default_caps.map((c) => (
                            <span key={c} className="text-[10px] px-1.5 py-0.5 rounded"
                              style={{ background: "rgba(99,102,241,0.15)", color: "#818cf8" }}>{c}</span>
                          ))}
                        </div>
                      )}
                    </div>
                    <div className="flex gap-1">
                      <Button isIconOnly size="sm" variant="ghost" onPress={() => startEdit(p)}>
                        <Pencil size={14} />
                      </Button>
                      <Button isIconOnly size="sm" variant="ghost" onPress={() => handleDelete(p.id)}>
                        <Trash2 size={14} style={{ color: "#ef4444" }} />
                      </Button>
                    </div>
                  </div>
                )}
              </div>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
