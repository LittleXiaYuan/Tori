/** Lightweight document-generate SDK facade over the Documents slice. */
import {
  DocumentsClient,
  DocumentsClientError,
  createDocumentsClient,
  type DocumentFormat,
  type DocumentGenerateRequest,
  type DocumentGenerateResponse,
  type DocumentsClientOptions,
} from "./documents.js";

export type {
  DocumentFormat,
  DocumentGenerateRequest,
  DocumentGenerateResponse,
  DocumentsClientOptions as DocumentGenerateClientOptions,
};

export { DocumentsClientError as DocumentGenerateClientError };

export class DocumentGenerateClient {
  private readonly client: DocumentsClient;

  constructor(options: DocumentsClientOptions) {
    this.client = createDocumentsClient(options);
  }

  generate(body: DocumentGenerateRequest): Promise<DocumentGenerateResponse> {
    return this.client.generate(body);
  }

  generateDocx(body: Omit<DocumentGenerateRequest, "format">): Promise<DocumentGenerateResponse> {
    return this.client.generateDocx(body);
  }

  generateXlsx(body: Omit<DocumentGenerateRequest, "format">): Promise<DocumentGenerateResponse> {
    return this.client.generateXlsx(body);
  }

  generatePptx(body: Omit<DocumentGenerateRequest, "format">): Promise<DocumentGenerateResponse> {
    return this.client.generatePptx(body);
  }

  generateHtml(body: Omit<DocumentGenerateRequest, "format">): Promise<DocumentGenerateResponse> {
    return this.client.generateHtml(body);
  }
}

export function createDocumentGenerateClient(options: DocumentsClientOptions): DocumentGenerateClient {
  return new DocumentGenerateClient(options);
}
