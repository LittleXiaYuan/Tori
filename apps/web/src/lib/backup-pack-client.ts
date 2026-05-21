import { BASE, fetcher, getApiKey } from "./api-core";

export interface BackupInfo {
  files: Record<string, number>;
  file_count: number;
  total_bytes: number;
  version: string;
}

export interface BackupRestoreResult {
  status: string;
  files_restored: number;
  from_version: string;
  size_bytes: number;
  warning?: string;
}

export interface BackupPackClient {
  info(): Promise<BackupInfo>;
  export(): Promise<void>;
  import(file: File): Promise<BackupRestoreResult>;
}

export function createBackupPackClient(): BackupPackClient {
  return {
    info: () => fetcher<BackupInfo>("/v1/backup/info"),
    export: exportBackup,
    import: importBackup,
  };
}

async function exportBackup(): Promise<void> {
  const key = getApiKey();
  const res = await fetch(`${BASE}/v1/backup/export`, {
    headers: { ...(key ? { "X-API-Key": key } : {}) },
  });
  if (!res.ok) throw new Error(`${res.status}`);
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  const cd = res.headers.get("Content-Disposition");
  const match = cd?.match(/filename="(.+)"/);
  a.download = match?.[1] || "yunque-backup.zip";
  a.click();
  URL.revokeObjectURL(url);
}

async function importBackup(file: File): Promise<BackupRestoreResult> {
  const key = getApiKey();
  const form = new FormData();
  form.append("backup", file);
  const res = await fetch(`${BASE}/v1/backup/import`, {
    method: "POST",
    headers: { ...(key ? { "X-API-Key": key } : {}) },
    body: form,
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(`${res.status}: ${text}`);
  }
  return res.json() as Promise<BackupRestoreResult>;
}
