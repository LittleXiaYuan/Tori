/** Lightweight permissions SDK facade over the RBAC slice. */
import {
  createRBACClient,
  RBACClient,
  RBACClientError,
  type RBACCheckRequest,
  type RBACCheckResponse,
  type RBACClientOptions,
  type RBACMyRolesResponse,
} from "./rbac.js";

export type {
  RBACCheckRequest,
  RBACCheckResponse,
  RBACClientOptions as PermissionsClientOptions,
  RBACMyRolesResponse,
};

export { RBACClientError as PermissionsClientError };

export class PermissionsClient {
  private readonly client: RBACClient;

  constructor(options: RBACClientOptions) {
    this.client = createRBACClient(options);
  }

  check(request: RBACCheckRequest): Promise<RBACCheckResponse> {
    return this.client.check(request);
  }

  myRoles(): Promise<RBACMyRolesResponse> {
    return this.client.myRoles();
  }
}

export function createPermissionsClient(options: RBACClientOptions): PermissionsClient {
  return new PermissionsClient(options);
}
