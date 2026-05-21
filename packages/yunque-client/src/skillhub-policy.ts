/** Lightweight skillhub-policy SDK facade over the SkillHub slice. */
import {
  createSkillHubClient,
  SkillHubClient,
  SkillHubClientError,
  type SkillHubAnalyticsResponse,
  type SkillHubClientOptions,
  type SkillHubPolicyCheckResponse,
  type SkillHubPolicyResponse,
  type SkillHubPolicyUpdateResponse,
} from "./skillhub.js";

export type {
  SkillHubAnalyticsResponse,
  SkillHubClientOptions as SkillHubPolicyClientOptions,
  SkillHubPolicyCheckResponse,
  SkillHubPolicyResponse,
  SkillHubPolicyUpdateResponse,
};

export { SkillHubClientError as SkillHubPolicyClientError };

export class SkillHubPolicyClient {
  private readonly client: SkillHubClient;

  constructor(options: SkillHubClientOptions) {
    this.client = createSkillHubClient(options);
  }

  policy(): Promise<SkillHubPolicyResponse> {
    return this.client.policy();
  }

  updatePolicy(policy: Record<string, unknown>): Promise<SkillHubPolicyUpdateResponse> {
    return this.client.updatePolicy(policy);
  }

  check(slug: string): Promise<SkillHubPolicyCheckResponse> {
    return this.client.policyCheck(slug);
  }

  analytics(): Promise<SkillHubAnalyticsResponse> {
    return this.client.analytics();
  }
}

export function createSkillHubPolicyClient(options: SkillHubClientOptions): SkillHubPolicyClient {
  return new SkillHubPolicyClient(options);
}
