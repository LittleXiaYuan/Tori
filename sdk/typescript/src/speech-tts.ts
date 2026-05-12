/** Lightweight speech-tts SDK facade over the Speech slice. */
import {
  SpeechClient,
  SpeechClientError,
  createSpeechClient,
  type SpeechAudioResponse,
  type SpeechClientOptions,
  type SpeechTTSRequest,
} from "./speech.js";

export type {
  SpeechAudioResponse,
  SpeechClientOptions as SpeechTTSClientOptions,
  SpeechTTSRequest,
};

export { SpeechClientError as SpeechTTSClientError };

export class SpeechTTSClient {
  private readonly client: SpeechClient;

  constructor(options: SpeechClientOptions) {
    this.client = createSpeechClient(options);
  }

  tts(body: SpeechTTSRequest): Promise<SpeechAudioResponse> {
    return this.client.tts(body);
  }
}

export function createSpeechTTSClient(options: SpeechClientOptions): SpeechTTSClient {
  return new SpeechTTSClient(options);
}
