/** Lightweight plugin-knowledge-search SDK facade over the Plugin API slice. */
import {
  createPluginApiClient,
  PluginApiClient,
  PluginApiClientError,
  type PluginApiClientOptions,
  type PluginKnowledgeSearchResponse,
} from "./plugin-api.js";

export type {
  PluginApiClientOptions as PluginKnowledgeSearchClientOptions,
  PluginKnowledgeSearchResponse,
};

export { PluginApiClientError as PluginKnowledgeSearchClientError };

export class PluginKnowledgeSearchClient {
  private readonly client: PluginApiClient;

  constructor(options: PluginApiClientOptions) {
    this.client = createPluginApiClient(options);
  }

  search(query: string, limit?: number): Promise<PluginKnowledgeSearchResponse> {
    return this.client.knowledgeSearch(query, limit);
  }
}

export function createPluginKnowledgeSearchClient(options: PluginApiClientOptions): PluginKnowledgeSearchClient {
  return new PluginKnowledgeSearchClient(options);
}
