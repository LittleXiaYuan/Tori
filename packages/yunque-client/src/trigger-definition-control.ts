/** Lightweight trigger-definition-control SDK facade over Triggers v2 definition mutations. */
import {
  TriggersClient,
  TriggersClientError,
  createTriggersClient,
  type TriggerDef,
  type TriggerDeleteResponse,
  type TriggersClientOptions,
} from "./triggers.js";

export type {
  TriggerDef,
  TriggerDeleteResponse,
  TriggersClientOptions as TriggerDefinitionControlClientOptions,
};

export { TriggersClientError as TriggerDefinitionControlClientError };

export class TriggerDefinitionControlClient {
  private readonly client: TriggersClient;

  constructor(options: TriggersClientOptions) { this.client = createTriggersClient(options); }
  create(trigger: TriggerDef): Promise<TriggerDef> { return this.client.create(trigger); }
  update(trigger: TriggerDef): Promise<TriggerDef> { return this.client.update(trigger); }
  delete(id: string): Promise<TriggerDeleteResponse> { return this.client.delete(id); }
}

export function createTriggerDefinitionControlClient(options: TriggersClientOptions): TriggerDefinitionControlClient { return new TriggerDefinitionControlClient(options); }
