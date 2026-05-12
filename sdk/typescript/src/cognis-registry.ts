/** Lightweight cognis-registry SDK facade over the Cognis slice. */
import {
  CognisClient,
  CognisClientError,
  createCognisClient,
  type CogniDeclaration,
  type CogniListResponse,
  type CogniMutationResponse,
  type CognisClientOptions,
} from "./cognis.js";

export type {
  CogniDeclaration,
  CogniListResponse,
  CogniMutationResponse,
  CognisClientOptions as CognisRegistryClientOptions,
};

export { CognisClientError as CognisRegistryClientError };

export class CognisRegistryClient {
  private readonly client: CognisClient;

  constructor(options: CognisClientOptions) {
    this.client = createCognisClient(options);
  }

  list(): Promise<CogniListResponse> { return this.client.list(); }
  create(declaration: CogniDeclaration): Promise<CogniDeclaration> { return this.client.create(declaration); }
  get(id: string): Promise<CogniDeclaration> { return this.client.get(id); }
  remove(id: string): Promise<CogniMutationResponse> { return this.client.remove(id); }
  enable(id: string): Promise<CogniMutationResponse> { return this.client.enable(id); }
  disable(id: string): Promise<CogniMutationResponse> { return this.client.disable(id); }
  reload(): Promise<CogniMutationResponse> { return this.client.reload(); }
}

export function createCognisRegistryClient(options: CognisClientOptions): CognisRegistryClient {
  return new CognisRegistryClient(options);
}
