/** Lightweight plugin-knowledge-ingest SDK facade over the Plugin API slice. */
import {
  createPluginApiClient,
  PluginApiClient,
  PluginApiClientError,
  type PluginApiClientOptions,
  type PluginOkResponse,
} from "./plugin-api.js";

export type {
  PluginApiClientOptions as PluginKnowledgeIngestClientOptions,
  PluginOkResponse as PluginKnowledgeIngestOkResponse,
};

export { PluginApiClientError as PluginKnowledgeIngestClientError };

export class PluginKnowledgeIngestClient {
  private readonly client: PluginApiClient;

  constructor(options: PluginApiClientOptions) {
    this.client = createPluginApiClient(options);
  }

  ingest(content: string, source?: string, filename?: string): Promise<PluginOkResponse> {
    return this.client.knowledgeIngest(content, source, filename);
  }
}

export function createPluginKnowledgeIngestClient(options: PluginApiClientOptions): PluginKnowledgeIngestClient {
  return new PluginKnowledgeIngestClient(options);
}
