/** Lightweight lora-config SDK facade over the LoRA slice. */
import {
  LoRAClient,
  LoRAClientError,
  createLoRAClient,
  type LoRAClientOptions,
  type LoRAConfig,
  type LoRAConfigResponse,
} from "./lora.js";

export type {
  LoRAClientOptions as LoRAConfigClientOptions,
  LoRAConfig,
  LoRAConfigResponse,
};

export { LoRAClientError as LoRAConfigClientError };

export class LoRAConfigClient {
  private readonly client: LoRAClient;

  constructor(options: LoRAClientOptions) { this.client = createLoRAClient(options); }
  get(): Promise<LoRAConfigResponse> { return this.client.config(); }
  update(config: LoRAConfig, method: "PUT" | "PATCH" = "PUT"): Promise<LoRAConfigResponse> { return this.client.updateConfig(config, method); }
}

export function createLoRAConfigClient(options: LoRAClientOptions): LoRAConfigClient { return new LoRAConfigClient(options); }
