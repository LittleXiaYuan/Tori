/** Lightweight settings-backup SDK facade over the Settings slice. */
import {
  SettingsClient,
  SettingsClientError,
  createSettingsClient,
  type BackupExportResponse,
  type BackupImportResponse,
  type BackupInfoResponse,
  type SettingsClientOptions,
} from "./settings.js";

export type {
  BackupExportResponse,
  BackupImportResponse,
  BackupInfoResponse,
  SettingsClientOptions as SettingsBackupClientOptions,
};

export { SettingsClientError as SettingsBackupClientError };

export class SettingsBackupClient {
  private readonly client: SettingsClient;

  constructor(options: SettingsClientOptions) {
    this.client = createSettingsClient(options);
  }

  backupInfo(): Promise<BackupInfoResponse> {
    return this.client.backupInfo();
  }

  exportBackup(): Promise<BackupExportResponse> {
    return this.client.exportBackup();
  }

  importBackup(backup: Blob, filename?: string): Promise<BackupImportResponse> {
    return this.client.importBackup(backup, filename);
  }
}

export function createSettingsBackupClient(options: SettingsClientOptions): SettingsBackupClient {
  return new SettingsBackupClient(options);
}
