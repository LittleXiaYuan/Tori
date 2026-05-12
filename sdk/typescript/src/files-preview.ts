/** Lightweight files-preview SDK facade over artifact preview reads. */
import {
  FilesClient,
  FilesClientError,
  createFilesClient,
  type FilePreviewResponse,
  type FilesClientOptions,
} from "./files.js";

export type {
  FilePreviewResponse,
  FilesClientOptions as FilesPreviewClientOptions,
};

export { FilesClientError as FilesPreviewClientError };

export class FilesPreviewClient {
  private readonly client: FilesClient;

  constructor(options: FilesClientOptions) { this.client = createFilesClient(options); }
  preview(path: string): Promise<FilePreviewResponse> { return this.client.preview(path); }
}

export function createFilesPreviewClient(options: FilesClientOptions): FilesPreviewClient { return new FilesPreviewClient(options); }
