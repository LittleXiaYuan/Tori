/** Lightweight skillhub-updates SDK facade over the SkillHub slice. */
import {
  createSkillHubClient,
  SkillHubClient,
  SkillHubClientError,
  type SkillHubClientOptions,
  type SkillHubRollbackResponse,
  type SkillHubUpdateResponse,
  type SkillHubUpdatesResponse,
  type SkillHubVersionsResponse,
} from "./skillhub.js";

export type {
  SkillHubClientOptions as SkillHubUpdatesClientOptions,
  SkillHubRollbackResponse,
  SkillHubUpdateResponse,
  SkillHubUpdatesResponse,
  SkillHubVersionsResponse,
};

export { SkillHubClientError as SkillHubUpdatesClientError };

export class SkillHubUpdatesClient {
  private readonly client: SkillHubClient;

  constructor(options: SkillHubClientOptions) { this.client = createSkillHubClient(options); }
  checkUpdates(): Promise<SkillHubUpdatesResponse> { return this.client.checkUpdates(); }
  update(slug: string): Promise<SkillHubUpdateResponse> { return this.client.update(slug); }
  rollback(slug: string, version: string): Promise<SkillHubRollbackResponse> { return this.client.rollback(slug, version); }
  versions(slug: string): Promise<SkillHubVersionsResponse> { return this.client.versions(slug); }
}

export function createSkillHubUpdatesClient(options: SkillHubClientOptions): SkillHubUpdatesClient { return new SkillHubUpdatesClient(options); }
