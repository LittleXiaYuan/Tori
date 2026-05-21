/** Lightweight files-list SDK facade over artifact file listing. */
import {
  FilesClient,
  FilesClientError,
  createFilesClient,
  type FileEntry,
  type FileListResponse,
  type FilesClientOptions,
} from "./files.js";

export type {
  FileEntry,
  FileListResponse,
  FilesClientOptions as FilesListClientOptions,
};

export { FilesClientError as FilesListClientError };

export class FilesListClient {
  private readonly client: FilesClient;

  constructor(options: FilesClientOptions) { this.client = createFilesClient(options); }
  list(path?: string): Promise<FileListResponse> { return this.client.list(path); }
}

export function createFilesListClient(options: FilesClientOptions): FilesListClient { return new FilesListClient(options); }
