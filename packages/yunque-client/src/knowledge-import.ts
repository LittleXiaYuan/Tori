/** Lightweight knowledge-import SDK facade over the Knowledge slice. */
import {
  createKnowledgeClient,
  KnowledgeClient,
  KnowledgeClientError,
  type KnowledgeClientOptions,
  type KnowledgeImportRepoRequest,
  type KnowledgeImportUrlRequest,
  type KnowledgeImportUrlResponse,
  type KnowledgeMutationResponse,
  type KnowledgeSource,
  type KnowledgeStats,
} from "./knowledge.js";

export type {
  KnowledgeClientOptions as KnowledgeImportClientOptions,
  KnowledgeImportRepoRequest,
  KnowledgeImportUrlRequest,
  KnowledgeImportUrlResponse,
  KnowledgeMutationResponse,
  KnowledgeSource,
  KnowledgeStats,
};

export { KnowledgeClientError as KnowledgeImportClientError };

export type KnowledgeImportUrlOptions = Omit<KnowledgeImportUrlRequest, "url">;
export type KnowledgeImportRepoOptions = Omit<KnowledgeImportRepoRequest, "path">;

export class KnowledgeImportClient {
  private readonly client: KnowledgeClient;

  constructor(options: KnowledgeClientOptions) {
    this.client = createKnowledgeClient(options);
  }

  importUrl(request: KnowledgeImportUrlRequest): Promise<KnowledgeImportUrlResponse> {
    return this.client.importUrl(request);
  }

  importUrlString(url: string, options: KnowledgeImportUrlOptions = {}): Promise<KnowledgeImportUrlResponse> {
    return this.client.importUrl({ ...options, url });
  }

  importRepo(request: KnowledgeImportRepoRequest): Promise<KnowledgeMutationResponse> {
    return this.client.importRepo(request);
  }

  importRepoPath(path: string, options: KnowledgeImportRepoOptions = {}): Promise<KnowledgeMutationResponse> {
    return this.client.importRepo({ ...options, path });
  }
}

export function createKnowledgeImportClient(options: KnowledgeClientOptions): KnowledgeImportClient {
  return new KnowledgeImportClient(options);
}
