/** Lightweight cognis-experience SDK facade over the Cognis slice. */
import {
  CognisClient,
  CognisClientError,
  createCognisClient,
  type CogniExperienceRecordRequest,
  type CogniExperienceResponse,
  type CogniMutationResponse,
  type CognisClientOptions,
} from "./cognis.js";

export type {
  CogniExperienceRecordRequest,
  CogniExperienceResponse,
  CogniMutationResponse,
  CognisClientOptions as CognisExperienceClientOptions,
};

export { CognisClientError as CognisExperienceClientError };

export class CognisExperienceClient {
  private readonly client: CognisClient;

  constructor(options: CognisClientOptions) {
    this.client = createCognisClient(options);
  }

  get(id: string): Promise<CogniExperienceResponse> { return this.client.experience(id); }
  record(id: string, request: CogniExperienceRecordRequest): Promise<CogniMutationResponse> { return this.client.recordExperience(id, request); }
  confirmPattern(id: string, patternId: string): Promise<CogniMutationResponse> { return this.client.confirmExperiencePattern(id, patternId); }
}

export function createCognisExperienceClient(options: CognisClientOptions): CognisExperienceClient {
  return new CognisExperienceClient(options);
}
