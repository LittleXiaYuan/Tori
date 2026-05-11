/** Lightweight roles SDK facade over the RBAC slice. */
import {
  createRBACClient,
  RBACClient,
  RBACClientError,
  type RBACDeletedResponse,
  type RBACMyRolesResponse,
  type RBACRole,
  type RBACRoleBindingRequest,
  type RBACRoleBindingResponse,
  type RBACRolesResponse,
  type RBACClientOptions,
} from "./rbac.js";

export type {
  RBACClientOptions as RolesClientOptions,
  RBACDeletedResponse,
  RBACMyRolesResponse,
  RBACRole,
  RBACRoleBindingRequest,
  RBACRoleBindingResponse,
  RBACRolesResponse,
};

export { RBACClientError as RolesClientError };

export class RolesClient {
  private readonly client: RBACClient;

  constructor(options: RBACClientOptions) {
    this.client = createRBACClient(options);
  }

  list(): Promise<RBACRolesResponse> {
    return this.client.roles();
  }

  create(role: RBACRole): Promise<RBACRole> {
    return this.client.createRole(role);
  }

  delete(id: string): Promise<RBACDeletedResponse> {
    return this.client.deleteRole(id);
  }

  assign(request: RBACRoleBindingRequest): Promise<RBACRoleBindingResponse> {
    return this.client.assignRole(request);
  }

  revoke(request: RBACRoleBindingRequest): Promise<RBACRoleBindingResponse> {
    return this.client.revokeRole(request);
  }

  mine(): Promise<RBACMyRolesResponse> {
    return this.client.myRoles();
  }
}

export function createRolesClient(options: RBACClientOptions): RolesClient {
  return new RolesClient(options);
}
