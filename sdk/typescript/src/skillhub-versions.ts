/** Lightweight skillhub-versions SDK facade over SkillHub version history and rollback. */
import {
  createSkillHubClient,
  SkillHubClient,
  SkillHubClientError,
  type SkillHubClientOptions,
  type SkillHubRollbackResponse,
  type SkillHubVersionsResponse,
} from "./skillhub.js";

export type {
  SkillHubClientOptions as SkillHubVersionsClientOptions,
  SkillHubRollbackResponse,
  SkillHubVersionsResponse,
};

export { SkillHubClientError as SkillHubVersionsClientError };

export class SkillHubVersionsClient {
  private readonly client: SkillHubClient;

  constructor(options: SkillHubClientOptions) { this.client = createSkillHubClient(options); }
  list(slug: string): Promise<SkillHubVersionsResponse> { return this.client.versions(slug); }
  rollback(slug: string, version: string): Promise<SkillHubRollbackResponse> { return this.client.rollback(slug, version); }
}

export function createSkillHubVersionsClient(options: SkillHubClientOptions): SkillHubVersionsClient { return new SkillHubVersionsClient(options); }
