/** Lightweight admin-tenants SDK facade over the Admin slice. */
import {
  AdminClient,
  AdminClientError,
  createAdminClient,
  type AdminClientOptions,
  type TenantListResponse,
  type TenantRecord,
} from "./admin.js";

export type {
  AdminClientOptions as AdminTenantsClientOptions,
  TenantListResponse,
  TenantRecord,
};

export { AdminClientError as AdminTenantsClientError };

export class AdminTenantsClient {
  private readonly client: AdminClient;
  constructor(options: AdminClientOptions) { this.client = createAdminClient(options); }
  listTenants(): Promise<TenantListResponse> { return this.client.listTenants(); }
  createTenant(name: string): Promise<TenantRecord> { return this.client.createTenant(name); }
}

export function createAdminTenantsClient(options: AdminClientOptions): AdminTenantsClient { return new AdminTenantsClient(options); }
