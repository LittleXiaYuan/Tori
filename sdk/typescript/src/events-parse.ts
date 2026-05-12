/** Lightweight events-parse SDK facade over the Events slice. */
import {
  EventsClient,
  EventsClientError,
  createEventsClient,
  type EventStreamClientOptions,
  type EventStreamMessage,
} from "./events.js";

export type {
  EventStreamClientOptions as EventsParseClientOptions,
  EventStreamMessage,
};

export { EventsClientError as EventsParseClientError };

export class EventsParseClient {
  private readonly client: EventsClient;

  constructor(options: EventStreamClientOptions) {
    this.client = createEventsClient(options);
  }

  parseStream<T = unknown>(body: ReadableStream<Uint8Array>): AsyncGenerator<EventStreamMessage<T>> {
    return this.client.parseStream<T>(body);
  }
}

export function createEventsParseClient(options: EventStreamClientOptions): EventsParseClient {
  return new EventsParseClient(options);
}
