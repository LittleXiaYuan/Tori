/** Lightweight document-xlsx SDK facade over XLSX document generation. */
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
  DocumentsClientOptions as DocumentXlsxClientOptions,
};

export type DocumentXlsxGenerateRequest = Omit<DocumentGenerateRequest, "format">;

export { DocumentsClientError as DocumentXlsxClientError };

export class DocumentXlsxClient {
  private readonly client: DocumentsClient;

  constructor(options: DocumentsClientOptions) { this.client = createDocumentsClient(options); }
  generate(body: DocumentXlsxGenerateRequest): Promise<DocumentGenerateResponse> { return this.client.generateXlsx(body); }
}

export function createDocumentXlsxClient(options: DocumentsClientOptions): DocumentXlsxClient { return new DocumentXlsxClient(options); }
