/** Lightweight browser-status SDK facade over the Browser slice. */
import {
  BrowserClient,
  BrowserClientError,
  createBrowserClient,
  type BrowserClientOptions,
  type BrowserConfigResponse,
  type BrowserStatusResponse,
} from "./browser.js";

export type {
  BrowserClientOptions as BrowserStatusClientOptions,
  BrowserConfigResponse,
  BrowserStatusResponse,
};

export { BrowserClientError as BrowserStatusClientError };

export class BrowserStatusClient {
  private readonly client: BrowserClient;

  constructor(options: BrowserClientOptions) {
    this.client = createBrowserClient(options);
  }

  status(): Promise<BrowserStatusResponse> {
    return this.client.status();
  }

  config(): Promise<BrowserConfigResponse> {
    return this.client.config();
  }

  extensionStatus(): Promise<BrowserStatusResponse> {
    return this.client.extensionStatus();
  }
}

export function createBrowserStatusClient(options: BrowserClientOptions): BrowserStatusClient {
  return new BrowserStatusClient(options);
}
