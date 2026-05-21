import {
  createPluginApiClient,
  PluginApiClient,
  PluginApiClientError,
  type PluginApiClientOptions,
  type PluginKnowledgeSearchResponse,
  type PluginOkResponse,
} from "./plugin-api.js";

export type {
  PluginApiClientOptions as PluginKnowledgeClientOptions,
  PluginKnowledgeSearchResponse,
  PluginOkResponse as PluginKnowledgeOkResponse,
};

export { PluginApiClientError as PluginKnowledgeClientError };

export class PluginKnowledgeClient {
  private readonly client: PluginApiClient;

  constructor(options: PluginApiClientOptions) {
    this.client = createPluginApiClient(options);
  }

  search(query: string, limit?: number): Promise<PluginKnowledgeSearchResponse> {
    return this.client.knowledgeSearch(query, limit);
  }

  ingest(content: string, source?: string, filename?: string): Promise<PluginOkResponse> {
    return this.client.knowledgeIngest(content, source, filename);
  }
}

export function createPluginKnowledgeClient(options: PluginApiClientOptions): PluginKnowledgeClient {
  return new PluginKnowledgeClient(options);
}
