/** Lightweight settings-config SDK facade over the Settings slice. */
import {
  SettingsClient,
  SettingsClientError,
  createSettingsClient,
  type SettingsCheckResponse,
  type SettingsClientOptions,
  type SettingsConfigResponse,
  type SettingsDetectDirsResponse,
  type SettingsReloadResponse,
  type SettingsSchemaResponse,
  type SettingsUpdateResponse,
} from "./settings.js";

export type {
  SettingsCheckResponse,
  SettingsClientOptions as SettingsConfigClientOptions,
  SettingsConfigResponse,
  SettingsDetectDirsResponse,
  SettingsReloadResponse,
  SettingsSchemaResponse,
  SettingsUpdateResponse,
};

export { SettingsClientError as SettingsConfigClientError };

export class SettingsConfigClient {
  private readonly client: SettingsClient;

  constructor(options: SettingsClientOptions) {
    this.client = createSettingsClient(options);
  }

  schema(): Promise<SettingsSchemaResponse> {
    return this.client.schema();
  }

  config(): Promise<SettingsConfigResponse> {
    return this.client.config();
  }

  updateConfig(values: Record<string, string>): Promise<SettingsUpdateResponse> {
    return this.client.updateConfig(values);
  }

  check(): Promise<SettingsCheckResponse> {
    return this.client.check();
  }

  reload(): Promise<SettingsReloadResponse> {
    return this.client.reload();
  }

  detectDirs(): Promise<SettingsDetectDirsResponse> {
    return this.client.detectDirs();
  }
}

export function createSettingsConfigClient(options: SettingsClientOptions): SettingsConfigClient {
  return new SettingsConfigClient(options);
}
