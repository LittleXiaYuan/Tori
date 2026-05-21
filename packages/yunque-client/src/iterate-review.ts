/** Lightweight iterate-review SDK facade over the Iterate slice. */
import {
  IterateClient,
  IterateClientError,
  createIterateClient,
  type IterateClientOptions,
  type IterateDecisionRequest,
  type IterateDecisionResponse,
  type IterateProposalsQuery,
  type IterateProposalsResponse,
} from "./iterate.js";

export type {
  IterateClientOptions as IterateReviewClientOptions,
  IterateDecisionRequest,
  IterateDecisionResponse,
  IterateProposalsQuery,
  IterateProposalsResponse,
};

export { IterateClientError as IterateReviewClientError };

export class IterateReviewClient {
  private readonly client: IterateClient;

  constructor(options: IterateClientOptions) {
    this.client = createIterateClient(options);
  }

  proposals(query?: IterateProposalsQuery): Promise<IterateProposalsResponse> {
    return this.client.proposals(query);
  }

  pendingProposals(): Promise<IterateProposalsResponse> {
    return this.client.pendingProposals();
  }

  approve(body: IterateDecisionRequest): Promise<IterateDecisionResponse> {
    return this.client.approve(body);
  }

  reject(body: IterateDecisionRequest): Promise<IterateDecisionResponse> {
    return this.client.reject(body);
  }
}

export function createIterateReviewClient(options: IterateClientOptions): IterateReviewClient {
  return new IterateReviewClient(options);
}
