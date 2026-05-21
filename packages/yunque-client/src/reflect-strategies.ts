/** Lightweight reflect-strategies SDK facade over the Missions reflection slice. */
import {
  MissionsClient,
  MissionsClientError,
  createMissionsClient,
  type ExperienceOutcome,
  type ExperienceSource,
  type MissionsClientOptions,
  type StrategiesOptions,
  type StrategiesResponse,
} from "./missions.js";

export type {
  ExperienceOutcome,
  ExperienceSource,
  MissionsClientOptions as ReflectStrategiesClientOptions,
  StrategiesOptions as ReflectStrategiesOptions,
  StrategiesResponse as ReflectStrategiesResponse,
};

export { MissionsClientError as ReflectStrategiesClientError };

export class ReflectStrategiesClient {
  private readonly client: MissionsClient;

  constructor(options: MissionsClientOptions) { this.client = createMissionsClient(options); }
  strategies(options: StrategiesOptions = {}): Promise<StrategiesResponse> { return this.client.strategies(options); }
}

export function createReflectStrategiesClient(options: MissionsClientOptions): ReflectStrategiesClient { return new ReflectStrategiesClient(options); }
