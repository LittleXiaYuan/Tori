/** Lightweight market-search SDK facade over the Skill Market slice. */
import {
  SkillMarketClient,
  SkillMarketClientError,
  createSkillMarketClient,
  type SkillMarketCategory,
  type SkillMarketClientOptions,
  type SkillMarketSearchResponse,
  type SkillMarketSkill,
  type SkillMarketTopOptions,
} from "./market.js";

export type {
  SkillMarketCategory,
  SkillMarketClientOptions as SkillMarketSearchClientOptions,
  SkillMarketSearchResponse,
  SkillMarketSkill,
  SkillMarketTopOptions,
};

export { SkillMarketClientError as SkillMarketSearchClientError };

export class SkillMarketSearchClient {
  private readonly client: SkillMarketClient;

  constructor(options: SkillMarketClientOptions) {
    this.client = createSkillMarketClient(options);
  }

  search(q?: string): Promise<SkillMarketSearchResponse> {
    return this.client.search(q);
  }

  top(options: SkillMarketTopOptions = {}): Promise<SkillMarketSearchResponse> {
    return this.client.top(options);
  }
}

export function createSkillMarketSearchClient(options: SkillMarketClientOptions): SkillMarketSearchClient {
  return new SkillMarketSearchClient(options);
}
