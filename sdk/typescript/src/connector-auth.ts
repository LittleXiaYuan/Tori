/** Lightweight connector-auth SDK facade over the Connectors slice. */
import {
  createConnectorsClient,
  ConnectorsClient,
  ConnectorsClientError,
  type ConnectorConnectRequest,
  type ConnectorConnectResponse,
  type ConnectorDisconnectResponse,
  type ConnectorStatus,
  type ConnectorsClientOptions,
} from "./connectors.js";

export type {
  ConnectorConnectRequest,
  ConnectorConnectResponse,
  ConnectorDisconnectResponse,
  ConnectorStatus,
  ConnectorsClientOptions as ConnectorAuthClientOptions,
};

export { ConnectorsClientError as ConnectorAuthClientError };

export class ConnectorAuthClient {
  private readonly client: ConnectorsClient;

  constructor(options: ConnectorsClientOptions) {
    this.client = createConnectorsClient(options);
  }

  connect(request: ConnectorConnectRequest): Promise<ConnectorConnectResponse> {
    return this.client.connect(request);
  }

  disconnect(connectorId: string): Promise<ConnectorDisconnectResponse> {
    return this.client.disconnect(connectorId);
  }
}

export function createConnectorAuthClient(options: ConnectorsClientOptions): ConnectorAuthClient {
  return new ConnectorAuthClient(options);
}
