/** Lightweight files-download SDK facade over the Files slice. */
import {
  FilesClient,
  FilesClientError,
  createFilesClient,
  type FileDownloadResponse,
  type FilesClientOptions,
} from "./files.js";

export type {
  FileDownloadResponse,
  FilesClientOptions as FilesDownloadClientOptions,
};

export { FilesClientError as FilesDownloadClientError };

export class FilesDownloadClient {
  private readonly client: FilesClient;

  constructor(options: FilesClientOptions) {
    this.client = createFilesClient(options);
  }

  download(path: string): Promise<FileDownloadResponse> {
    return this.client.download(path);
  }
}

export function createFilesDownloadClient(options: FilesClientOptions): FilesDownloadClient {
  return new FilesDownloadClient(options);
}
