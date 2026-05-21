/** Lightweight speech-voices SDK facade over the Speech slice. */
import {
  SpeechClient,
  SpeechClientError,
  createSpeechClient,
  type SpeechClientOptions,
  type SpeechVoicesResponse,
} from "./speech.js";

export type {
  SpeechClientOptions as SpeechVoicesClientOptions,
  SpeechVoicesResponse,
};

export { SpeechClientError as SpeechVoicesClientError };

export class SpeechVoicesClient {
  private readonly client: SpeechClient;

  constructor(options: SpeechClientOptions) {
    this.client = createSpeechClient(options);
  }

  voices(): Promise<SpeechVoicesResponse> {
    return this.client.voices();
  }
}

export function createSpeechVoicesClient(options: SpeechClientOptions): SpeechVoicesClient {
  return new SpeechVoicesClient(options);
}
