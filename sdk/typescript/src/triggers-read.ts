/** Lightweight triggers-read SDK facade over the Triggers v2 slice. */
import {
  TriggersClient,
  TriggersClientError,
  createTriggersClient,
  type TriggerDef,
  type TriggerEventsResponse,
  type TriggerHistoryOptions,
  type TriggerListResponse,
  type TriggerRunsResponse,
  type TriggerV2ListOptions,
  type TriggersClientOptions,
} from "./triggers.js";

export type {
  TriggerDef,
  TriggerEventsResponse,
  TriggerHistoryOptions,
  TriggerListResponse,
  TriggerRunsResponse,
  TriggerV2ListOptions,
  TriggersClientOptions as TriggersReadClientOptions,
};

export { TriggersClientError as TriggersReadClientError };

export class TriggersReadClient {
  private readonly client: TriggersClient;

  constructor(options: TriggersClientOptions) { this.client = createTriggersClient(options); }
  list(options: TriggerV2ListOptions = {}): Promise<TriggerListResponse<TriggerDef>> { return this.client.list(options); }
  get(id: string): Promise<TriggerDef> { return this.client.get(id); }
  runs(options: TriggerHistoryOptions = {}): Promise<TriggerRunsResponse> { return this.client.runs(options); }
  events(options: TriggerHistoryOptions = {}): Promise<TriggerEventsResponse> { return this.client.events(options); }
}

export function createTriggersReadClient(options: TriggersClientOptions): TriggersReadClient { return new TriggersReadClient(options); }
