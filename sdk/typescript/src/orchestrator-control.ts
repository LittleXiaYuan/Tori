/** Lightweight orchestrator-control SDK facade over the Orchestrator slice. */
import {
  OrchestratorClient,
  OrchestratorClientError,
  createOrchestratorClient,
  type OrchestratorAdapterConfig,
  type OrchestratorAdapterResponse,
  type OrchestratorClientOptions,
  type OrchestratorPolicy,
  type OrchestratorPolicyUpdateResponse,
  type OrchestratorToggleAction,
  type OrchestratorToggleResponse,
} from "./orchestrator.js";

export type {
  OrchestratorAdapterConfig,
  OrchestratorAdapterResponse,
  OrchestratorClientOptions as OrchestratorControlClientOptions,
  OrchestratorPolicy,
  OrchestratorPolicyUpdateResponse,
  OrchestratorToggleAction,
  OrchestratorToggleResponse,
};

export { OrchestratorClientError as OrchestratorControlClientError };

export class OrchestratorControlClient {
  private readonly client: OrchestratorClient;
  constructor(options: OrchestratorClientOptions) { this.client = createOrchestratorClient(options); }
  toggle(action: OrchestratorToggleAction): Promise<OrchestratorToggleResponse> { return this.client.toggle(action); }
  updatePolicy(policy: OrchestratorPolicy): Promise<OrchestratorPolicyUpdateResponse> { return this.client.updatePolicy(policy); }
  addAdapter(config: OrchestratorAdapterConfig): Promise<OrchestratorAdapterResponse> { return this.client.addAdapter(config); }
}

export function createOrchestratorControlClient(options: OrchestratorClientOptions): OrchestratorControlClient { return new OrchestratorControlClient(options); }
