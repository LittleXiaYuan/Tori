/** Lightweight document-templates SDK facade over the Documents slice. */
import {
  DocumentsClient,
  DocumentsClientError,
  createDocumentsClient,
  type DocumentFormat,
  type DocumentTemplate,
  type DocumentTemplatesResponse,
  type DocumentsClientOptions,
} from "./documents.js";

export type {
  DocumentFormat,
  DocumentTemplate,
  DocumentTemplatesResponse,
  DocumentsClientOptions as DocumentTemplatesClientOptions,
};

export { DocumentsClientError as DocumentTemplatesClientError };

export class DocumentTemplatesClient {
  private readonly client: DocumentsClient;

  constructor(options: DocumentsClientOptions) {
    this.client = createDocumentsClient(options);
  }

  templates(): Promise<DocumentTemplatesResponse> {
    return this.client.templates();
  }
}

export function createDocumentTemplatesClient(options: DocumentsClientOptions): DocumentTemplatesClient {
  return new DocumentTemplatesClient(options);
}
