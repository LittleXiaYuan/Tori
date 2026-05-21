/** Lightweight skills-dynamic SDK facade over dynamic skill review. */
import {
  SkillsClient,
  SkillsClientError,
  createSkillsClient,
  type DynamicSkill,
  type DynamicSkillsResponse,
  type SkillApproveRequest,
  type SkillMutationResponse,
  type SkillRejectRequest,
  type SkillsClientOptions,
} from "./skills.js";

export type {
  DynamicSkill,
  DynamicSkillsResponse,
  SkillApproveRequest,
  SkillMutationResponse,
  SkillRejectRequest,
  SkillsClientOptions as SkillsDynamicClientOptions,
};

export { SkillsClientError as SkillsDynamicClientError };

export class SkillsDynamicClient {
  private readonly client: SkillsClient;

  constructor(options: SkillsClientOptions) { this.client = createSkillsClient(options); }
  list(): Promise<DynamicSkillsResponse> { return this.client.dynamic(); }
  approve(request: SkillApproveRequest): Promise<SkillMutationResponse> { return this.client.approve(request); }
  reject(nameOrRequest: string | SkillRejectRequest): Promise<SkillMutationResponse> { return this.client.reject(nameOrRequest); }
}

export function createSkillsDynamicClient(options: SkillsClientOptions): SkillsDynamicClient { return new SkillsDynamicClient(options); }
