/** Lightweight skillhub-installed SDK facade over installed SkillHub skills. */
import {
  createSkillHubClient,
  SkillHubClient,
  SkillHubClientError,
  type SkillHubClientOptions,
  type SkillHubInstalledResponse,
  type SkillHubInstalledSkill,
} from "./skillhub.js";

export type {
  SkillHubClientOptions as SkillHubInstalledClientOptions,
  SkillHubInstalledResponse,
  SkillHubInstalledSkill,
};

export { SkillHubClientError as SkillHubInstalledClientError };

export class SkillHubInstalledClient {
  private readonly client: SkillHubClient;

  constructor(options: SkillHubClientOptions) { this.client = createSkillHubClient(options); }
  list(): Promise<SkillHubInstalledResponse> { return this.client.installed(); }
}

export function createSkillHubInstalledClient(options: SkillHubClientOptions): SkillHubInstalledClient { return new SkillHubInstalledClient(options); }
