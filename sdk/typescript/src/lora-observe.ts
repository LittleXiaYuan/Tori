/** Lightweight lora-observe SDK facade over the LoRA slice. */
import {
  LoRAClient,
  LoRAClientError,
  createLoRAClient,
  type LoRAClientOptions,
  type LoRAEvolutionResponse,
  type LoRAHistoryResponse,
  type LoRAPreviewQuery,
  type LoRAPreviewResponse,
  type LoRAStatusResponse,
  type LoRASummaryResponse,
} from "./lora.js";

export type {
  LoRAClientOptions as LoRAObserveClientOptions,
  LoRAEvolutionResponse,
  LoRAHistoryResponse,
  LoRAPreviewQuery,
  LoRAPreviewResponse,
  LoRAStatusResponse,
  LoRASummaryResponse,
};

export { LoRAClientError as LoRAObserveClientError };

export class LoRAObserveClient {
  private readonly client: LoRAClient;

  constructor(options: LoRAClientOptions) {
    this.client = createLoRAClient(options);
  }

  status(): Promise<LoRAStatusResponse> {
    return this.client.status();
  }

  history(): Promise<LoRAHistoryResponse> {
    return this.client.history();
  }

  summary(): Promise<LoRASummaryResponse> {
    return this.client.summary();
  }

  preview(query?: LoRAPreviewQuery): Promise<LoRAPreviewResponse> {
    return this.client.preview(query);
  }

  evolution(): Promise<LoRAEvolutionResponse> {
    return this.client.evolution();
  }
}

export function createLoRAObserveClient(options: LoRAClientOptions): LoRAObserveClient {
  return new LoRAObserveClient(options);
}
