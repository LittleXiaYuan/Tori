/** Lightweight iterate-pending SDK facade over the Iterate slice. */
import {
  IterateClient,
  IterateClientError,
  createIterateClient,
  type IterateClientOptions,
  type IterateProposalsResponse,
} from "./iterate.js";

export type {
  IterateClientOptions as IteratePendingClientOptions,
  IterateProposalsResponse,
};

export { IterateClientError as IteratePendingClientError };

export class IteratePendingClient {
  private readonly client: IterateClient;

  constructor(options: IterateClientOptions) { this.client = createIterateClient(options); }
  list(): Promise<IterateProposalsResponse> { return this.client.pendingProposals(); }
}

export function createIteratePendingClient(options: IterateClientOptions): IteratePendingClient { return new IteratePendingClient(options); }
