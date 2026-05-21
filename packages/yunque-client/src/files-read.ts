/** Lightweight files-read SDK facade over the Files slice. */
import {
  FilesClient,
  FilesClientError,
  createFilesClient,
  type FileEntry,
  type FileListResponse,
  type FilePreviewResponse,
  type FilesClientOptions,
} from "./files.js";

export type {
  FileEntry,
  FileListResponse,
  FilePreviewResponse,
  FilesClientOptions as FilesReadClientOptions,
};

export { FilesClientError as FilesReadClientError };

export class FilesReadClient {
  private readonly client: FilesClient;

  constructor(options: FilesClientOptions) {
    this.client = createFilesClient(options);
  }

  list(path?: string): Promise<FileListResponse> {
    return this.client.list(path);
  }

  preview(path: string): Promise<FilePreviewResponse> {
    return this.client.preview(path);
  }
}

export function createFilesReadClient(options: FilesClientOptions): FilesReadClient {
  return new FilesReadClient(options);
}
