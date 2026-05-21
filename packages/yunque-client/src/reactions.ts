/** Lightweight reactions SDK facade over the interactions slice. */
import {
  createInteractionsClient,
  InteractionsClient,
  InteractionsClientError,
  type InteractionsClientOptions,
  type ReactRequest,
  type SendStickerRequest,
  type StatusResponse,
} from "./interactions.js";

export type {
  InteractionsClientOptions as ReactionsClientOptions,
  ReactRequest,
  SendStickerRequest,
  StatusResponse,
};

export { InteractionsClientError as ReactionsClientError };

export class ReactionsClient {
  private readonly client: InteractionsClient;

  constructor(options: InteractionsClientOptions) {
    this.client = createInteractionsClient(options);
  }

  react(request: ReactRequest): Promise<StatusResponse> {
    return this.client.react(request);
  }

  sendSticker(request: SendStickerRequest): Promise<StatusResponse> {
    return this.client.sendSticker(request);
  }
}

export function createReactionsClient(options: InteractionsClientOptions): ReactionsClient {
  return new ReactionsClient(options);
}
