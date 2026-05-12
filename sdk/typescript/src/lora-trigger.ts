/** Lightweight lora-trigger SDK facade over the LoRA slice. */
import {
  LoRAClient,
  LoRAClientError,
  createLoRAClient,
  type LoRAClientOptions,
  type TriggerLoRARequest,
  type TriggerLoRAResponse,
} from "./lora.js";

export type {
  LoRAClientOptions as LoRATriggerClientOptions,
  TriggerLoRARequest,
  TriggerLoRAResponse,
};

export { LoRAClientError as LoRATriggerClientError };

export class LoRATriggerClient {
  private readonly client: LoRAClient;

  constructor(options: LoRAClientOptions) {
    this.client = createLoRAClient(options);
  }

  trigger(body?: TriggerLoRARequest): Promise<TriggerLoRAResponse> {
    return this.client.trigger(body);
  }
}

export function createLoRATriggerClient(options: LoRAClientOptions): LoRATriggerClient {
  return new LoRATriggerClient(options);
}
