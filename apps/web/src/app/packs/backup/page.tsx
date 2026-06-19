"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Card, Button, Spinner, Chip, Tooltip } from "@heroui/react";
import { createBackupPackClient, type BackupInfo, type BackupRestoreResult } from "@/lib/backup-pack-client";
import {
  HardDriveDownload, HardDriveUpload, FileArchive, Download, Upload,
  CheckCircle2, AlertTriangle, Loader2, Info,
} from "lucide-react";
import PageHeader from "@/components/page-header";
import { formatErrorMessage } from "@/lib/error-utils";

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + sizes[i];
}

const backupPack = createBackupPackClient();

const userFacingSteps = [
  {
    title: "1. 先看备份范围",
    body: "确认会打包哪些本地数据、文件数量和当前版本。",
  },
  {
    title: "2. 导出可回滚副本",
    body: "把当前云雀数据下载成 ZIP，适合迁移、升级前留底或排障交接。",
  },
  {
    title: "3. 谨慎导入恢复",
    body: "选择旧备份覆盖当前记录，恢复后再检查对话、记忆和任务结果。",
  },
];

const boundaryItems = [
  "导入恢复会覆盖当前记录，请先导出当前副本。",
  "不会把备份自动上传到云端。",
  "不会校验第三方 ZIP 的可信来源。",
  "不会只恢复单个模块，当前按备份包整体恢复。",
];

export default function BackupPage() {
  const [info, setInfo] = useState<BackupInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [exporting, setExporting] = useState(false);
  const [importing, setImporting] = useState(false);
  const [result, setResult] = useState<BackupRestoreResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const fileRef = useRef<HTMLInputElement>(null);

  const fetchInfo = useCallback(async () => {
    try {
      setLoading(true);
      const data = await backupPack.info();
      setInfo(data);
    } catch (e: unknown) {
      setError(formatErrorMessage(e, "加载备份信息失败"));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { fetchInfo(); }, [fetchInfo]);

  const handleExport = async () => {
    try {
      setExporting(true);
      setError(null);
      setResult(null);
      await backupPack.export();
    } catch (e: unknown) {
      setError(formatErrorMessage(e, "导出备份失败"));
    } finally {
      setExporting(false);
    }
  };

  const handleImport = async (file: File) => {
    try {
      setImporting(true);
      setError(null);
      setResult(null);
      const res = await backupPack.import(file);
      setResult(res);
      fetchInfo();
    } catch (e: unknown) {
      setError(formatErrorMessage(e, "导入备份失败"));
    } finally {
      setImporting(false);
    }
  };

  return (
    <div className="page-root space-y-6 animate-fade-in-up">
      <PageHeader icon={<FileArchive size={20} />} title="备份与恢复" />

      <Card className="section-card overflow-hidden p-0">
        <div className="grid gap-0 lg:grid-cols-[minmax(0,1fr)_320px]">
          <div className="p-5">
            <div className="flex flex-wrap items-center gap-2">
              <Chip size="sm" style={{ background: "rgba(34,197,94,0.10)", color: "var(--yunque-success)" }}>可直接使用</Chip>
              <Chip size="sm" variant="soft">本地数据</Chip>
              <Chip size="sm" variant="soft">导入会覆盖</Chip>
            </div>
            <div className="mt-3 text-base font-semibold" style={{ color: "var(--yunque-text)" }}>
              这个能力包现在适合做什么
            </div>
            <div className="mt-2 max-w-3xl text-sm leading-6" style={{ color: "var(--yunque-text-secondary)" }}>
              它用于在升级、迁移或排障前保留一份云雀本地数据副本。当前可以查看备份范围、导出 ZIP、从 ZIP 恢复；导入恢复是破坏性动作，会覆盖当前记录，建议先导出当前状态再操作。
            </div>
            <div className="mt-4 grid gap-3 md:grid-cols-3">
              {userFacingSteps.map((item) => (
                <div key={item.title} className="rounded-lg p-3" style={{ background: "var(--yunque-bg-hover)", border: "1px solid var(--yunque-border)" }}>
                  <div className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{item.title}</div>
                  <div className="mt-2 text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>{item.body}</div>
                </div>
              ))}
            </div>
          </div>
          <div className="p-5" style={{ background: "rgba(245,158,11,0.08)", borderLeft: "1px solid var(--yunque-border)" }}>
            <div className="mb-3 text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>操作前请确认</div>
            <div className="space-y-2 text-xs leading-5" style={{ color: "var(--yunque-text-secondary)" }}>
              {boundaryItems.map((item) => <div key={item}>{item}</div>)}
            </div>
          </div>
        </div>
      </Card>

      {/* Info banner */}
      <Card className="section-card p-4 flex items-start gap-3">
        <Info size={16} className="mt-0.5 shrink-0" style={{ color: "var(--yunque-text-muted)" }} />
        <div className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>
          {"备份包含所有对话记录、记忆数据、人格配置、知识库索引、定时任务和审计日志。导出为 ZIP 文件，可随时恢复。"}
        </div>
      </Card>

      {/* Stats */}
      {loading ? (
        <div className="flex items-center gap-2 text-sm" style={{ color: "var(--yunque-text-muted)" }}>
          <Spinner size="sm" /> {"加载中…"}
        </div>
      ) : info ? (
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 stagger-children">
          <Card className="section-card p-4 hover-lift">
            <div className="kpi-label">{"数据文件"}</div>
            <div className="kpi-value">{info.file_count}</div>
            <div className="kpi-sub">{"个文件"}</div>
          </Card>
          <Card className="section-card p-4 hover-lift">
            <div className="kpi-label">{"总计大小"}</div>
            <div className="kpi-value">{formatBytes(info.total_bytes)}</div>
          </Card>
          <Card className="section-card p-4 hover-lift">
            <div className="kpi-label">Agent {"版本"}</div>
            <div className="kpi-value">{info.version || "dev"}</div>
          </Card>
        </div>
      ) : null}

      {/* File list */}
      {info && info.files && Object.keys(info.files).length > 0 && (
        <Card className="section-card overflow-hidden">
          <div className="px-4 py-2 text-xs font-medium" style={{ color: "var(--yunque-text-muted)", background: "rgba(255,255,255,0.02)" }}>
            {"备份文件列表"}
          </div>
          <div className="divide-y" style={{ borderColor: "var(--yunque-border)" }}>
            {Object.entries(info.files).sort(([a], [b]) => a.localeCompare(b)).map(([path, size]) => (
              <div key={path} className="flex items-center justify-between px-4 py-2 text-xs">
                <span className="font-mono" style={{ color: "var(--yunque-text)" }}>{path}</span>
                <span style={{ color: "var(--yunque-text-muted)" }}>{formatBytes(size as number)}</span>
              </div>
            ))}
          </div>
        </Card>
      )}

      {/* Actions */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <Card className="section-card p-5 hover-lift">
          <div className="flex items-start gap-4">
            <div className="w-10 h-10 rounded-xl flex items-center justify-center shrink-0" style={{ background: "rgba(0,111,238,0.1)" }}>
              <HardDriveDownload size={20} style={{ color: "var(--yunque-accent)" }} />
            </div>
            <div className="flex-1 min-w-0">
              <div className="text-sm font-semibold mb-0.5" style={{ color: "var(--yunque-text)" }}>{"导出备份"}</div>
              <div className="text-xs mb-4" style={{ color: "var(--yunque-text-muted)" }}>{"将当前所有数据打包为 ZIP 文件下载"}</div>
              <Button
                isPending={exporting}
                onPress={handleExport}
                className="w-full btn-accent"
              >
                <Download size={14} /> {"导出备份"}
              </Button>
            </div>
          </div>
        </Card>

        <Card className="section-card p-5 hover-lift">
          <div className="flex items-start gap-4">
            <div className="w-10 h-10 rounded-xl flex items-center justify-center shrink-0" style={{ background: "rgba(245,158,11,0.1)" }}>
              <HardDriveUpload size={20} style={{ color: "var(--yunque-warning)" }} />
            </div>
            <div className="flex-1 min-w-0">
              <div className="text-sm font-semibold mb-0.5" style={{ color: "var(--yunque-text)" }}>{"导入恢复"}</div>
              <div className="text-xs mb-4" style={{ color: "var(--yunque-text-muted)" }}>{"从 ZIP 备份文件恢复数据，将覆盖当前记录"}</div>
              <Button
                isPending={importing}
                onPress={() => fileRef.current?.click()}
                className="w-full"
                style={{ border: "1px solid var(--yunque-border)", color: "var(--yunque-text)" }}
              >
                <Upload size={14} /> {"选择文件恢复"}
              </Button>
            </div>
          </div>
        </Card>
      </div>

        <input
          ref={fileRef}
          type="file"
          accept=".zip"
          className="hidden"
          onChange={(e) => {
            const file = e.target.files?.[0];
            if (file) handleImport(file);
          }}
        />

      {/* Error / Result */}
      {error && (
        <Card className="p-4 animate-scale-in" style={{ background: "rgba(239,68,68,0.05)" }}>
          <div className="flex items-center gap-2">
            <AlertTriangle size={16} style={{ color: "var(--yunque-danger)" }} />
            <span className="text-sm" style={{ color: "var(--yunque-danger)" }}>{error}</span>
          </div>
        </Card>
      )}
      {result && (
        <Card className="p-4 animate-scale-in" style={{ background: "rgba(34,197,94,0.05)" }}>
          <div className="flex items-center gap-2">
            <CheckCircle2 size={16} style={{ color: "var(--yunque-success)" }} />
            <span className="text-sm" style={{ color: "var(--yunque-success)" }}>
              {"恢复成功"}{result.files_restored ? ` (${result.files_restored} 条记录)` : ""}
            </span>
          </div>
        </Card>
      )}
    </div>
  );
}
