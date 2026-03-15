"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import {
  HardDriveDownload,
  HardDriveUpload,
  FileArchive,
  Download,
  Upload,
  CheckCircle2,
  AlertTriangle,
  Loader2,
  Info,
} from "lucide-react";
import { api, BackupInfo, BackupRestoreResult } from "@/lib/api";

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + sizes[i];
}

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
      const data = await api.backupInfo();
      setInfo(data);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "Failed to load backup info");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchInfo();
  }, [fetchInfo]);

  const handleExport = async () => {
    try {
      setExporting(true);
      setError(null);
      setResult(null);
      await api.backupExport();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "Export failed");
    } finally {
      setExporting(false);
    }
  };

  const handleImport = async (file: File) => {
    try {
      setImporting(true);
      setError(null);
      setResult(null);
      const res = await api.backupImport(file);
      setResult(res);
      fetchInfo();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "Import failed");
    } finally {
      setImporting(false);
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <FileArchive size={22} style={{ color: "var(--text-muted)" }} />
        <h1 className="text-xl font-semibold">备份与恢复</h1>
      </div>

      {/* Info banner */}
      <div
        className="rounded-lg border p-4 text-sm flex items-start gap-3"
        style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}
      >
        <Info size={16} className="mt-0.5 shrink-0" style={{ color: "var(--text-muted)" }} />
        <div style={{ color: "var(--text-muted)" }}>
          备份包含所有对话记录、记忆数据、人格配置、知识库索引、定时任务和审计日志。
          导出为 ZIP 文件，可随时恢复。
        </div>
      </div>

      {/* Stats cards */}
      {loading ? (
        <div className="flex items-center gap-2 text-sm" style={{ color: "var(--text-muted)" }}>
          <Loader2 size={14} className="animate-spin" /> 加载中…
        </div>
      ) : info ? (
        <div className="grid grid-cols-3 gap-4">
          <StatCard label="数据文件" value={String(info.file_count)} sub="个文件" />
          <StatCard label="总计大小" value={formatBytes(info.total_bytes)} sub="" />
          <StatCard label="Agent 版本" value={info.version || "dev"} sub="" />
        </div>
      ) : null}

      {/* File list */}
      {info && Object.keys(info.files).length > 0 && (
        <div
          className="rounded-lg border overflow-hidden"
          style={{ borderColor: "var(--border)" }}
        >
          <div
            className="px-4 py-2 text-xs font-medium"
            style={{ color: "var(--text-muted)", background: "var(--bg-card)" }}
          >
            备份文件列表
          </div>
          <div className="divide-y" style={{ borderColor: "var(--border)" }}>
            {Object.entries(info.files)
              .sort(([a], [b]) => a.localeCompare(b))
              .map(([path, size]) => (
                <div
                  key={path}
                  className="flex items-center justify-between px-4 py-2 text-xs"
                  style={{ borderColor: "var(--border)" }}
                >
                  <span className="font-mono" style={{ color: "var(--text)" }}>
                    {path}
                  </span>
                  <span style={{ color: "var(--text-muted)" }}>{formatBytes(size)}</span>
                </div>
              ))}
          </div>
        </div>
      )}

      {/* Action buttons */}
      <div className="flex gap-4">
        <button
          onClick={handleExport}
          disabled={exporting || loading}
          className="flex items-center gap-2 px-5 py-2.5 rounded-lg text-sm font-medium transition-all"
          style={{
            background: "var(--text)",
            color: "var(--bg)",
            opacity: exporting ? 0.6 : 1,
          }}
        >
          {exporting ? (
            <Loader2 size={14} className="animate-spin" />
          ) : (
            <Download size={14} />
          )}
          {exporting ? "导出中…" : "导出备份"}
        </button>

        <button
          onClick={() => fileRef.current?.click()}
          disabled={importing}
          className="flex items-center gap-2 px-5 py-2.5 rounded-lg text-sm font-medium border transition-all"
          style={{
            borderColor: "var(--border)",
            color: "var(--text)",
            opacity: importing ? 0.6 : 1,
          }}
        >
          {importing ? (
            <Loader2 size={14} className="animate-spin" />
          ) : (
            <Upload size={14} />
          )}
          {importing ? "恢复中…" : "导入恢复"}
        </button>

        <input
          ref={fileRef}
          type="file"
          accept=".zip"
          className="hidden"
          onChange={(e) => {
            const f = e.target.files?.[0];
            if (f) handleImport(f);
            e.target.value = "";
          }}
        />
      </div>

      {/* Result / Error */}
      {result && (
        <div
          className="rounded-lg border p-4 flex items-start gap-3"
          style={{
            borderColor: result.warning ? "#f59e0b" : "#22c55e",
            background: result.warning ? "rgba(245,158,11,0.08)" : "rgba(34,197,94,0.08)",
          }}
        >
          {result.warning ? (
            <AlertTriangle size={16} className="mt-0.5 shrink-0" style={{ color: "#f59e0b" }} />
          ) : (
            <CheckCircle2 size={16} className="mt-0.5 shrink-0" style={{ color: "#22c55e" }} />
          )}
          <div className="text-sm space-y-1">
            <div>
              恢复完成：已恢复 <strong>{result.files_restored}</strong> 个文件
              （{formatBytes(result.size_bytes)}），
              来自版本 <strong>{result.from_version}</strong>
            </div>
            {result.warning && (
              <div style={{ color: "#f59e0b" }}>{result.warning}</div>
            )}
            <div className="text-xs" style={{ color: "var(--text-muted)" }}>
              建议重启 Agent 以使恢复的数据生效。
            </div>
          </div>
        </div>
      )}

      {error && (
        <div
          className="rounded-lg border p-4 flex items-start gap-3 text-sm"
          style={{ borderColor: "#ef4444", background: "rgba(239,68,68,0.08)", color: "#ef4444" }}
        >
          <AlertTriangle size={16} className="mt-0.5 shrink-0" />
          {error}
        </div>
      )}
    </div>
  );
}

function StatCard({ label, value, sub }: { label: string; value: string; sub: string }) {
  return (
    <div
      className="rounded-lg border p-4"
      style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}
    >
      <div className="text-xs mb-1" style={{ color: "var(--text-muted)" }}>
        {label}
      </div>
      <div className="text-lg font-semibold" style={{ color: "var(--text)" }}>
        {value}
        {sub && (
          <span className="text-xs font-normal ml-1" style={{ color: "var(--text-muted)" }}>
            {sub}
          </span>
        )}
      </div>
    </div>
  );
}
