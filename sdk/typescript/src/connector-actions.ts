/** Lightweight connector-actions SDK facade over the Connectors slice. */
import {
  createConnectorsClient,
  ConnectorsClient,
  ConnectorsClientError,
  type ConnectorExecuteRequest,
  type ConnectorExecuteResponse,
  type ConnectorsClientOptions,
} from "./connectors.js";

export type {
  ConnectorExecuteRequest,
  ConnectorExecuteResponse,
  ConnectorsClientOptions as ConnectorActionsClientOptions,
};

export { ConnectorsClientError as ConnectorActionsClientError };

export class ConnectorActionsClient {
  private readonly client: ConnectorsClient;

  constructor(options: ConnectorsClientOptions) {
    this.client = createConnectorsClient(options);
  }

  execute<T = unknown>(request: ConnectorExecuteRequest): Promise<ConnectorExecuteResponse<T>> {
    return this.client.execute<T>(request);
  }
}

export function createConnectorActionsClient(options: ConnectorsClientOptions): ConnectorActionsClient {
  return new ConnectorActionsClient(options);
}
