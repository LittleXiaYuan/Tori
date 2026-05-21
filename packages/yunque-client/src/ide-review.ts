/** Lightweight ide-review SDK facade over the IDE slice. */
import {
  IDEClient,
  IDEClientError,
  createIDEClient,
  type IDEClientOptions,
  type IDEReviewIssue,
  type IDEReviewMode,
  type IDEReviewRequest,
  type IDEReviewResponse,
} from "./ide.js";

export type {
  IDEClientOptions as IDEReviewClientOptions,
  IDEReviewIssue,
  IDEReviewMode,
  IDEReviewRequest,
  IDEReviewResponse,
};

export { IDEClientError as IDEReviewClientError };

export class IDEReviewClient {
  private readonly client: IDEClient;

  constructor(options: IDEClientOptions) {
    this.client = createIDEClient(options);
  }

  review(body: IDEReviewRequest): Promise<IDEReviewResponse> {
    return this.client.review(body);
  }

  reviewDiff(body: Omit<IDEReviewRequest, "mode" | "content"> & { diff: string }): Promise<IDEReviewResponse> {
    return this.client.reviewDiff(body);
  }

  reviewQuick(body: Omit<IDEReviewRequest, "mode" | "diff"> & { content: string }): Promise<IDEReviewResponse> {
    return this.client.reviewQuick(body);
  }

  reviewFull(body: Omit<IDEReviewRequest, "mode" | "diff"> & { content: string }): Promise<IDEReviewResponse> {
    return this.client.reviewFull(body);
  }
}

export function createIDEReviewClient(options: IDEClientOptions): IDEReviewClient {
  return new IDEReviewClient(options);
}
