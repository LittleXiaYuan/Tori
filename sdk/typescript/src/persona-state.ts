/** Lightweight persona-state SDK facade over the Persona slice. */
import {
  PersonaClient,
  PersonaClientError,
  createPersonaClient,
  type PersonaClientOptions,
  type PersonaState,
  type PersonaStatusResponse,
  type UpdatePersonaRequest,
} from "./persona.js";

export type {
  PersonaClientOptions as PersonaStateClientOptions,
  PersonaState,
  PersonaStatusResponse,
  UpdatePersonaRequest,
};

export { PersonaClientError as PersonaStateClientError };

export class PersonaStateClient {
  private readonly client: PersonaClient;

  constructor(options: PersonaClientOptions) {
    this.client = createPersonaClient(options);
  }

  get(): Promise<PersonaState> {
    return this.client.get();
  }

  update(body: UpdatePersonaRequest): Promise<PersonaStatusResponse> {
    return this.client.update(body);
  }
}

export function createPersonaStateClient(options: PersonaClientOptions): PersonaStateClient {
  return new PersonaStateClient(options);
}
