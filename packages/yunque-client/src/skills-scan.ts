/** Lightweight skills-scan SDK facade over runtime skill scanning. */
import {
  SkillsClient,
  SkillsClientError,
  createSkillsClient,
  type SkillScanResponse,
  type SkillsClientOptions,
} from "./skills.js";

export type {
  SkillScanResponse,
  SkillsClientOptions as SkillsScanClientOptions,
};

export { SkillsClientError as SkillsScanClientError };

export class SkillsScanClient {
  private readonly client: SkillsClient;

  constructor(options: SkillsClientOptions) { this.client = createSkillsClient(options); }
  scan(): Promise<SkillScanResponse> { return this.client.scan(); }
}

export function createSkillsScanClient(options: SkillsClientOptions): SkillsScanClient { return new SkillsScanClient(options); }
