/** Lightweight emotion-history SDK facade over the interactions slice. */
import {
  createInteractionsClient,
  InteractionsClient,
  InteractionsClientError,
  type EmotionHistoryOptions,
  type EmotionHistoryResponse,
  type InteractionsClientOptions,
} from "./interactions.js";

export type {
  EmotionHistoryOptions,
  EmotionHistoryResponse,
  InteractionsClientOptions as EmotionHistoryClientOptions,
};

export { InteractionsClientError as EmotionHistoryClientError };

export class EmotionHistoryClient {
  private readonly client: InteractionsClient;

  constructor(options: InteractionsClientOptions) { this.client = createInteractionsClient(options); }
  history(options: EmotionHistoryOptions = {}): Promise<EmotionHistoryResponse> { return this.client.emotionHistory(options); }
}

export function createEmotionHistoryClient(options: InteractionsClientOptions): EmotionHistoryClient { return new EmotionHistoryClient(options); }
