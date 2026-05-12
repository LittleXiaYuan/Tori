/** Lightweight tori-observe SDK facade over the Tori slice. */
import {
  ToriClient,
  ToriClientError,
  createToriClient,
  type ToriBindingStatus,
  type ToriClientOptions,
  type ToriHealthResponse,
  type ToriUsageResponse,
} from "./tori.js";

export type {
  ToriBindingStatus,
  ToriClientOptions as ToriObserveClientOptions,
  ToriHealthResponse,
  ToriUsageResponse,
};

export { ToriClientError as ToriObserveClientError };

export class ToriObserveClient {
  private readonly client: ToriClient;

  constructor(options: ToriClientOptions) {
    this.client = createToriClient(options);
  }

  status(): Promise<ToriBindingStatus> {
    return this.client.status();
  }

  health(): Promise<ToriHealthResponse> {
    return this.client.health();
  }

  usage(): Promise<ToriUsageResponse> {
    return this.client.usage();
  }
}

export function createToriObserveClient(options: ToriClientOptions): ToriObserveClient {
  return new ToriObserveClient(options);
}
