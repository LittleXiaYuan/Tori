/** Lightweight emotion-stickers SDK facade over the interactions slice. */
import {
  createInteractionsClient,
  InteractionsClient,
  InteractionsClientError,
  type ClearStickersRequest,
  type InteractionsClientOptions,
  type RegisterStickersRequest,
  type StatusResponse,
  type StickerMap,
  type StickerSuggestion,
} from "./interactions.js";

export type {
  ClearStickersRequest,
  InteractionsClientOptions as EmotionStickersClientOptions,
  RegisterStickersRequest,
  StatusResponse,
  StickerMap,
  StickerSuggestion,
};

export { InteractionsClientError as EmotionStickersClientError };

export class EmotionStickersClient {
  private readonly client: InteractionsClient;

  constructor(options: InteractionsClientOptions) { this.client = createInteractionsClient(options); }
  list(): Promise<StickerMap> { return this.client.stickers(); }
  register(request: RegisterStickersRequest): Promise<StatusResponse> { return this.client.registerStickers(request); }
  clear(request: ClearStickersRequest): Promise<StatusResponse> { return this.client.clearStickers(request); }
}

export function createEmotionStickersClient(options: InteractionsClientOptions): EmotionStickersClient { return new EmotionStickersClient(options); }
