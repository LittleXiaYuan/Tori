/** Lightweight trigger-definitions SDK facade over Triggers v2 definition reads. */
import {
  TriggersClient,
  TriggersClientError,
  createTriggersClient,
  type TriggerDef,
  type TriggerListResponse,
  type TriggerV2ListOptions,
  type TriggersClientOptions,
} from "./triggers.js";

export type {
  TriggerDef,
  TriggerListResponse,
  TriggerV2ListOptions,
  TriggersClientOptions as TriggerDefinitionsClientOptions,
};

export { TriggersClientError as TriggerDefinitionsClientError };

export class TriggerDefinitionsClient {
  private readonly client: TriggersClient;

  constructor(options: TriggersClientOptions) { this.client = createTriggersClient(options); }
  list(options: TriggerV2ListOptions = {}): Promise<TriggerListResponse<TriggerDef>> { return this.client.list(options); }
  get(id: string): Promise<TriggerDef> { return this.client.get(id); }
}

export function createTriggerDefinitionsClient(options: TriggersClientOptions): TriggerDefinitionsClient { return new TriggerDefinitionsClient(options); }
