/** Lightweight connector-list SDK facade over the Connectors slice. */
import {
  createConnectorsClient,
  ConnectorsClient,
  ConnectorsClientError,
  type ConnectorListResponse,
  type ConnectorStatus,
  type ConnectorView,
  type ConnectorsClientOptions,
} from "./connectors.js";

export type {
  ConnectorListResponse,
  ConnectorStatus,
  ConnectorView,
  ConnectorsClientOptions as ConnectorListClientOptions,
};

export { ConnectorsClientError as ConnectorListClientError };

export class ConnectorListClient {
  private readonly client: ConnectorsClient;

  constructor(options: ConnectorsClientOptions) {
    this.client = createConnectorsClient(options);
  }

  list(): Promise<ConnectorListResponse> {
    return this.client.list();
  }
}

export function createConnectorListClient(options: ConnectorsClientOptions): ConnectorListClient {
  return new ConnectorListClient(options);
}
