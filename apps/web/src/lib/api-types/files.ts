export type FileParseStatus = "parsed" | "needs_document_parser" | "error" | "ready" | string;

export interface FileParseMeta {
  parser?: string;
  backend?: string;
  markdown_chars?: number;
  has_layout_json?: boolean;
  preview?: string;
  status?: FileParseStatus;
  note?: string;
}

export interface FilePreviewResponse {
  name: string;
  path: string;
  size: number;
  ext: string;
  kind: string;
  content_type: string;
  preview: string;
  truncated: boolean;
  editable: boolean;
  parse?: FileParseMeta;
}

export interface FileUploadResponse {
  filename: string;
  size: number;
  path: string;
  parse?: FileParseMeta;
  analysis?: unknown;
  actions?: unknown[];
  rich?: unknown;
}
