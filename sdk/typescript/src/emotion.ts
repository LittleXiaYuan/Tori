/** Lightweight emotion SDK facade over the interactions slice. */
import {
  createInteractionsClient,
  InteractionsClient,
  InteractionsClientError,
  type ClearStickersRequest,
  type EmotionHistoryOptions,
  type EmotionHistoryResponse,
  type InteractionsClientOptions,
  type RegisterStickersRequest,
  type StatusResponse,
  type StickerMap,
  type StickerSuggestion,
} from "./interactions.js";

export type {
  ClearStickersRequest,
  EmotionHistoryOptions,
  EmotionHistoryResponse,
  InteractionsClientOptions as EmotionClientOptions,
  RegisterStickersRequest,
  StatusResponse,
  StickerMap,
  StickerSuggestion,
};

export { InteractionsClientError as EmotionClientError };

export class EmotionClient {
  private readonly client: InteractionsClient;

  constructor(options: InteractionsClientOptions) {
    this.client = createInteractionsClient(options);
  }

  history(options: EmotionHistoryOptions = {}): Promise<EmotionHistoryResponse> {
    return this.client.emotionHistory(options);
  }

  stickers(): Promise<StickerMap> {
    return this.client.stickers();
  }

  registerStickers(request: RegisterStickersRequest): Promise<StatusResponse> {
    return this.client.registerStickers(request);
  }

  clearStickers(request: ClearStickersRequest): Promise<StatusResponse> {
    return this.client.clearStickers(request);
  }
}

export function createEmotionClient(options: InteractionsClientOptions): EmotionClient {
  return new EmotionClient(options);
}
