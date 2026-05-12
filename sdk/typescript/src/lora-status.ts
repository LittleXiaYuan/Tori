/** Lightweight lora-status SDK facade over the LoRA slice. */
import {
  LoRAClient,
  LoRAClientError,
  createLoRAClient,
  type LoRAClientOptions,
  type LoRAEvolutionResponse,
  type LoRAPreviewQuery,
  type LoRAPreviewResponse,
  type LoRAStatusResponse,
} from "./lora.js";

export type {
  LoRAClientOptions as LoRAStatusClientOptions,
  LoRAEvolutionResponse,
  LoRAPreviewQuery,
  LoRAPreviewResponse,
  LoRAStatusResponse,
};

export { LoRAClientError as LoRAStatusClientError };

export class LoRAStatusClient {
  private readonly client: LoRAClient;

  constructor(options: LoRAClientOptions) { this.client = createLoRAClient(options); }
  status(): Promise<LoRAStatusResponse> { return this.client.status(); }
  preview(query?: LoRAPreviewQuery): Promise<LoRAPreviewResponse> { return this.client.preview(query); }
  evolution(): Promise<LoRAEvolutionResponse> { return this.client.evolution(); }
}

export function createLoRAStatusClient(options: LoRAClientOptions): LoRAStatusClient { return new LoRAStatusClient(options); }
