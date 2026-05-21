/** Lightweight lora-control SDK facade over the LoRA slice. */
import {
  LoRAClient,
  LoRAClientError,
  createLoRAClient,
  type LoRAClientOptions,
  type LoRAConfig,
  type LoRAConfigResponse,
  type LoRARollbackResponse,
  type TriggerLoRARequest,
  type TriggerLoRAResponse,
} from "./lora.js";

export type {
  LoRAClientOptions as LoRAControlClientOptions,
  LoRAConfig,
  LoRAConfigResponse,
  LoRARollbackResponse,
  TriggerLoRARequest,
  TriggerLoRAResponse,
};

export { LoRAClientError as LoRAControlClientError };

export class LoRAControlClient {
  private readonly client: LoRAClient;

  constructor(options: LoRAClientOptions) {
    this.client = createLoRAClient(options);
  }

  trigger(body?: TriggerLoRARequest): Promise<TriggerLoRAResponse> {
    return this.client.trigger(body);
  }

  rollback(): Promise<LoRARollbackResponse> {
    return this.client.rollback();
  }

  config(): Promise<LoRAConfigResponse> {
    return this.client.config();
  }

  updateConfig(config: LoRAConfig, method: "PUT" | "PATCH" = "PUT"): Promise<LoRAConfigResponse> {
    return this.client.updateConfig(config, method);
  }
}

export function createLoRAControlClient(options: LoRAClientOptions): LoRAControlClient {
  return new LoRAControlClient(options);
}
