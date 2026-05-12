/** Lightweight settings-runtime SDK facade over setup check, reload, and directory detection. */
import {
  SettingsClient,
  SettingsClientError,
  createSettingsClient,
  type SettingsCheckResponse,
  type SettingsClientOptions,
  type SettingsDetectDirsResponse,
  type SettingsReloadResponse,
} from "./settings.js";

export type {
  SettingsCheckResponse,
  SettingsClientOptions as SettingsRuntimeClientOptions,
  SettingsDetectDirsResponse,
  SettingsReloadResponse,
};

export { SettingsClientError as SettingsRuntimeClientError };

export class SettingsRuntimeClient {
  private readonly client: SettingsClient;

  constructor(options: SettingsClientOptions) { this.client = createSettingsClient(options); }
  check(): Promise<SettingsCheckResponse> { return this.client.check(); }
  reload(): Promise<SettingsReloadResponse> { return this.client.reload(); }
  detectDirs(): Promise<SettingsDetectDirsResponse> { return this.client.detectDirs(); }
}

export function createSettingsRuntimeClient(options: SettingsClientOptions): SettingsRuntimeClient { return new SettingsRuntimeClient(options); }
