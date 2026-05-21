/** Lightweight market-top SDK facade over the Skill Market slice. */
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
  SkillMarketClientOptions as SkillMarketTopClientOptions,
  SkillMarketSearchResponse,
  SkillMarketSkill,
  SkillMarketTopOptions,
};

export { SkillMarketClientError as SkillMarketTopClientError };

export class SkillMarketTopClient {
  private readonly client: SkillMarketClient;

  constructor(options: SkillMarketClientOptions) {
    this.client = createSkillMarketClient(options);
  }

  top(options: SkillMarketTopOptions = {}): Promise<SkillMarketSearchResponse> {
    return this.client.top(options);
  }
}

export function createSkillMarketTopClient(options: SkillMarketClientOptions): SkillMarketTopClient {
  return new SkillMarketTopClient(options);
}
