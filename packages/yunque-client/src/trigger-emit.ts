/** Lightweight trigger-emit SDK facade over Triggers v2 event emission. */
import {
  TriggersClient,
  TriggersClientError,
  createTriggersClient,
  type TriggerEmitResponse,
  type TriggerPayload,
  type TriggersClientOptions,
} from "./triggers.js";

export type {
  TriggerEmitResponse,
  TriggerPayload,
  TriggersClientOptions as TriggerEmitClientOptions,
};

export { TriggersClientError as TriggerEmitClientError };

export class TriggerEmitClient {
  private readonly client: TriggersClient;

  constructor(options: TriggersClientOptions) { this.client = createTriggersClient(options); }
  emit(payload: TriggerPayload): Promise<TriggerEmitResponse> { return this.client.emit(payload); }
}

export function createTriggerEmitClient(options: TriggersClientOptions): TriggerEmitClient { return new TriggerEmitClient(options); }
