/** Lightweight review-gate SDK facade over the trust slice. */
import {
  createTrustClient,
  TrustClient,
  TrustClientError,
  type ReviewStatusResponse,
  type TrustClientOptions,
} from "./trust.js";

export type {
  ReviewStatusResponse,
  TrustClientOptions as ReviewClientOptions,
};

export { TrustClientError as ReviewClientError };

export class ReviewClient {
  private readonly client: TrustClient;

  constructor(options: TrustClientOptions) {
    this.client = createTrustClient(options);
  }

  status(): Promise<ReviewStatusResponse> {
    return this.client.reviewStatus();
  }
}

export function createReviewClient(options: TrustClientOptions): ReviewClient {
  return new ReviewClient(options);
}
