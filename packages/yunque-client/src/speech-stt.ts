/** Lightweight speech-stt SDK facade over the Speech slice. */
import {
  SpeechClient,
  SpeechClientError,
  createSpeechClient,
  type AudioBody,
  type SpeechClientOptions,
  type SpeechSTTOptions,
  type SpeechSTTResponse,
} from "./speech.js";

export type {
  AudioBody,
  SpeechClientOptions as SpeechSTTClientOptions,
  SpeechSTTOptions,
  SpeechSTTResponse,
};

export { SpeechClientError as SpeechSTTClientError };

export class SpeechSTTClient {
  private readonly client: SpeechClient;

  constructor(options: SpeechClientOptions) {
    this.client = createSpeechClient(options);
  }

  stt(audio: AudioBody, options?: SpeechSTTOptions): Promise<SpeechSTTResponse> {
    return this.client.stt(audio, options);
  }

  sttStreamUrl(options?: SpeechSTTOptions): string {
    return this.client.sttStreamUrl(options);
  }
}

export function createSpeechSTTClient(options: SpeechClientOptions): SpeechSTTClient {
  return new SpeechSTTClient(options);
}
