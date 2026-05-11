/** Lightweight skill-growth SDK facade over the trust slice. */
import {
  createTrustClient,
  TrustClient,
  TrustClientError,
  type SkillGrowPattern,
  type SkillGrowPatternsResponse,
  type TrustClientOptions,
} from "./trust.js";

export type {
  SkillGrowPattern,
  SkillGrowPatternsResponse,
  TrustClientOptions as SkillGrowClientOptions,
};

export { TrustClientError as SkillGrowClientError };

export class SkillGrowClient {
  private readonly client: TrustClient;

  constructor(options: TrustClientOptions) {
    this.client = createTrustClient(options);
  }

  patterns(): Promise<SkillGrowPatternsResponse> {
    return this.client.skillGrowPatterns();
  }
}

export function createSkillGrowClient(options: TrustClientOptions): SkillGrowClient {
  return new SkillGrowClient(options);
}
