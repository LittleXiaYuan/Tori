/** Lightweight browser-opp SDK facade over the Browser slice. */
import {
  BrowserClient,
  BrowserClientError,
  createBrowserClient,
  type BrowserClientOptions,
  type BrowserOPPDecisionResponse,
  type BrowserOPPItem,
  type BrowserOPPPendingResponse,
} from "./browser.js";

export type BrowserOPPDecisionInput = { problem_id?: string; id?: string; decision: string };

export type {
  BrowserClientOptions as BrowserOPPClientOptions,
  BrowserOPPDecisionResponse,
  BrowserOPPItem,
  BrowserOPPPendingResponse,
};

export { BrowserClientError as BrowserOPPClientError };

export class BrowserOPPClient {
  private readonly client: BrowserClient;

  constructor(options: BrowserClientOptions) {
    this.client = createBrowserClient(options);
  }

  pending(): Promise<BrowserOPPPendingResponse> {
    return this.client.oppPending();
  }

  decide(input: BrowserOPPDecisionInput): Promise<BrowserOPPDecisionResponse> {
    return this.client.oppDecide(input);
  }
}

export function createBrowserOPPClient(options: BrowserClientOptions): BrowserOPPClient {
  return new BrowserOPPClient(options);
}
