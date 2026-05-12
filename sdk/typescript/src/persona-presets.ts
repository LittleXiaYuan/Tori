/** Lightweight persona-presets SDK facade over the Persona slice. */
import {
  PersonaClient,
  PersonaClientError,
  createPersonaClient,
  type AddCustomPersonaPresetRequest,
  type AddCustomPersonaPresetResponse,
  type DeleteCustomPersonaPresetRequest,
  type ListPersonaPresetsResponse,
  type PersonaClientOptions,
  type PersonaPreset,
  type PersonaStatusResponse,
  type SwitchPersonaPresetRequest,
  type SwitchPersonaPresetResponse,
  type UpdatePersonaPresetFeaturesRequest,
} from "./persona.js";

export type {
  AddCustomPersonaPresetRequest,
  AddCustomPersonaPresetResponse,
  DeleteCustomPersonaPresetRequest,
  ListPersonaPresetsResponse,
  PersonaClientOptions as PersonaPresetsClientOptions,
  PersonaPreset,
  PersonaStatusResponse,
  SwitchPersonaPresetRequest,
  SwitchPersonaPresetResponse,
  UpdatePersonaPresetFeaturesRequest,
};

export { PersonaClientError as PersonaPresetsClientError };

export class PersonaPresetsClient {
  private readonly client: PersonaClient;

  constructor(options: PersonaClientOptions) {
    this.client = createPersonaClient(options);
  }

  presets(): Promise<ListPersonaPresetsResponse> {
    return this.client.presets();
  }

  switchPreset(body: SwitchPersonaPresetRequest): Promise<SwitchPersonaPresetResponse> {
    return this.client.switchPreset(body);
  }

  addCustomPreset(body: AddCustomPersonaPresetRequest): Promise<AddCustomPersonaPresetResponse> {
    return this.client.addCustomPreset(body);
  }

  deleteCustomPreset(body: DeleteCustomPersonaPresetRequest): Promise<PersonaStatusResponse> {
    return this.client.deleteCustomPreset(body);
  }

  updatePresetFeatures(body: UpdatePersonaPresetFeaturesRequest): Promise<PersonaStatusResponse> {
    return this.client.updatePresetFeatures(body);
  }
}

export function createPersonaPresetsClient(options: PersonaClientOptions): PersonaPresetsClient {
  return new PersonaPresetsClient(options);
}
