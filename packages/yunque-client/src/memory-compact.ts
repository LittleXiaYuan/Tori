/** Lightweight memory-compact SDK facade over the Memory slice. */
import {
  createMemoryClient,
  MemoryClient,
  MemoryClientError,
  type MemoryClientOptions,
  type MemoryCompactRequest,
  type MemoryCompactResponse,
} from "./memory.js";

export type {
  MemoryClientOptions as MemoryCompactClientOptions,
  MemoryCompactRequest,
  MemoryCompactResponse,
};

export { MemoryClientError as MemoryCompactClientError };

export class MemoryCompactClient {
  private readonly client: MemoryClient;

  constructor(options: MemoryClientOptions) {
    this.client = createMemoryClient(options);
  }

  compact(request: MemoryCompactRequest = {}): Promise<MemoryCompactResponse> {
    return this.client.compact(request);
  }
}

export function createMemoryCompactClient(options: MemoryClientOptions): MemoryCompactClient {
  return new MemoryCompactClient(options);
}
