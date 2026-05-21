/** Lightweight trust-control SDK facade over the Trust slice. */
import {
  TrustClient,
  TrustClientError,
  createTrustClient,
  type TrustClientOptions,
  type TrustGrantResponse,
  type TrustResetResponse,
  type TrustScoresResponse,
  type TrustSlugRequest,
} from "./trust.js";

export type {
  TrustClientOptions as TrustControlClientOptions,
  TrustGrantResponse,
  TrustResetResponse,
  TrustScoresResponse,
  TrustSlugRequest,
};

export { TrustClientError as TrustControlClientError };

export class TrustControlClient {
  private readonly client: TrustClient;

  constructor(options: TrustClientOptions) {
    this.client = createTrustClient(options);
  }

  scores(): Promise<TrustScoresResponse> {
    return this.client.scores();
  }

  reset(body: TrustSlugRequest): Promise<TrustResetResponse> {
    return this.client.reset(body);
  }

  grant(body: TrustSlugRequest): Promise<TrustGrantResponse> {
    return this.client.grant(body);
  }

  grantAll(): Promise<TrustGrantResponse> {
    return this.client.grantAll();
  }
}

export function createTrustControlClient(options: TrustClientOptions): TrustControlClient {
  return new TrustControlClient(options);
}
