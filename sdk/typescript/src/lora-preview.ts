/** Lightweight lora-preview SDK facade over the LoRA slice. */
import {
  LoRAClient,
  LoRAClientError,
  createLoRAClient,
  type LoRAClientOptions,
  type LoRAPreviewQuery,
  type LoRAPreviewResponse,
} from "./lora.js";

export type {
  LoRAClientOptions as LoRAPreviewClientOptions,
  LoRAPreviewQuery,
  LoRAPreviewResponse,
};

export { LoRAClientError as LoRAPreviewClientError };

export class LoRAPreviewClient {
  private readonly client: LoRAClient;

  constructor(options: LoRAClientOptions) {
    this.client = createLoRAClient(options);
  }

  preview(query?: LoRAPreviewQuery): Promise<LoRAPreviewResponse> {
    return this.client.preview(query);
  }
}

export function createLoRAPreviewClient(options: LoRAClientOptions): LoRAPreviewClient {
  return new LoRAPreviewClient(options);
}
