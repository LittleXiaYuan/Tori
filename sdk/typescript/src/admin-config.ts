/** Lightweight admin-config SDK facade over the Admin slice. */
import {
  AdminClient,
  AdminClientError,
  createAdminClient,
  type AdminClientOptions,
  type NLConfigRequest,
  type NLConfigResponse,
} from "./admin.js";

export type {
  AdminClientOptions as AdminConfigClientOptions,
  NLConfigRequest,
  NLConfigResponse,
};

export { AdminClientError as AdminConfigClientError };

export class AdminConfigClient {
  private readonly client: AdminClient;
  constructor(options: AdminClientOptions) { this.client = createAdminClient(options); }
  nlConfig(body: NLConfigRequest): Promise<NLConfigResponse> { return this.client.nlConfig(body); }
  nlConfigTranslate(text: string): Promise<NLConfigResponse> { return this.client.nlConfigTranslate(text); }
}

export function createAdminConfigClient(options: AdminClientOptions): AdminConfigClient { return new AdminConfigClient(options); }
