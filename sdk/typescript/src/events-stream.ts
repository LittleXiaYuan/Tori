/** Lightweight events-stream SDK facade over the Events slice. */
import {
  EventsClient,
  EventsClientError,
  createEventsClient,
  type EventStreamClientOptions,
  type EventStreamMessage,
  type EventStreamOptions,
} from "./events.js";

export type {
  EventStreamClientOptions as EventsStreamClientOptions,
  EventStreamMessage,
  EventStreamOptions,
};

export { EventsClientError as EventsStreamClientError };

export class EventsStreamClient {
  private readonly client: EventsClient;

  constructor(options: EventStreamClientOptions) {
    this.client = createEventsClient(options);
  }

  stream<T = unknown>(options: EventStreamOptions = {}): AsyncGenerator<EventStreamMessage<T>> {
    return this.client.stream<T>(options);
  }
}

export function createEventsStreamClient(options: EventStreamClientOptions): EventsStreamClient {
  return new EventsStreamClient(options);
}
