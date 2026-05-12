/** Lightweight document-html SDK facade over HTML document generation. */
import {
  DocumentsClient,
  DocumentsClientError,
  createDocumentsClient,
  type DocumentGenerateRequest,
  type DocumentGenerateResponse,
  type DocumentsClientOptions,
} from "./documents.js";

export type {
  DocumentGenerateResponse,
  DocumentsClientOptions as DocumentHtmlClientOptions,
};

export type DocumentHtmlGenerateRequest = Omit<DocumentGenerateRequest, "format">;

export { DocumentsClientError as DocumentHtmlClientError };

export class DocumentHtmlClient {
  private readonly client: DocumentsClient;

  constructor(options: DocumentsClientOptions) { this.client = createDocumentsClient(options); }
  generate(body: DocumentHtmlGenerateRequest): Promise<DocumentGenerateResponse> { return this.client.generateHtml(body); }
}

export function createDocumentHtmlClient(options: DocumentsClientOptions): DocumentHtmlClient { return new DocumentHtmlClient(options); }
