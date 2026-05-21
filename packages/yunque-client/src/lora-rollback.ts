/** Lightweight lora-rollback SDK facade over the LoRA slice. */
import {
  LoRAClient,
  LoRAClientError,
  createLoRAClient,
  type LoRAClientOptions,
  type LoRARollbackResponse,
} from "./lora.js";

export type {
  LoRAClientOptions as LoRARollbackClientOptions,
  LoRARollbackResponse,
};

export { LoRAClientError as LoRARollbackClientError };

export class LoRARollbackClient {
  private readonly client: LoRAClient;

  constructor(options: LoRAClientOptions) {
    this.client = createLoRAClient(options);
  }

  rollback(): Promise<LoRARollbackResponse> {
    return this.client.rollback();
  }
}

export function createLoRARollbackClient(options: LoRAClientOptions): LoRARollbackClient {
  return new LoRARollbackClient(options);
}
