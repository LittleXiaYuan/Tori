/** Lightweight tori-bind SDK facade over the Tori slice. */
import {
  ToriClient,
  ToriClientError,
  createToriClient,
  type ToriBindRequest,
  type ToriBindResponse,
  type ToriClientOptions,
  type ToriUnbindResponse,
} from "./tori.js";

export type {
  ToriBindRequest,
  ToriBindResponse,
  ToriClientOptions as ToriBindClientOptions,
  ToriUnbindResponse,
};

export { ToriClientError as ToriBindClientError };

export class ToriBindClient {
  private readonly client: ToriClient;

  constructor(options: ToriClientOptions) {
    this.client = createToriClient(options);
  }

  bind(body: ToriBindRequest = {}): Promise<ToriBindResponse> {
    return this.client.bind(body);
  }

  unbind(): Promise<ToriUnbindResponse> {
    return this.client.unbind();
  }
}

export function createToriBindClient(options: ToriClientOptions): ToriBindClient {
  return new ToriBindClient(options);
}
