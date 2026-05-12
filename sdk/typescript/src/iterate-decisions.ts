/** Lightweight iterate-decisions SDK facade over the Iterate slice. */
import {
  IterateClient,
  IterateClientError,
  createIterateClient,
  type IterateClientOptions,
  type IterateDecisionRequest,
  type IterateDecisionResponse,
} from "./iterate.js";

export type {
  IterateClientOptions as IterateDecisionsClientOptions,
  IterateDecisionRequest,
  IterateDecisionResponse,
};

export { IterateClientError as IterateDecisionsClientError };

export class IterateDecisionsClient {
  private readonly client: IterateClient;

  constructor(options: IterateClientOptions) { this.client = createIterateClient(options); }
  approve(body: IterateDecisionRequest): Promise<IterateDecisionResponse> { return this.client.approve(body); }
  reject(body: IterateDecisionRequest): Promise<IterateDecisionResponse> { return this.client.reject(body); }
}

export function createIterateDecisionsClient(options: IterateClientOptions): IterateDecisionsClient { return new IterateDecisionsClient(options); }
