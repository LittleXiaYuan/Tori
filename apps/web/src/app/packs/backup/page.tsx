"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Card, Button, Spinner, Chip } from "@heroui/react";
import { createBackupPackClient, type BackupInfo, type BackupRestoreResult } from "@/lib/backup-pack-client";
import {
  HardDriveDownload, HardDriveUpload, FileArchive, Download, Upload,
  CheckCircle2, AlertTriangle,
} from "lucide-react";
import PageHeader from "@/components/page-header";
import { formatErrorMessage } from "@/lib/error-utils";
import { confirmAction } from "@/components/confirm-dialog";
import { PackInfoAccordion } from "@/components/packs/pack-page-kit";

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
    const confirmed = await confirmAction({
      title: "确认导入恢复？",
      body: `将从「${file.name}」恢复数据，并覆盖当前本地记录。建议先导出当前副本；此操作完成后无法从界面直接撤销。`,
      confirmLabel: "覆盖并恢复",
      cancelLabel: "取消",
      tone: "danger",
    });
    if (!confirmed) return;
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
      <PageHeader
        icon={<FileArchive size={20} />}
        title="备份与恢复"
        actions={
          <div className="flex flex-wrap items-center gap-2">
            <Chip size="sm" color="success">可直接使用</Chip>
            <Chip size="sm" variant="soft">本地数据</Chip>
            <Chip size="sm" color="warning" variant="soft">导入会覆盖</Chip>
          </div>
        }
      />

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <Card variant="default">
          <Card.Content className="flex items-start gap-4">
            <span className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-surface-secondary text-accent">
              <HardDriveDownload size={20} />
            </span>
            <div className="min-w-0 flex-1">
              <div className="text-sm font-semibold text-foreground">导出备份</div>
              <div className="mb-4 mt-0.5 text-xs text-muted">将当前所有数据打包为 ZIP 文件下载</div>
              <Button isPending={exporting} onPress={handleExport} className="btn-accent w-full">
                <Download size={14} /> 导出备份
              </Button>
            </div>
          </Card.Content>
        </Card>

        <Card variant="default">
          <Card.Content className="flex items-start gap-4">
            <span className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-surface-secondary text-warning">
              <HardDriveUpload size={20} />
            </span>
            <div className="min-w-0 flex-1">
              <div className="text-sm font-semibold text-foreground">导入恢复</div>
              <div className="mb-4 mt-0.5 text-xs text-muted">从 ZIP 备份文件恢复数据，将覆盖当前记录</div>
              <Button isPending={importing} onPress={() => fileRef.current?.click()} variant="outline" className="w-full">
                <Upload size={14} /> 选择文件恢复
              </Button>
            </div>
          </Card.Content>
        </Card>
      </div>

      <input
        ref={fileRef}
        type="file"
        accept=".zip"
        className="hidden"
        onChange={(e) => {
          const file = e.target.files?.[0];
          if (file) void handleImport(file);
          e.currentTarget.value = "";
        }}
      />

      {error && (
        <Card variant="secondary" className="animate-scale-in">
          <Card.Content className="flex items-center gap-2 text-sm text-danger">
            <AlertTriangle size={16} /> {error}
          </Card.Content>
        </Card>
      )}
      {result && (
        <Card variant="secondary" className="animate-scale-in">
          <Card.Content className="flex items-center gap-2 text-sm text-success">
            <CheckCircle2 size={16} />
            恢复成功{result.files_restored ? ` (${result.files_restored} 条记录)` : ""}
          </Card.Content>
        </Card>
      )}

      {loading ? (
        <div className="flex items-center gap-2 text-sm text-muted">
          <Spinner size="sm" /> 加载中…
        </div>
      ) : info ? (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-3 stagger-children">
          <Card variant="default">
            <Card.Content className="flex flex-col gap-1">
              <span className="text-xs text-muted">数据文件</span>
              <span className="text-2xl font-semibold text-foreground">{info.file_count}</span>
              <span className="text-xs text-muted">个文件</span>
            </Card.Content>
          </Card>
          <Card variant="default">
            <Card.Content className="flex flex-col gap-1">
              <span className="text-xs text-muted">总计大小</span>
              <span className="text-2xl font-semibold text-foreground">{formatBytes(info.total_bytes)}</span>
            </Card.Content>
          </Card>
          <Card variant="default">
            <Card.Content className="flex flex-col gap-1">
              <span className="text-xs text-muted">Agent 版本</span>
              <span className="text-2xl font-semibold text-foreground">{info.version || "dev"}</span>
            </Card.Content>
          </Card>
        </div>
      ) : null}

      {info && info.files && Object.keys(info.files).length > 0 && (
        <Card variant="default" className="overflow-hidden">
          <div className="bg-surface-secondary px-4 py-2 text-xs font-medium text-muted">备份文件列表</div>
          <div>
            {Object.entries(info.files).sort(([a], [b]) => a.localeCompare(b)).map(([path, size]) => (
              <div key={path} className="flex items-center justify-between border-t border-separator px-4 py-2 text-xs">
                <span className="font-mono text-foreground">{path}</span>
                <span className="text-muted">{formatBytes(size as number)}</span>
              </div>
            ))}
          </div>
        </Card>
      )}

      <PackInfoAccordion
        sections={[
          {
            key: "about",
            title: "关于这个能力包",
            body: (
              <div className="flex flex-col gap-4">
                <p className="max-w-3xl leading-6">
                  它用于在升级、迁移或排障前保留一份云雀本地数据副本。当前可以查看备份范围、导出 ZIP、从 ZIP 恢复；导入恢复是破坏性动作，会覆盖当前记录，建议先导出当前状态再操作。备份包含所有对话记录、记忆数据、人格配置、知识库索引、定时任务和审计日志。
                </p>
                <div className="grid gap-3 md:grid-cols-3">
                  {userFacingSteps.map((item) => (
                    <div key={item.title} className="rounded-xl bg-surface-secondary px-4 py-3">
                      <div className="text-sm font-medium text-foreground">{item.title}</div>
                      <div className="mt-1.5 text-xs leading-5 text-muted">{item.body}</div>
                    </div>
                  ))}
                </div>
                <div className="flex flex-col gap-2">
                  <div className="text-xs font-semibold text-muted">操作前请确认</div>
                  <div className="grid gap-3 md:grid-cols-2">
                    {boundaryItems.map((item) => (
                      <div key={item} className="rounded-xl bg-surface-secondary px-4 py-3 text-sm leading-6 text-muted">{item}</div>
                    ))}
                  </div>
                </div>
              </div>
            ),
          },
        ]}
      />
    </div>
  );
}
