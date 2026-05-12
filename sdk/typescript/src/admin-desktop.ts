/** Lightweight admin-desktop SDK facade over the Admin slice. */
import {
  AdminClient,
  AdminClientError,
  createAdminClient,
  type AdminClientOptions,
  type DesktopAutostartResponse,
  type DesktopConsoleResponse,
} from "./admin.js";

export type {
  AdminClientOptions as AdminDesktopClientOptions,
  DesktopAutostartResponse,
  DesktopConsoleResponse,
};

export { AdminClientError as AdminDesktopClientError };

export class AdminDesktopClient {
  private readonly client: AdminClient;
  constructor(options: AdminClientOptions) { this.client = createAdminClient(options); }
  consoleStatus(): Promise<DesktopConsoleResponse> { return this.client.consoleStatus(); }
  toggleConsole(): Promise<DesktopConsoleResponse> { return this.client.toggleConsole(); }
  autostartStatus(): Promise<DesktopAutostartResponse> { return this.client.autostartStatus(); }
  toggleAutostart(): Promise<DesktopAutostartResponse> { return this.client.toggleAutostart(); }
}

export function createAdminDesktopClient(options: AdminClientOptions): AdminDesktopClient { return new AdminDesktopClient(options); }
