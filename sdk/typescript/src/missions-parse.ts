/** Lightweight missions-parse SDK facade over the Missions slice. */
import {
  MissionsClient,
  MissionsClientError,
  createMissionsClient,
  type MissionParseResult,
  type MissionsClientOptions,
} from "./missions.js";

export type {
  MissionParseResult,
  MissionsClientOptions as MissionsParseClientOptions,
};

export { MissionsClientError as MissionsParseClientError };

export class MissionsParseClient {
  private readonly client: MissionsClient;

  constructor(options: MissionsClientOptions) { this.client = createMissionsClient(options); }
  parse(description: string): Promise<MissionParseResult> { return this.client.parse(description); }
}

export function createMissionsParseClient(options: MissionsClientOptions): MissionsParseClient { return new MissionsParseClient(options); }
