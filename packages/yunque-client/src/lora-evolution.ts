/** Lightweight lora-evolution SDK facade over the LoRA slice. */
import {
  LoRAClient,
  LoRAClientError,
  createLoRAClient,
  type LoRAClientOptions,
  type LoRAEvolutionResponse,
} from "./lora.js";

export type {
  LoRAClientOptions as LoRAEvolutionClientOptions,
  LoRAEvolutionResponse,
};

export { LoRAClientError as LoRAEvolutionClientError };

export class LoRAEvolutionClient {
  private readonly client: LoRAClient;

  constructor(options: LoRAClientOptions) {
    this.client = createLoRAClient(options);
  }

  evolution(): Promise<LoRAEvolutionResponse> {
    return this.client.evolution();
  }
}

export function createLoRAEvolutionClient(options: LoRAClientOptions): LoRAEvolutionClient {
  return new LoRAEvolutionClient(options);
}
