/** Lightweight document-pptx SDK facade over PPTX document generation. */
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
  DocumentsClientOptions as DocumentPptxClientOptions,
};

export type DocumentPptxGenerateRequest = Omit<DocumentGenerateRequest, "format">;

export { DocumentsClientError as DocumentPptxClientError };

export class DocumentPptxClient {
  private readonly client: DocumentsClient;

  constructor(options: DocumentsClientOptions) { this.client = createDocumentsClient(options); }
  generate(body: DocumentPptxGenerateRequest): Promise<DocumentGenerateResponse> { return this.client.generatePptx(body); }
}

export function createDocumentPptxClient(options: DocumentsClientOptions): DocumentPptxClient { return new DocumentPptxClient(options); }
