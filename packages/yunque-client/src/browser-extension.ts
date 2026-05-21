/** Lightweight browser-extension SDK facade over the Browser slice. */
import {
  BrowserClient,
  BrowserClientError,
  createBrowserClient,
  type BrowserAction,
  type BrowserActionResult,
  type BrowserClientOptions,
  type BrowserExtensionSessionResponse,
  type BrowserRunScenarioResponse,
  type BrowserScenario,
  type BrowserScenariosResponse,
} from "./browser.js";

export type {
  BrowserAction,
  BrowserActionResult,
  BrowserClientOptions as BrowserExtensionClientOptions,
  BrowserExtensionSessionResponse,
  BrowserRunScenarioResponse,
  BrowserScenario,
  BrowserScenariosResponse,
};

export { BrowserClientError as BrowserExtensionClientError };

export class BrowserExtensionClient {
  private readonly client: BrowserClient;

  constructor(options: BrowserClientOptions) {
    this.client = createBrowserClient(options);
  }

  session(): Promise<BrowserExtensionSessionResponse> {
    return this.client.extensionSession();
  }

  action(action: BrowserAction): Promise<BrowserActionResult> {
    return this.client.extensionAction(action);
  }

  scenarios(): Promise<BrowserScenariosResponse> {
    return this.client.scenarios();
  }

  runScenario(scenarioId: string): Promise<BrowserRunScenarioResponse> {
    return this.client.runScenario(scenarioId);
  }
}

export function createBrowserExtensionClient(options: BrowserClientOptions): BrowserExtensionClient {
  return new BrowserExtensionClient(options);
}
