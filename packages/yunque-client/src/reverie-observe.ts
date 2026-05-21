/** Lightweight reverie-observe SDK facade over the Reverie slice. */
import {
  ReverieClient,
  ReverieClientError,
  createReverieClient,
  type ReverieActionsResponse,
  type ReverieClientOptions,
  type ReverieConfigResponse,
  type ReverieJournalQuery,
  type ReverieJournalResponse,
  type ReverieTargetsResponse,
} from "./reverie.js";

export type {
  ReverieActionsResponse,
  ReverieClientOptions as ReverieObserveClientOptions,
  ReverieConfigResponse,
  ReverieJournalQuery,
  ReverieJournalResponse,
  ReverieTargetsResponse,
};

export { ReverieClientError as ReverieObserveClientError };

export class ReverieObserveClient {
  private readonly client: ReverieClient;

  constructor(options: ReverieClientOptions) {
    this.client = createReverieClient(options);
  }

  journal(query?: ReverieJournalQuery): Promise<ReverieJournalResponse> {
    return this.client.journal(query);
  }

  stats(): Promise<Record<string, unknown>> {
    return this.client.stats();
  }

  config(): Promise<ReverieConfigResponse> {
    return this.client.config();
  }

  actions(): Promise<ReverieActionsResponse> {
    return this.client.actions();
  }

  targets(): Promise<ReverieTargetsResponse> {
    return this.client.targets();
  }
}

export function createReverieObserveClient(options: ReverieClientOptions): ReverieObserveClient {
  return new ReverieObserveClient(options);
}
