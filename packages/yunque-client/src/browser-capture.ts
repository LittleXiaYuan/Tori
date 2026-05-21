/** Lightweight browser-capture SDK facade over the Browser slice. */
import {
  BrowserClient,
  BrowserClientError,
  createBrowserClient,
  type BrowserClientOptions,
  type BrowserOCRResponse,
  type BrowserScreenshotResponse,
} from "./browser.js";

export type {
  BrowserClientOptions as BrowserCaptureClientOptions,
  BrowserOCRResponse,
  BrowserScreenshotResponse,
};

export { BrowserClientError as BrowserCaptureClientError };

export class BrowserCaptureClient {
  private readonly client: BrowserClient;

  constructor(options: BrowserClientOptions) {
    this.client = createBrowserClient(options);
  }

  screenshot(): Promise<BrowserScreenshotResponse> {
    return this.client.screenshot();
  }

  latestScreenshot(): Promise<BrowserScreenshotResponse> {
    return this.client.latestScreenshot();
  }

  ocr(): Promise<BrowserOCRResponse> {
    return this.client.ocr();
  }
}

export function createBrowserCaptureClient(options: BrowserClientOptions): BrowserCaptureClient {
  return new BrowserCaptureClient(options);
}
