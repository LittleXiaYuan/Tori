/** Lightweight memory-add SDK facade over the Memory slice. */
import {
  createMemoryClient,
  MemoryClient,
  MemoryClientError,
  type MemoryAddRequest,
  type MemoryAddResponse,
  type MemoryClientOptions,
  type MemoryLayer,
} from "./memory.js";

export type {
  MemoryAddRequest,
  MemoryAddResponse,
  MemoryClientOptions as MemoryAddClientOptions,
  MemoryLayer,
};

export { MemoryClientError as MemoryAddClientError };

export type MemoryRememberOptions = Omit<MemoryAddRequest, "content" | "value">;

export class MemoryAddClient {
  private readonly client: MemoryClient;

  constructor(options: MemoryClientOptions) {
    this.client = createMemoryClient(options);
  }

  add(request: MemoryAddRequest): Promise<MemoryAddResponse> {
    return this.client.add(request);
  }

  remember(content: string, options: MemoryRememberOptions = {}): Promise<MemoryAddResponse> {
    return this.client.add({ ...options, content });
  }
}

export function createMemoryAddClient(options: MemoryClientOptions): MemoryAddClient {
  return new MemoryAddClient(options);
}
