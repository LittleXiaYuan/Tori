/** Lightweight market-query SDK facade over the Skill Market slice. */
import {
  SkillMarketClient,
  SkillMarketClientError,
  createSkillMarketClient,
  type SkillMarketCategory,
  type SkillMarketClientOptions,
  type SkillMarketSearchResponse,
  type SkillMarketSkill,
} from "./market.js";

export type {
  SkillMarketCategory,
  SkillMarketClientOptions as SkillMarketQueryClientOptions,
  SkillMarketSearchResponse,
  SkillMarketSkill,
};

export { SkillMarketClientError as SkillMarketQueryClientError };

export class SkillMarketQueryClient {
  private readonly client: SkillMarketClient;

  constructor(options: SkillMarketClientOptions) {
    this.client = createSkillMarketClient(options);
  }

  search(q?: string): Promise<SkillMarketSearchResponse> {
    return this.client.search(q);
  }
}

export function createSkillMarketQueryClient(options: SkillMarketClientOptions): SkillMarketQueryClient {
  return new SkillMarketQueryClient(options);
}
