/** Lightweight document-docx SDK facade over DOCX document generation. */
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
  DocumentsClientOptions as DocumentDocxClientOptions,
};

export type DocumentDocxGenerateRequest = Omit<DocumentGenerateRequest, "format">;

export { DocumentsClientError as DocumentDocxClientError };

export class DocumentDocxClient {
  private readonly client: DocumentsClient;

  constructor(options: DocumentsClientOptions) { this.client = createDocumentsClient(options); }
  generate(body: DocumentDocxGenerateRequest): Promise<DocumentGenerateResponse> { return this.client.generateDocx(body); }
}

export function createDocumentDocxClient(options: DocumentsClientOptions): DocumentDocxClient { return new DocumentDocxClient(options); }
