"use client";

import { useEffect, useMemo, useState } from "react";
import { useSearchParams } from "next/navigation";
import { Button, Card, Chip, Spinner, TextField, Input, Label } from "@heroui/react";
import {
  Boxes,
  ChevronDown,
  ChevronUp,
  Download,
  PackageCheck,
  PackageX,
  Power,
  RotateCcw,
} from "lucide-react";
import PageHeader from "@/components/page-header";
import { showToast } from "@/components/toast-provider";
import { createPacksClient, type InstalledPack } from "yunque-client/packs";
import { createYunqueSDKClientOptions } from "@/lib/sdk-client";
import { useApiData } from "@/lib/use-api-data";
import { formatErrorMessage } from "@/lib/error-utils";

const OFFICIAL_BACKUP_MANIFEST = "packs/official/backup-pack/pack.json";
const packsClient = createPacksClient(createYunqueSDKClientOptions());

function formatTime(value?: string): string {
  if (!value) return "-";
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return value;
  return d.toLocaleString();
}

function statusTone(status: string): { label: string; color: string; bg: string } {
  if (status === "enabled") return { label: "已启用", color: "var(--yunque-success)", bg: "rgba(34,197,94,0.10)" };
  if (status === "disabled") return { label: "已禁用", color: "var(--yunque-text-muted)", bg: "rgba(255,255,255,0.05)" };
  return { label: status || "未知", color: "var(--yunque-warning)", bg: "rgba(245,158,11,0.12)" };
}

function packStatusBadge(packStatus?: string): { icon: string; label: string; color: string; bg: string } {
  if (packStatus === "stable") return { icon: "✅", label: "完整可用", color: "var(--yunque-success)", bg: "rgba(34,197,94,0.10)" };
  if (packStatus === "beta") return { icon: "⚠️", label: "部分可用", color: "var(--yunque-warning)", bg: "rgba(245,158,11,0.12)" };
  if (packStatus === "alpha") return { icon: "🚧", label: "开发中", color: "var(--yunque-text-muted)", bg: "rgba(255,255,255,0.05)" };
  return { icon: "❓", label: "未知", color: "var(--yunque-text-muted)", bg: "rgba(255,255,255,0.05)" };
}

export default function PacksPageOptimized() {
  const searchParams = useSearchParams();
  const { data, loading, refresh } = useApiData(async () => packsClient.installed(), { packs: [], count: 0 });
  const [manifestPath, setManifestPath] = useState(OFFICIAL_BACKUP_MANIFEST);
  const [busy, setBusy] = useState<string | null>(null);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [showAlpha, setShowAlpha] = useState(false);

  const packs = data?.packs || [];

  // 按 Pack 状态分组
  const groupedPacks = useMemo(() => {
    const stable = packs.filter((p) => p.manifest.status === "stable");
    const beta = packs.filter((p) => p.manifest.status === "beta");
    const alpha = packs.filter((p) => p.manifest.status === "alpha");
    return { stable, beta, alpha };
  }, [packs]);

  const stats = useMemo(() => ({
    total: packs.length,
    enabled: packs.filter((p) => p.status === "enabled").length,
    disabled: packs.filter((p) => p.status === "disabled").length,
    stable: groupedPacks.stable.length,
    beta: groupedPacks.beta.length,
    alpha: groupedPacks.alpha.length,
  }), [packs, groupedPacks]);

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

  const install = () => run("install", () => packsClient.install({ manifestPath, download: false }));
  const enable = (id: string) => run(`enable:${id}`, () => packsClient.enable(id));
  const disable = (id: string) => run(`disable:${id}`, () => packsClient.disable(id));
  const rollback = (id: string) => run(`rollback:${id}`, () => packsClient.rollback(id));

  if (loading) {
    return <div className="flex items-center justify-center h-[60vh]"><Spinner size="lg" /></div>;
  }

  return (
    <div className="flex flex-col h-screen overflow-hidden">
      {/* 固定头部 */}
      <div className="flex-shrink-0 p-5 border-b" style={{ borderColor: "var(--yunque-border)" }}>
        <PageHeader
          icon={<Boxes size={20} />}
          title="能力包"
          description="管理已安装的能力包，启用或禁用功能模块"
          onRefresh={refresh}
        />

        {/* 统计卡片 */}
        <div className="grid grid-cols-3 gap-3 mt-4">
          <Card className="section-card p-4">
            <div className="kpi-label">✅ 完整可用</div>
            <div className="kpi-value">{stats.stable}</div>
          </Card>
          <Card className="section-card p-4">
            <div className="kpi-label">⚠️ 部分可用</div>
            <div className="kpi-value">{stats.beta}</div>
          </Card>
          <Card className="section-card p-4">
            <div className="kpi-label">🚧 开发中</div>
            <div className="kpi-value">{stats.alpha}</div>
          </Card>
        </div>

        {/* 显示选项 */}
        <div className="flex items-center gap-3 mt-4">
          <Button
            size="sm"
            variant="ghost"
            onPress={() => setShowAdvanced(!showAdvanced)}
          >
            {showAdvanced ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
            {showAdvanced ? "隐藏" : "显示"}技术详情
          </Button>
          <Button
            size="sm"
            variant="ghost"
            onPress={() => setShowAlpha(!showAlpha)}
          >
            {showAlpha ? "隐藏" : "显示"}开发中功能
          </Button>
        </div>
      </div>

      {/* 可滚动内容区域 */}
      <div className="flex-1 overflow-y-auto p-5 space-y-4">
        {/* 快速安装 */}
        <Card className="section-card p-4">
          <div className="flex items-center justify-between mb-3">
            <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>快速安装</div>
            <Button
              size="sm"
              variant="ghost"
              onPress={() => setShowAdvanced(!showAdvanced)}
            >
              {showAdvanced ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
              {showAdvanced ? "收起" : "高级选项"}
            </Button>
          </div>
          <div className="flex gap-3">
            <TextField value={manifestPath} onChange={(v: string) => setManifestPath(v)} className="flex-1">
              <Label>Pack 路径</Label>
              <Input placeholder={OFFICIAL_BACKUP_MANIFEST} />
            </TextField>
            <Button className="btn-accent self-end" isDisabled={!manifestPath || busy === "install"} onPress={install}>
              <Download size={14} /> 安装
            </Button>
          </div>
        </Card>

        {/* 已安装的包列表 */}
        {packs.length === 0 ? (
          <Card className="section-card p-12 text-center">
            <Boxes size={40} className="mx-auto mb-3" style={{ color: "var(--yunque-text-muted)" }} />
            <div className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>还没有安装能力包</div>
            <div className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>
              先安装一个示例包试试
            </div>
            <Button className="btn-accent mt-4" isDisabled={busy === "install"} onPress={install}>
              <Download size={14} /> 安装 backup-pack
            </Button>
          </Card>
        ) : (
          <div className="space-y-6">
            {/* 完整可用的 Pack */}
            {groupedPacks.stable.length > 0 && (
              <div>
                <div className="flex items-center gap-2 mb-3">
                  <span className="text-lg">✅</span>
                  <h3 className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
                    完整可用 ({groupedPacks.stable.length})
                  </h3>
                </div>
                <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                  {groupedPacks.stable.map((pack) => renderPackCard(pack))}
                </div>
              </div>
            )}

            {/* 部分可用的 Pack */}
            {groupedPacks.beta.length > 0 && (
              <div>
                <div className="flex items-center gap-2 mb-3">
                  <span className="text-lg">⚠️</span>
                  <h3 className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
                    部分可用 ({groupedPacks.beta.length})
                  </h3>
                </div>
                <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                  {groupedPacks.beta.map((pack) => renderPackCard(pack))}
                </div>
              </div>
            )}

            {/* 开发中的 Pack（可选显示） */}
            {showAlpha && groupedPacks.alpha.length > 0 && (
              <div>
                <div className="flex items-center gap-2 mb-3">
                  <span className="text-lg">🚧</span>
                  <h3 className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
                    开发中 ({groupedPacks.alpha.length})
                  </h3>
                  <Chip size="sm" style={{ background: "rgba(245,158,11,0.12)", color: "var(--yunque-warning)" }}>
                    仅供测试
                  </Chip>
                </div>
                <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                  {groupedPacks.alpha.map((pack) => renderPackCard(pack))}
                </div>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );

  function renderPackCard(pack: InstalledPack) {
    const tone = statusTone(pack.status);
    const packBadge = packStatusBadge(pack.manifest.status);
    const manifest = pack.manifest;
    const caps = manifest.backend?.capabilities || [];
    const menus = manifest.frontend?.menus || [];
    const metadata = manifest.metadata || {};

    // 提取用户友好的功能描述
    const friendlyExamples = [
      metadata.example1,
      metadata.example2,
      metadata.example3,
    ].filter(Boolean);

    return (
      <Card key={manifest.id} className="section-card p-4 hover-lift">
        <div className="flex items-start justify-between gap-3 mb-3">
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2 flex-wrap">
              <PackageCheck size={16} style={{ color: "var(--yunque-accent)" }} />
              <span className="font-semibold text-sm" style={{ color: "var(--yunque-text)" }}>{manifest.name}</span>
              <Chip size="sm" style={{ background: tone.bg, color: tone.color }}>{tone.label}</Chip>
              <Chip size="sm" style={{ background: packBadge.bg, color: packBadge.color }}>
                {packBadge.icon} {packBadge.label}
              </Chip>
            </div>
            <div className="text-xs mt-1 font-mono" style={{ color: "var(--yunque-text-muted)" }}>{manifest.id}</div>
            {manifest.description && (
              <div className="text-xs mt-2" style={{ color: "var(--yunque-text-secondary)" }}>{manifest.description}</div>
            )}
          </div>
        </div>

        {/* 用户友好的功能说明 */}
        {friendlyExamples.length > 0 && (
          <div className="mb-3 space-y-1">
            {friendlyExamples.map((example, idx) => (
              <div key={idx} className="flex items-start gap-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                <span style={{ color: "var(--yunque-accent)" }}>•</span>
                <span>{example}</span>
              </div>
            ))}
          </div>
        )}

        {/* 技术能力标签（仅在高级模式显示） */}
        {showAdvanced && caps.length > 0 && (
          <div className="flex flex-wrap gap-1.5 mb-3">
            {caps.slice(0, 4).map((cap) => (
              <Chip key={cap} size="sm" style={{ background: "rgba(59,130,246,0.08)", color: "var(--yunque-primary)" }}>
                {cap}
              </Chip>
            ))}
            {caps.length > 4 && (
              <Chip size="sm" style={{ background: "rgba(255,255,255,0.05)", color: "var(--yunque-text-muted)" }}>
                +{caps.length - 4}
              </Chip>
            )}
          </div>
        )}

        {/* 操作按钮 */}
        <div className="flex items-center gap-2">
          {pack.status === "enabled" ? (
            <Button size="sm" variant="outline" isDisabled={busy === `disable:${manifest.id}`} onPress={() => disable(manifest.id)}>
              <PackageX size={14} /> 禁用
            </Button>
          ) : (
            <Button size="sm" className="btn-accent" isDisabled={busy === `enable:${manifest.id}`} onPress={() => enable(manifest.id)}>
              <Power size={14} /> 启用
            </Button>
          )}
          {manifest.update?.rollback && pack.previousVersion && (
            <Button
              size="sm"
              variant="ghost"
              isDisabled={busy === `rollback:${manifest.id}`}
              onPress={() => rollback(manifest.id)}
            >
              <RotateCcw size={14} /> 回滚
            </Button>
          )}
        </div>

        {/* 展开详情 */}
        {showAdvanced && (
          <div className="mt-3 pt-3 border-t text-xs space-y-1" style={{ borderColor: "var(--yunque-border)", color: "var(--yunque-text-muted)" }}>
            <div>版本：v{manifest.version}</div>
            <div>更新时间：{formatTime(pack.updatedAt)}</div>
            {pack.previousVersion && <div>上一版本：{pack.previousVersion}</div>}
            {menus.length > 0 && (
              <div>前端入口：{menus.map(m => m.label).join(", ")}</div>
            )}
          </div>
        )}
      </Card>
    );
  }
