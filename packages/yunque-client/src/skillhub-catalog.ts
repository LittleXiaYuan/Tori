/** Lightweight skillhub-catalog SDK facade over the SkillHub slice. */
import {
  createSkillHubClient,
  SkillHubClient,
  SkillHubClientError,
  type SkillHubClientOptions,
  type SkillHubDetailResponse,
  type SkillHubSearchOptions,
  type SkillHubSearchResponse,
  type SkillHubSkillSummary,
  type SkillHubSource,
  type SkillHubTrendingOptions,
  type SkillHubTrendingResponse,
} from "./skillhub.js";

export type {
  SkillHubClientOptions as SkillHubCatalogClientOptions,
  SkillHubDetailResponse,
  SkillHubSearchOptions,
  SkillHubSearchResponse,
  SkillHubSkillSummary,
  SkillHubSource,
  SkillHubTrendingOptions,
  SkillHubTrendingResponse,
};

export { SkillHubClientError as SkillHubCatalogClientError };

export class SkillHubCatalogClient {
  private readonly client: SkillHubClient;

  constructor(options: SkillHubClientOptions) {
    this.client = createSkillHubClient(options);
  }

  search(options: SkillHubSearchOptions): Promise<SkillHubSearchResponse> {
    return this.client.search(options);
  }

  trending(options: SkillHubTrendingOptions = {}): Promise<SkillHubTrendingResponse> {
    return this.client.trending(options);
  }

  detail(slug: string): Promise<SkillHubDetailResponse> {
    return this.client.detail(slug);
  }
}

export function createSkillHubCatalogClient(options: SkillHubClientOptions): SkillHubCatalogClient {
  return new SkillHubCatalogClient(options);
}
