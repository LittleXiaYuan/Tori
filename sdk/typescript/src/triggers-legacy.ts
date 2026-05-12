/** Lightweight triggers-legacy SDK facade over the Triggers slice. */
import {
  TriggersClient,
  TriggersClientError,
  createTriggersClient,
  type TriggerDeleteResponse,
  type TriggerEmitResponse,
  type TriggerLegacy,
  type TriggerListResponse,
  type TriggerPayload,
  type TriggersClientOptions,
} from "./triggers.js";

export type {
  TriggerDeleteResponse,
  TriggerEmitResponse,
  TriggerLegacy,
  TriggerListResponse,
  TriggerPayload,
  TriggersClientOptions as TriggersLegacyClientOptions,
};

export { TriggersClientError as TriggersLegacyClientError };

export class TriggersLegacyClient {
  private readonly client: TriggersClient;

  constructor(options: TriggersClientOptions) { this.client = createTriggersClient(options); }
  list(): Promise<TriggerListResponse<TriggerLegacy>> { return this.client.listLegacy(); }
  get(id: string): Promise<TriggerLegacy> { return this.client.getLegacy(id); }
  create(trigger: TriggerLegacy): Promise<TriggerLegacy> { return this.client.createLegacy(trigger); }
  delete(id: string): Promise<TriggerDeleteResponse> { return this.client.deleteLegacy(id); }
  emit(payload: TriggerPayload): Promise<TriggerEmitResponse> { return this.client.emitLegacy(payload); }
}

export function createTriggersLegacyClient(options: TriggersClientOptions): TriggersLegacyClient { return new TriggersLegacyClient(options); }
