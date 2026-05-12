/** Lightweight market-stats SDK facade over the Skill Market slice. */
import {
  SkillMarketClient,
  SkillMarketClientError,
  createSkillMarketClient,
  type SkillMarketClientOptions,
  type SkillMarketStatsResponse,
} from "./market.js";

export type {
  SkillMarketClientOptions as SkillMarketStatsClientOptions,
  SkillMarketStatsResponse,
};

export { SkillMarketClientError as SkillMarketStatsClientError };

export class SkillMarketStatsClient {
  private readonly client: SkillMarketClient;

  constructor(options: SkillMarketClientOptions) {
    this.client = createSkillMarketClient(options);
  }

  stats(): Promise<SkillMarketStatsResponse> {
    return this.client.stats();
  }
}

export function createSkillMarketStatsClient(options: SkillMarketClientOptions): SkillMarketStatsClient {
  return new SkillMarketStatsClient(options);
}
