/** Lightweight trigger-history SDK facade over Triggers v2 runs and events. */
import {
  TriggersClient,
  TriggersClientError,
  createTriggersClient,
  type TriggerEventsResponse,
  type TriggerHistoryOptions,
  type TriggerRunsResponse,
  type TriggersClientOptions,
} from "./triggers.js";

export type {
  TriggerEventsResponse,
  TriggerHistoryOptions,
  TriggerRunsResponse,
  TriggersClientOptions as TriggerHistoryClientOptions,
};

export { TriggersClientError as TriggerHistoryClientError };

export class TriggerHistoryClient {
  private readonly client: TriggersClient;

  constructor(options: TriggersClientOptions) { this.client = createTriggersClient(options); }
  runs(options: TriggerHistoryOptions = {}): Promise<TriggerRunsResponse> { return this.client.runs(options); }
  events(options: TriggerHistoryOptions = {}): Promise<TriggerEventsResponse> { return this.client.events(options); }
}

export function createTriggerHistoryClient(options: TriggersClientOptions): TriggerHistoryClient { return new TriggerHistoryClient(options); }
