/** Lightweight Models SDK facade over the providers slice. */
import {
  createProvidersClient,
  ProvidersClient,
  ProvidersClientError,
  type ModelEntry,
  type ModelsResponse,
  type ProviderActionResponse,
  type ProvidersClientOptions,
} from "./providers.js";

export type {
  ModelEntry,
  ModelsResponse,
  ProviderActionResponse as DeleteModelResponse,
  ProvidersClientOptions as ModelsClientOptions,
};

export { ProvidersClientError as ModelsClientError };

export class ModelsClient {
  private readonly client: ProvidersClient;

  constructor(options: ProvidersClientOptions) {
    this.client = createProvidersClient(options);
  }

  listModels(): Promise<ModelsResponse> {
    return this.client.listModels();
  }

  addModel(model: ModelEntry): Promise<ModelEntry> {
    return this.client.addModel(model);
  }

  deleteModel(id: string): Promise<ProviderActionResponse> {
    return this.client.deleteModel(id);
  }
}

export function createModelsClient(options: ProvidersClientOptions): ModelsClient {
  return new ModelsClient(options);
}
