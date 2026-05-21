/** Lightweight my-roles SDK facade over the RBAC slice. */
import {
  createRBACClient,
  RBACClient,
  RBACClientError,
  type RBACClientOptions,
  type RBACMyRolesResponse,
} from "./rbac.js";

export type {
  RBACClientOptions as MyRolesClientOptions,
  RBACMyRolesResponse,
};

export { RBACClientError as MyRolesClientError };

export class MyRolesClient {
  private readonly client: RBACClient;

  constructor(options: RBACClientOptions) { this.client = createRBACClient(options); }
  get(): Promise<RBACMyRolesResponse> { return this.client.myRoles(); }
}

export function createMyRolesClient(options: RBACClientOptions): MyRolesClient { return new MyRolesClient(options); }
