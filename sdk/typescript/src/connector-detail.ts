/** Lightweight connector-detail SDK facade over the Connectors slice. */
import {
  createConnectorsClient,
  ConnectorsClient,
  ConnectorsClientError,
  type ConnectorAction,
  type ConnectorDefinition,
  type ConnectorDetailResponse,
  type ConnectorStatus,
  type ConnectorsClientOptions,
} from "./connectors.js";

export type {
  ConnectorAction,
  ConnectorDefinition,
  ConnectorDetailResponse,
  ConnectorStatus,
  ConnectorsClientOptions as ConnectorDetailClientOptions,
};

export { ConnectorsClientError as ConnectorDetailClientError };

export class ConnectorDetailClient {
  private readonly client: ConnectorsClient;

  constructor(options: ConnectorsClientOptions) {
    this.client = createConnectorsClient(options);
  }

  detail(id: string): Promise<ConnectorDetailResponse> {
    return this.client.detail(id);
  }
}

export function createConnectorDetailClient(options: ConnectorsClientOptions): ConnectorDetailClient {
  return new ConnectorDetailClient(options);
}
