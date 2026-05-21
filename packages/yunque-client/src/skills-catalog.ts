/** Lightweight skills-catalog SDK facade over runtime skill catalog reads. */
import {
  SkillsClient,
  SkillsClientError,
  createSkillsClient,
  type SkillCategory,
  type SkillInfo,
  type SkillsClientOptions,
  type SkillsResponse,
} from "./skills.js";

export type {
  SkillCategory,
  SkillInfo,
  SkillsClientOptions as SkillsCatalogClientOptions,
  SkillsResponse,
};

export { SkillsClientError as SkillsCatalogClientError };

export class SkillsCatalogClient {
  private readonly client: SkillsClient;

  constructor(options: SkillsClientOptions) { this.client = createSkillsClient(options); }
  list(): Promise<SkillsResponse> { return this.client.list(); }
}

export function createSkillsCatalogClient(options: SkillsClientOptions): SkillsCatalogClient { return new SkillsCatalogClient(options); }
