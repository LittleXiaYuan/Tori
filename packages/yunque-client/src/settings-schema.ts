/** Lightweight settings-schema SDK facade over settings schema reads. */
import {
  SettingsClient,
  SettingsClientError,
  createSettingsClient,
  type SettingsClientOptions,
  type SettingsSchemaField,
  type SettingsSchemaGroup,
  type SettingsSchemaResponse,
} from "./settings.js";

export type {
  SettingsClientOptions as SettingsSchemaClientOptions,
  SettingsSchemaField,
  SettingsSchemaGroup,
  SettingsSchemaResponse,
};

export { SettingsClientError as SettingsSchemaClientError };

export class SettingsSchemaClient {
  private readonly client: SettingsClient;

  constructor(options: SettingsClientOptions) { this.client = createSettingsClient(options); }
  schema(): Promise<SettingsSchemaResponse> { return this.client.schema(); }
}

export function createSettingsSchemaClient(options: SettingsClientOptions): SettingsSchemaClient { return new SettingsSchemaClient(options); }
