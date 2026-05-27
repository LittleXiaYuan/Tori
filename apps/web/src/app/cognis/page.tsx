"use client";

import { useEffect, useMemo, useState } from "react";
import { useSearchParams } from "next/navigation";
import { Button, Card, Chip, Spinner, Input, Label, TextField } from "@heroui/react";
import {
  Brain,
  ChevronDown,
  ChevronUp,
  Download,
  Upload,
  Power,
  PowerOff,
  Search,
  Sparkles,
  FileJson,
  Check,
  X,
  AlertCircle,
  Share,
  Link,
  Copy,
} from "lucide-react";
import PageHeader from "@/components/page-header";
import { showToast } from "@/components/toast-provider";
import { createCognisClient, type CogniDeclaration } from "yunque-client/cognis";
import { createYunqueSDKClientOptions } from "@/lib/sdk-client";
import { useApiData } from "@/lib/use-api-data";
import { formatErrorMessage } from "@/lib/error-utils";
import { CherryModal } from "@/components/cherry/overlay";

const cognisClient = createCognisClient(createYunqueSDKClientOptions());

interface CogniWithHealth extends CogniDeclaration {
  health?: {
    status?: string;
    activation_count?: number;
    success_rate?: number;
  };
}

interface TemplateMetadata {
  id: string;
  display_name: string;
  description: string;
  category: string;
  tags: string[];
  isBuiltin: boolean;
}

const BUILTIN_TEMPLATES: TemplateMetadata[] = [
  {
    id: "code-reviewer",
    display_name: "代码审查助手",
    description: "自动审查代码质量、发现潜在问题、提供改进建议",
    category: "开发工具",
    tags: ["代码", "审查", "质量"],
    isBuiltin: true,
  },
  {
    id: "data-analyst",
    display_name: "数据分析助手",
    description: "专业的数据分析助手，帮助用户分析数据、生成报表、可视化结果",
    category: "数据分析",
    tags: ["数据", "分析", "报表"],
    isBuiltin: true,
  },
  {
    id: "doc-generator",
    display_name: "文档生成器",
    description: "自动生成技术文档、API 文档、用户手册",
    category: "文档工具",
    tags: ["文档", "生成", "API"],
    isBuiltin: true,
  },
  {
    id: "monitor-alert",
    display_name: "监控告警助手",
    description: "监控系统状态、分析日志、发送告警通知",
    category: "运维工具",
    tags: ["监控", "告警", "日志"],
    isBuiltin: true,
  },
  {
    id: "task-scheduler",
    display_name: "任务调度器",
    description: "智能任务调度、优先级管理、自动化执行",
    category: "效率工具",
    tags: ["任务", "调度", "自动化"],
    isBuiltin: true,
  },
];

function statusTone(enabled?: boolean): { label: string; color: string; bg: string } {
  if (enabled) return { label: "已启用", color: "var(--yunque-success)", bg: "rgba(34,197,94,0.10)" };
  return { label: "已禁用", color: "var(--yunque-text-muted)", bg: "rgba(255,255,255,0.05)" };
}

function healthBadge(health?: { status?: string; success_rate?: number }): { icon: string; label: string; color: string; bg: string } {
  if (!health || health.status === "idle") return { icon: "⚪", label: "未激活", color: "var(--yunque-text-muted)", bg: "rgba(255,255,255,0.05)" };
  if (health.status === "healthy" || (health.success_rate && health.success_rate >= 0.8)) return { icon: "✅", label: "健康", color: "var(--yunque-success)", bg: "rgba(34,197,94,0.10)" };
  if (health.status === "degraded" || (health.success_rate && health.success_rate >= 0.5)) return { icon: "⚠️", label: "降级", color: "var(--yunque-warning)", bg: "rgba(245,158,11,0.12)" };
  return { icon: "❌", label: "异常", color: "var(--yunque-error)", bg: "rgba(239,68,68,0.12)" };
}

export default function CognisPage() {
  const searchParams = useSearchParams();
  const { data, loading, refresh } = useApiData(async () => cognisClient.list(), { cognis: [], count: 0 });
  const [searchQuery, setSearchQuery] = useState("");
  const [selectedCategory, setSelectedCategory] = useState<string>("all");
  const [busy, setBusy] = useState<string | null>(null);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [importFile, setImportFile] = useState<File | null>(null);
  const [shareModalOpen, setShareModalOpen] = useState(false);
  const [shareModalCogniId, setShareModalCogniId] = useState<string | null>(null);
  const [importPreviewOpen, setImportPreviewOpen] = useState(false);
  const [importPreviewBundle, setImportPreviewBundle] = useState<any>(null);
  const [dragOver, setDragOver] = useState(false);

  const cognis = (data?.cognis || data?.items || []) as CogniWithHealth[];
  const healthMap = (data as any)?.health || {};

  // 检测 URL 中的导入链接
  useEffect(() => {
    const bundleParam = searchParams.get("bundle");
    if (bundleParam) {
      try {
        const decoded = atob(bundleParam);
        const bundle = JSON.parse(decoded);
        setImportPreviewBundle(bundle);
        setImportPreviewOpen(true);
      } catch (e) {
        showToast("无效的分享链接", "error");
      }
    }
  }, [searchParams]);

  // 合并健康数据
  const cognisWithHealth = useMemo(() => {
    return cognis.map((c) => ({
      ...c,
      health: healthMap[c.id || ""],
    }));
  }, [cognis, healthMap]);

  // 分类统计
  const stats = useMemo(() => {
    const installed = cognisWithHealth.filter((c) => !BUILTIN_TEMPLATES.find((t) => t.id === c.id));
    const enabled = cognisWithHealth.filter((c) => c.enabled);
    const healthy = cognisWithHealth.filter((c) => c.health?.status === "healthy" || (c.health?.success_rate && c.health.success_rate >= 0.8));

    return {
      total: cognisWithHealth.length,
      builtin: BUILTIN_TEMPLATES.filter((t) => cognisWithHealth.find((c) => c.id === t.id)).length,
      installed: installed.length,
      enabled: enabled.length,
      healthy: healthy.length,
    };
  }, [cognisWithHealth]);

  // 获取所有可用模板（内置 + 已安装）
  const allTemplates = useMemo(() => {
    const installedIds = new Set(cognisWithHealth.map((c) => c.id));
    const templates: Array<TemplateMetadata & { installed?: boolean; cogni?: CogniWithHealth }> = [];

    // 添加内置模板
    BUILTIN_TEMPLATES.forEach((template) => {
      const cogni = cognisWithHealth.find((c) => c.id === template.id);
      templates.push({
        ...template,
        installed: installedIds.has(template.id),
        cogni,
      });
    });

    // 添加用户创建的 Cogni
    cognisWithHealth.forEach((cogni) => {
      if (!BUILTIN_TEMPLATES.find((t) => t.id === cogni.id)) {
        templates.push({
          id: cogni.id || "",
          display_name: (cogni as any).display_name || cogni.name || cogni.id || "未命名",
          description: cogni.description || "用户创建的 Cogni",
          category: "用户创建",
          tags: [],
          isBuiltin: false,
          installed: true,
          cogni,
        });
      }
    });

    return templates;
  }, [cognisWithHealth]);

  // 搜索和过滤
  const filteredTemplates = useMemo(() => {
    let filtered = allTemplates;

    // 分类过滤
    if (selectedCategory !== "all") {
      if (selectedCategory === "builtin") {
        filtered = filtered.filter((t) => t.isBuiltin);
      } else if (selectedCategory === "user") {
        filtered = filtered.filter((t) => !t.isBuiltin);
      } else if (selectedCategory === "installed") {
        filtered = filtered.filter((t) => t.installed);
      } else {
        filtered = filtered.filter((t) => t.category === selectedCategory);
      }
    }

    // 搜索过滤
    if (searchQuery.trim()) {
      const query = searchQuery.toLowerCase();
      filtered = filtered.filter(
        (t) =>
          t.display_name.toLowerCase().includes(query) ||
          t.description.toLowerCase().includes(query) ||
          t.tags.some((tag) => tag.toLowerCase().includes(query))
      );
    }

    return filtered;
  }, [allTemplates, selectedCategory, searchQuery]);

  // 分组显示
  const groupedTemplates = useMemo(() => {
    const builtin = filteredTemplates.filter((t) => t.isBuiltin);
    const user = filteredTemplates.filter((t) => !t.isBuiltin);
    return { builtin, user };
  }, [filteredTemplates]);

  const run = async (label: string, op: () => Promise<unknown>) => {
    setBusy(label);
    try {
      await op();
      showToast("操作成功", "success");
      await refresh();
    } catch (e) {
      showToast(formatErrorMessage(e, "操作失败"), "error");
    } finally {
      setBusy(null);
    }
  };

  const installTemplate = async (templateId: string) => {
    try {
      const response = await fetch(`/data/cogni/templates/${templateId}.json`);
      if (!response.ok) throw new Error("模板文件不存在");
      const declaration = await response.json();
      await run(`install:${templateId}`, () => cognisClient.create(declaration));
    } catch (e) {
      showToast(formatErrorMessage(e, "安装失败"), "error");
    }
  };

  const enable = (id: string) => run(`enable:${id}`, () => cognisClient.enable(id));
  const disable = (id: string) => run(`disable:${id}`, () => cognisClient.disable(id));
  const remove = (id: string) => run(`remove:${id}`, () => cognisClient.remove(id));

  const handleExport = async () => {
    try {
      const bundle = await cognisClient.exportBundle();
      const blob = new Blob([JSON.stringify(bundle, null, 2)], { type: "application/json" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `cogni-bundle-${new Date().toISOString().split("T")[0]}.json`;
      a.click();
      URL.revokeObjectURL(url);
      showToast("导出成功", "success");
    } catch (e) {
      showToast(formatErrorMessage(e, "导出失败"), "error");
    }
  };

  const handleImport = async () => {
    if (!importFile) return;
    try {
      const text = await importFile.text();
      const bundle = JSON.parse(text);
      await run("import", () => cognisClient.importBundle(bundle));
      setImportFile(null);
    } catch (e) {
      showToast(formatErrorMessage(e, "导入失败"), "error");
    }
  };

  const handleShareCogni = async (cogniId: string) => {
    setShareModalCogniId(cogniId);
    setShareModalOpen(true);
  };

  const handleCopyShareLink = async () => {
    if (!shareModalCogniId) return;
    try {
      const bundle = await cognisClient.exportBundle([shareModalCogniId]);
      const encoded = btoa(JSON.stringify(bundle));
      const shareUrl = `${window.location.origin}/cognis?bundle=${encoded}`;
      await navigator.clipboard.writeText(shareUrl);
      showToast("分享链接已复制到剪贴板", "success");
      setShareModalOpen(false);
    } catch (e) {
      showToast(formatErrorMessage(e, "生成分享链接失败"), "error");
    }
  };

  const handleDownloadCogni = async () => {
    if (!shareModalCogniId) return;
    try {
      const bundle = await cognisClient.exportBundle([shareModalCogniId]);
      const blob = new Blob([JSON.stringify(bundle, null, 2)], { type: "application/json" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `cogni-${shareModalCogniId}-${new Date().toISOString().split("T")[0]}.json`;
      a.click();
      URL.revokeObjectURL(url);
      showToast("下载成功", "success");
      setShareModalOpen(false);
    } catch (e) {
      showToast(formatErrorMessage(e, "下载失败"), "error");
    }
  };

  const handleImportFromPreview = async () => {
    if (!importPreviewBundle) return;
    try {
      await run("import", () => cognisClient.importBundle(importPreviewBundle));
      setImportPreviewOpen(false);
      setImportPreviewBundle(null);
      // 清除 URL 参数
      window.history.replaceState({}, "", "/cognis");
    } catch (e) {
      showToast(formatErrorMessage(e, "导入失败"), "error");
    }
  };

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    setDragOver(true);
  };

  const handleDragLeave = (e: React.DragEvent) => {
    e.preventDefault();
    setDragOver(false);
  };

  const handleDrop = async (e: React.DragEvent) => {
    e.preventDefault();
    setDragOver(false);
    const files = e.dataTransfer.files;
    if (files.length > 0) {
      const file = files[0];
      if (file.type === "application/json" || file.name.endsWith(".json")) {
        try {
          const text = await file.text();
          const bundle = JSON.parse(text);
          setImportPreviewBundle(bundle);
          setImportPreviewOpen(true);
        } catch (e) {
          showToast("无效的 JSON 文件", "error");
        }
      } else {
        showToast("请拖放 JSON 文件", "error");
      }
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-[60vh]">
        <Spinner size="lg" />
      </div>
    );
  }

  return (
    <div className="flex flex-col h-screen overflow-hidden">
      {/* 固定头部 */}
      <div className="flex-shrink-0 p-5 border-b" style={{ borderColor: "var(--yunque-border)" }}>
        <PageHeader
          icon={<Brain size={20} />}
          title="Cogni 市场"
          description="浏览、安装和管理 AI 认知配置模板"
          onRefresh={refresh}
        />

        {/* 统计卡片 */}
        <div className="grid grid-cols-4 gap-3 mt-4">
          <Card className="section-card p-4">
            <div className="kpi-label">📦 内置模板</div>
            <div className="kpi-value">{stats.builtin}</div>
          </Card>
          <Card className="section-card p-4">
            <div className="kpi-label">⚡ 已安装</div>
            <div className="kpi-value">{stats.installed}</div>
          </Card>
          <Card className="section-card p-4">
            <div className="kpi-label">✅ 已启用</div>
            <div className="kpi-value">{stats.enabled}</div>
          </Card>
          <Card className="section-card p-4">
            <div className="kpi-label">💚 健康</div>
            <div className="kpi-value">{stats.healthy}</div>
          </Card>
        </div>

        {/* 搜索和过滤 */}
        <div className="flex items-center gap-3 mt-4">
          <TextField value={searchQuery} onChange={(v: string) => setSearchQuery(v)} className="flex-1">
            <Label>搜索 Cogni</Label>
            <Input placeholder="搜索名称、描述或标签..." />
          </TextField>
          <div className="flex gap-2">
            <Button
              size="sm"
              variant={selectedCategory === "all" ? "primary" : "ghost"}
              onPress={() => setSelectedCategory("all")}
            >
              全部
            </Button>
            <Button
              size="sm"
              variant={selectedCategory === "builtin" ? "primary" : "ghost"}
              onPress={() => setSelectedCategory("builtin")}
            >
              内置
            </Button>
            <Button
              size="sm"
              variant={selectedCategory === "user" ? "primary" : "ghost"}
              onPress={() => setSelectedCategory("user")}
            >
              用户
            </Button>
            <Button
              size="sm"
              variant={selectedCategory === "installed" ? "primary" : "ghost"}
              onPress={() => setSelectedCategory("installed")}
            >
              已安装
            </Button>
          </div>
        </div>

        {/* 导入导出 */}
        <div className="flex items-center gap-3 mt-4">
          <Button size="sm" variant="ghost" onPress={handleExport}>
            <Download size={14} /> 导出全部
          </Button>
          <div className="flex items-center gap-2">
            <input
              type="file"
              accept=".json"
              onChange={(e) => setImportFile(e.target.files?.[0] || null)}
              className="hidden"
              id="import-file"
            />
            <label htmlFor="import-file" style={{ cursor: "pointer" }}>
              <Button size="sm" variant="ghost" onPress={() => document.getElementById("import-file")?.click()}>
                <Upload size={14} /> 选择文件
              </Button>
            </label>
            {importFile && (
              <>
                <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                  {importFile.name}
                </span>
                <Button size="sm" className="btn-accent" onPress={handleImport} isDisabled={busy === "import"}>
                  <FileJson size={14} /> 导入
                </Button>
              </>
            )}
          </div>
          <Button
            size="sm"
            variant="ghost"
            onPress={() => setShowAdvanced(!showAdvanced)}
          >
            {showAdvanced ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
            {showAdvanced ? "隐藏" : "显示"}详情
          </Button>
        </div>
      </div>

      {/* 可滚动内容区域 */}
      <div
        className="flex-1 overflow-y-auto p-5 space-y-6"
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
        onDrop={handleDrop}
      >
        {/* 拖放提示 */}
        {dragOver && (
          <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
            <Card className="section-card p-8 text-center">
              <Upload size={48} className="mx-auto mb-3" style={{ color: "var(--yunque-accent)" }} />
              <div className="text-lg font-semibold" style={{ color: "var(--yunque-text)" }}>
                拖放 JSON 文件以导入
              </div>
            </Card>
          </div>
        )}

        {filteredTemplates.length === 0 ? (
          <Card className="section-card p-12 text-center">
            <Brain size={40} className="mx-auto mb-3" style={{ color: "var(--yunque-text-muted)" }} />
            <div className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>
              没有找到匹配的 Cogni
            </div>
            <div className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>
              尝试调整搜索条件或分类筛选
            </div>
          </Card>
        ) : (
          <>
            {/* 内置模板 */}
            {groupedTemplates.builtin.length > 0 && (
              <div>
                <div className="flex items-center gap-2 mb-3">
                  <span className="text-lg">📦</span>
                  <h3 className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
                    内置模板 ({groupedTemplates.builtin.length})
                  </h3>
                </div>
                <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                  {groupedTemplates.builtin.map((template) => renderTemplateCard(template))}
                </div>
              </div>
            )}

            {/* 用户创建 */}
            {groupedTemplates.user.length > 0 && (
              <div>
                <div className="flex items-center gap-2 mb-3">
                  <span className="text-lg">👤</span>
                  <h3 className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
                    用户创建 ({groupedTemplates.user.length})
                  </h3>
                </div>
                <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                  {groupedTemplates.user.map((template) => renderTemplateCard(template))}
                </div>
              </div>
            )}
          </>
        )}
      </div>

      {/* 分享对话框 */}
      <CherryModal
        open={shareModalOpen}
        onClose={() => setShareModalOpen(false)}
        size="md"
        ariaLabel="分享 Cogni"
        header={
          <div className="flex items-center gap-2">
            <Share size={18} style={{ color: "var(--yunque-accent)" }} />
            <span>分享 Cogni</span>
          </div>
        }
      >
        <div className="space-y-3">
          <div className="text-sm" style={{ color: "var(--yunque-text-secondary)" }}>
            选择分享方式：
          </div>
          <Button
            className="w-full justify-start"
            variant="outline"
            onPress={handleCopyShareLink}
          >
            <Link size={16} />
            <span className="flex-1 text-left">复制分享链接</span>
          </Button>
          <Button
            className="w-full justify-start"
            variant="outline"
            onPress={handleDownloadCogni}
          >
            <Download size={16} />
            <span className="flex-1 text-left">下载为文件</span>
          </Button>
          <div className="flex justify-end gap-2 mt-4 pt-3 border-t" style={{ borderColor: "var(--yunque-border)" }}>
            <Button size="sm" variant="ghost" onPress={() => setShareModalOpen(false)}>
              取消
            </Button>
          </div>
        </div>
      </CherryModal>

      {/* 导入预览对话框 */}
      <CherryModal
        open={importPreviewOpen}
        onClose={() => setImportPreviewOpen(false)}
        size="lg"
        ariaLabel="导入预览"
        header={
          <div className="flex items-center gap-2">
            <FileJson size={18} style={{ color: "var(--yunque-accent)" }} />
            <span>导入预览</span>
          </div>
        }
      >
        {importPreviewBundle && (
          <div className="space-y-3">
            <div className="text-sm" style={{ color: "var(--yunque-text-secondary)" }}>
              将要导入以下 Cogni：
            </div>
            <div className="space-y-2 max-h-96 overflow-y-auto">
              {(importPreviewBundle.cognis || []).map((cogni: any, index: number) => (
                <Card key={index} className="section-card p-3">
                  <div className="flex items-start gap-2">
                    <Brain size={16} style={{ color: "var(--yunque-accent)" }} />
                    <div className="flex-1 min-w-0">
                      <div className="font-semibold text-sm" style={{ color: "var(--yunque-text)" }}>
                        {cogni.display_name || cogni.name || cogni.id || "未命名"}
                      </div>
                      <div className="text-xs font-mono mt-1" style={{ color: "var(--yunque-text-muted)" }}>
                        {cogni.id}
                      </div>
                      {cogni.description && (
                        <div className="text-xs mt-1" style={{ color: "var(--yunque-text-secondary)" }}>
                          {cogni.description}
                        </div>
                      )}
                    </div>
                  </div>
                </Card>
              ))}
            </div>
            {importPreviewBundle.notes && (
              <div className="text-xs p-2 rounded" style={{ background: "rgba(255,255,255,0.05)", color: "var(--yunque-text-muted)" }}>
                <div className="font-semibold mb-1">备注：</div>
                {importPreviewBundle.notes}
              </div>
            )}
            <div className="flex justify-end gap-2 mt-4 pt-3 border-t" style={{ borderColor: "var(--yunque-border)" }}>
              <Button size="sm" variant="ghost" onPress={() => setImportPreviewOpen(false)}>
                取消
              </Button>
              <Button size="sm" className="btn-accent" onPress={handleImportFromPreview}>
                <Check size={14} /> 确认导入
              </Button>
            </div>
          </div>
        )}
      </CherryModal>
    </div>
  );

  function renderTemplateCard(template: TemplateMetadata & { installed?: boolean; cogni?: CogniWithHealth }) {
    const { installed, cogni } = template;
    const tone = statusTone(cogni?.enabled);
    const health = healthBadge(cogni?.health);

    return (
      <Card key={template.id} className="section-card p-4 hover-lift">
        <div className="flex items-start justify-between gap-3 mb-3">
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2 flex-wrap">
              <Brain size={16} style={{ color: "var(--yunque-accent)" }} />
              <span className="font-semibold text-sm" style={{ color: "var(--yunque-text)" }}>
                {template.display_name}
              </span>
              {installed && <Chip size="sm" style={{ background: tone.bg, color: tone.color }}>{tone.label}</Chip>}
              {installed && cogni?.health && (
                <Chip size="sm" style={{ background: health.bg, color: health.color }}>
                  {health.icon} {health.label}
                </Chip>
              )}
            </div>
            <div className="text-xs mt-1 font-mono" style={{ color: "var(--yunque-text-muted)" }}>
              {template.id}
            </div>
            <div className="text-xs mt-2" style={{ color: "var(--yunque-text-secondary)" }}>
              {template.description}
            </div>
          </div>
        </div>

        {/* 分类和标签 */}
        <div className="flex flex-wrap gap-1.5 mb-3">
          <Chip size="sm" style={{ background: "rgba(59,130,246,0.08)", color: "var(--yunque-primary)" }}>
            {template.category}
          </Chip>
          {template.tags.slice(0, 3).map((tag) => (
            <Chip key={tag} size="sm" style={{ background: "rgba(255,255,255,0.05)", color: "var(--yunque-text-muted)" }}>
              {tag}
            </Chip>
          ))}
        </div>

        {/* 高级信息 */}
        {showAdvanced && installed && cogni && (
          <div className="mb-3 pt-3 border-t text-xs space-y-1" style={{ borderColor: "var(--yunque-border)", color: "var(--yunque-text-muted)" }}>
            {cogni.health?.activation_count !== undefined && (
              <div>激活次数：{cogni.health.activation_count}</div>
            )}
            {cogni.health?.success_rate !== undefined && (
              <div>成功率：{(cogni.health.success_rate * 100).toFixed(1)}%</div>
            )}
            {(cogni as any).priority !== undefined && (
              <div>优先级：{(cogni as any).priority}</div>
            )}
          </div>
        )}

        {/* 操作按钮 */}
        <div className="flex items-center gap-2">
          {!installed ? (
            <Button
              size="sm"
              className="btn-accent"
              isDisabled={busy === `install:${template.id}`}
              onPress={() => installTemplate(template.id)}
            >
              <Download size={14} /> 安装
            </Button>
          ) : (
            <>
              {cogni?.enabled ? (
                <Button
                  size="sm"
                  variant="outline"
                  isDisabled={busy === `disable:${template.id}`}
                  onPress={() => disable(template.id)}
                >
                  <PowerOff size={14} /> 禁用
                </Button>
              ) : (
                <Button
                  size="sm"
                  className="btn-accent"
                  isDisabled={busy === `enable:${template.id}`}
                  onPress={() => enable(template.id)}
                >
                  <Power size={14} /> 启用
                </Button>
              )}
              <Button
                size="sm"
                variant="ghost"
                onPress={() => handleShareCogni(template.id)}
              >
                <Share size={14} /> 分享
              </Button>
              {!template.isBuiltin && (
                <Button
                  size="sm"
                  variant="ghost"
                  isDisabled={busy === `remove:${template.id}`}
                  onPress={() => remove(template.id)}
                >
                  <X size={14} /> 删除
                </Button>
              )}
            </>
          )}
        </div>
      </Card>
    );
  }
}
