/** Lightweight knowledge-upload SDK facade over the Knowledge slice. */
import {
  createKnowledgeClient,
  KnowledgeClient,
  KnowledgeClientError,
  type KnowledgeClientOptions,
  type KnowledgeMutationResponse,
  type KnowledgeSource,
  type KnowledgeStats,
  type KnowledgeUploadRequest,
} from "./knowledge.js";

export type {
  KnowledgeClientOptions as KnowledgeUploadClientOptions,
  KnowledgeMutationResponse,
  KnowledgeSource,
  KnowledgeStats,
  KnowledgeUploadRequest,
};

export { KnowledgeClientError as KnowledgeUploadClientError };

export class KnowledgeUploadClient {
  private readonly client: KnowledgeClient;

  constructor(options: KnowledgeClientOptions) {
    this.client = createKnowledgeClient(options);
  }

  upload(request: KnowledgeUploadRequest): Promise<KnowledgeMutationResponse> {
    return this.client.upload(request);
  }

  uploadFile(file: Blob, filename?: string): Promise<KnowledgeMutationResponse> {
    return this.client.upload({ file, filename });
  }
}

export function createKnowledgeUploadClient(options: KnowledgeClientOptions): KnowledgeUploadClient {
  return new KnowledgeUploadClient(options);
}
