/** Lightweight triggers-control SDK facade over the Triggers v2 slice. */
import {
  TriggersClient,
  TriggersClientError,
  createTriggersClient,
  type TriggerDef,
  type TriggerDeleteResponse,
  type TriggerEmitResponse,
  type TriggerPayload,
  type TriggersClientOptions,
} from "./triggers.js";

export type {
  TriggerDef,
  TriggerDeleteResponse,
  TriggerEmitResponse,
  TriggerPayload,
  TriggersClientOptions as TriggersControlClientOptions,
};

export { TriggersClientError as TriggersControlClientError };

export class TriggersControlClient {
  private readonly client: TriggersClient;

  constructor(options: TriggersClientOptions) { this.client = createTriggersClient(options); }
  create(trigger: TriggerDef): Promise<TriggerDef> { return this.client.create(trigger); }
  update(trigger: TriggerDef): Promise<TriggerDef> { return this.client.update(trigger); }
  delete(id: string): Promise<TriggerDeleteResponse> { return this.client.delete(id); }
  emit(payload: TriggerPayload): Promise<TriggerEmitResponse> { return this.client.emit(payload); }
}

export function createTriggersControlClient(options: TriggersClientOptions): TriggersControlClient { return new TriggersControlClient(options); }
