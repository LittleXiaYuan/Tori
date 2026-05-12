/** Lightweight connector-disconnect SDK facade over the Connectors slice. */
import {
  createConnectorsClient,
  ConnectorsClient,
  ConnectorsClientError,
  type ConnectorDisconnectResponse,
  type ConnectorsClientOptions,
} from "./connectors.js";

export type {
  ConnectorDisconnectResponse,
  ConnectorsClientOptions as ConnectorDisconnectClientOptions,
};

export { ConnectorsClientError as ConnectorDisconnectClientError };

export class ConnectorDisconnectClient {
  private readonly client: ConnectorsClient;

  constructor(options: ConnectorsClientOptions) {
    this.client = createConnectorsClient(options);
  }

  disconnect(connectorId: string): Promise<ConnectorDisconnectResponse> {
    return this.client.disconnect(connectorId);
  }
}

export function createConnectorDisconnectClient(options: ConnectorsClientOptions): ConnectorDisconnectClient {
  return new ConnectorDisconnectClient(options);
}
