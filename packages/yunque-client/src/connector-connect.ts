/** Lightweight connector-connect SDK facade over the Connectors slice. */
import {
  createConnectorsClient,
  ConnectorsClient,
  ConnectorsClientError,
  type ConnectorConnectRequest,
  type ConnectorConnectResponse,
  type ConnectorStatus,
  type ConnectorsClientOptions,
} from "./connectors.js";

export type {
  ConnectorConnectRequest,
  ConnectorConnectResponse,
  ConnectorStatus,
  ConnectorsClientOptions as ConnectorConnectClientOptions,
};

export { ConnectorsClientError as ConnectorConnectClientError };

export class ConnectorConnectClient {
  private readonly client: ConnectorsClient;

  constructor(options: ConnectorsClientOptions) {
    this.client = createConnectorsClient(options);
  }

  connect(request: ConnectorConnectRequest): Promise<ConnectorConnectResponse> {
    return this.client.connect(request);
  }
}

export function createConnectorConnectClient(options: ConnectorsClientOptions): ConnectorConnectClient {
  return new ConnectorConnectClient(options);
}
