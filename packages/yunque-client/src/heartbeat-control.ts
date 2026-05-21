/** Lightweight heartbeat-control SDK facade over the Heartbeat slice. */
import {
  HeartbeatClient,
  HeartbeatClientError,
  createHeartbeatClient,
  type HeartbeatClientOptions,
  type HeartbeatLogEntry,
  type UpdateHeartbeatRequest,
  type UpdateHeartbeatResponse,
} from "./heartbeat.js";

export type {
  HeartbeatClientOptions as HeartbeatControlClientOptions,
  HeartbeatLogEntry,
  UpdateHeartbeatRequest,
  UpdateHeartbeatResponse,
};

export { HeartbeatClientError as HeartbeatControlClientError };

export class HeartbeatControlClient {
  private readonly client: HeartbeatClient;

  constructor(options: HeartbeatClientOptions) {
    this.client = createHeartbeatClient(options);
  }

  update(body: UpdateHeartbeatRequest): Promise<UpdateHeartbeatResponse> {
    return this.client.update(body);
  }

  trigger(): Promise<HeartbeatLogEntry> {
    return this.client.trigger();
  }
}

export function createHeartbeatControlClient(options: HeartbeatClientOptions): HeartbeatControlClient {
  return new HeartbeatControlClient(options);
}
