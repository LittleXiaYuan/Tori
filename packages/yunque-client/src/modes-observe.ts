/** Lightweight modes-observe SDK facade over the Modes slice. */
import {
  ModesClient,
  ModesClientError,
  createModesClient,
  type CurrentModeResponse,
  type ListModesResponse,
  type ModesClientOptions,
  type ModesQuery,
  type PersonaMode,
  type PersonaModeInfo,
} from "./modes.js";

export type {
  CurrentModeResponse,
  ListModesResponse,
  ModesClientOptions as ModesObserveClientOptions,
  ModesQuery,
  PersonaMode,
  PersonaModeInfo,
};

export { ModesClientError as ModesObserveClientError };

export class ModesObserveClient {
  private readonly client: ModesClient;

  constructor(options: ModesClientOptions) {
    this.client = createModesClient(options);
  }

  list(query?: ModesQuery): Promise<ListModesResponse> {
    return this.client.list(query);
  }

  current(query?: ModesQuery): Promise<CurrentModeResponse> {
    return this.client.current(query);
  }
}

export function createModesObserveClient(options: ModesClientOptions): ModesObserveClient {
  return new ModesObserveClient(options);
}
