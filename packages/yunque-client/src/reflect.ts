/** Lightweight Reflect SDK slice for experience retrieval and strategy context. */
import {
  createMissionsClient,
  MissionsClient,
  MissionsClientError,
  type ExperienceListOptions,
  type ExperiencesResponse,
  type ExperienceStatsOptions,
  type ExperienceStatsResponse,
  type MissionsClientOptions,
  type StrategiesOptions,
  type StrategiesResponse,
  type ReflectExperience,
  type ExperienceOutcome,
  type ExperienceSource,
} from "./missions";

export type {
  ExperienceListOptions as ReflectExperienceListOptions,
  ExperiencesResponse as ReflectExperiencesResponse,
  ExperienceStatsOptions as ReflectExperienceStatsOptions,
  ExperienceStatsResponse as ReflectExperienceStatsResponse,
  StrategiesOptions as ReflectStrategiesOptions,
  StrategiesResponse as ReflectStrategiesResponse,
  ReflectExperience,
  ExperienceOutcome,
  ExperienceSource,
  MissionsClientOptions as ReflectClientOptions,
};

export { MissionsClientError as ReflectClientError };

export class ReflectClient {
  private readonly client: MissionsClient;

  constructor(options: MissionsClientOptions) {
    this.client = createMissionsClient(options);
  }

  experiences(options: ExperienceListOptions = {}): Promise<ExperiencesResponse> {
    return this.client.experiences(options);
  }

  experienceStats(options: ExperienceStatsOptions = {}): Promise<ExperienceStatsResponse> {
    return this.client.experienceStats(options);
  }

  strategies(options: StrategiesOptions = {}): Promise<StrategiesResponse> {
    return this.client.strategies(options);
  }
}

export function createReflectClient(options: MissionsClientOptions): ReflectClient {
  return new ReflectClient(options);
}
