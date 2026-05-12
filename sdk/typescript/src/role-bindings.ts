/** Lightweight role-bindings SDK facade over the RBAC slice. */
import {
  createRBACClient,
  RBACClient,
  RBACClientError,
  type RBACClientOptions,
  type RBACRoleBindingRequest,
  type RBACRoleBindingResponse,
} from "./rbac.js";

export type {
  RBACClientOptions as RoleBindingsClientOptions,
  RBACRoleBindingRequest,
  RBACRoleBindingResponse,
};

export { RBACClientError as RoleBindingsClientError };

export class RoleBindingsClient {
  private readonly client: RBACClient;

  constructor(options: RBACClientOptions) { this.client = createRBACClient(options); }
  assign(request: RBACRoleBindingRequest): Promise<RBACRoleBindingResponse> { return this.client.assignRole(request); }
  revoke(request: RBACRoleBindingRequest): Promise<RBACRoleBindingResponse> { return this.client.revokeRole(request); }
}

export function createRoleBindingsClient(options: RBACClientOptions): RoleBindingsClient { return new RoleBindingsClient(options); }
