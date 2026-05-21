/** Lightweight modes-control SDK facade over the Modes slice. */
import {
  ModesClient,
  ModesClientError,
  createModesClient,
  type ModesClientOptions,
  type ModesQuery,
  type PersonaMode,
  type PersonaModeInfo,
  type SetModeRequest,
  type SetModeResponse,
} from "./modes.js";

export type {
  ModesClientOptions as ModesControlClientOptions,
  ModesQuery,
  PersonaMode,
  PersonaModeInfo,
  SetModeRequest,
  SetModeResponse,
};

export { ModesClientError as ModesControlClientError };

export class ModesControlClient {
  private readonly client: ModesClient;

  constructor(options: ModesClientOptions) {
    this.client = createModesClient(options);
  }

  set(body: SetModeRequest): Promise<SetModeResponse> {
    return this.client.set(body);
  }
}

export function createModesControlClient(options: ModesClientOptions): ModesControlClient {
  return new ModesControlClient(options);
}
