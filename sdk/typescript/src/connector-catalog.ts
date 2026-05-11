/** Lightweight connector-catalog SDK facade over the Connectors slice. */
import {
  createConnectorsClient,
  ConnectorsClient,
  ConnectorsClientError,
  type ConnectorAction,
  type ConnectorDefinition,
  type ConnectorDetailResponse,
  type ConnectorListResponse,
  type ConnectorStatus,
  type ConnectorView,
  type ConnectorsClientOptions,
} from "./connectors.js";

export type {
  ConnectorAction,
  ConnectorDefinition,
  ConnectorDetailResponse,
  ConnectorListResponse,
  ConnectorStatus,
  ConnectorView,
  ConnectorsClientOptions as ConnectorCatalogClientOptions,
};

export { ConnectorsClientError as ConnectorCatalogClientError };

export class ConnectorCatalogClient {
  private readonly client: ConnectorsClient;

  constructor(options: ConnectorsClientOptions) {
    this.client = createConnectorsClient(options);
  }

  list(): Promise<ConnectorListResponse> {
    return this.client.list();
  }

  detail(id: string): Promise<ConnectorDetailResponse> {
    return this.client.detail(id);
  }
}

export function createConnectorCatalogClient(options: ConnectorsClientOptions): ConnectorCatalogClient {
  return new ConnectorCatalogClient(options);
}
