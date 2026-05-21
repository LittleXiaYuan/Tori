/** Lightweight lora-history SDK facade over the LoRA slice. */
import {
  LoRAClient,
  LoRAClientError,
  createLoRAClient,
  type LoRAClientOptions,
  type LoRAHistoryResponse,
  type LoRASummaryResponse,
} from "./lora.js";

export type {
  LoRAClientOptions as LoRAHistoryClientOptions,
  LoRAHistoryResponse,
  LoRASummaryResponse,
};

export { LoRAClientError as LoRAHistoryClientError };

export class LoRAHistoryClient {
  private readonly client: LoRAClient;

  constructor(options: LoRAClientOptions) { this.client = createLoRAClient(options); }
  history(): Promise<LoRAHistoryResponse> { return this.client.history(); }
  summary(): Promise<LoRASummaryResponse> { return this.client.summary(); }
}

export function createLoRAHistoryClient(options: LoRAClientOptions): LoRAHistoryClient { return new LoRAHistoryClient(options); }
