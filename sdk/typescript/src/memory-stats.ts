/** Lightweight memory-stats SDK facade over the Memory slice. */
import {
  createMemoryClient,
  MemoryClient,
  MemoryClientError,
  type MemoryClientOptions,
  type MemoryStats,
} from "./memory.js";

export type {
  MemoryClientOptions as MemoryStatsClientOptions,
  MemoryStats,
};

export { MemoryClientError as MemoryStatsClientError };

export class MemoryStatsClient {
  private readonly client: MemoryClient;

  constructor(options: MemoryClientOptions) {
    this.client = createMemoryClient(options);
  }

  stats(): Promise<MemoryStats> {
    return this.client.stats();
  }
}

export function createMemoryStatsClient(options: MemoryClientOptions): MemoryStatsClient {
  return new MemoryStatsClient(options);
}
