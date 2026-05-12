/** Lightweight persona-skills SDK facade over the Persona slice. */
import {
  PersonaClient,
  PersonaClientError,
  createPersonaClient,
  type AddPersonaSkillRequest,
  type DeletePersonaSkillRequest,
  type PersonaClientOptions,
  type PersonaSkill,
  type PersonaSkillsResponse,
  type PersonaStatusResponse,
} from "./persona.js";

export type {
  AddPersonaSkillRequest,
  DeletePersonaSkillRequest,
  PersonaClientOptions as PersonaSkillsClientOptions,
  PersonaSkill,
  PersonaSkillsResponse,
  PersonaStatusResponse,
};

export { PersonaClientError as PersonaSkillsClientError };

export class PersonaSkillsClient {
  private readonly client: PersonaClient;

  constructor(options: PersonaClientOptions) {
    this.client = createPersonaClient(options);
  }

  skills(): Promise<PersonaSkillsResponse> {
    return this.client.skills();
  }

  addSkill(body: AddPersonaSkillRequest): Promise<PersonaStatusResponse> {
    return this.client.addSkill(body);
  }

  deleteSkill(body: DeletePersonaSkillRequest): Promise<PersonaStatusResponse> {
    return this.client.deleteSkill(body);
  }
}

export function createPersonaSkillsClient(options: PersonaClientOptions): PersonaSkillsClient {
  return new PersonaSkillsClient(options);
}
