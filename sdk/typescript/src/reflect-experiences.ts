/** Lightweight reflect-experiences SDK facade over the Missions reflection slice. */
import {
  MissionsClient,
  MissionsClientError,
  createMissionsClient,
  type ExperienceListOptions,
  type ExperienceOutcome,
  type ExperienceSource,
  type ExperienceStatsOptions,
  type ExperienceStatsResponse,
  type ExperiencesResponse,
  type MissionsClientOptions,
  type ReflectExperience,
} from "./missions.js";

export type {
  ExperienceListOptions as ReflectExperiencesListOptions,
  ExperienceOutcome,
  ExperienceSource,
  ExperienceStatsOptions as ReflectExperiencesStatsOptions,
  ExperienceStatsResponse as ReflectExperiencesStatsResponse,
  ExperiencesResponse as ReflectExperiencesResponse,
  MissionsClientOptions as ReflectExperiencesClientOptions,
  ReflectExperience,
};

export { MissionsClientError as ReflectExperiencesClientError };

export class ReflectExperiencesClient {
  private readonly client: MissionsClient;

  constructor(options: MissionsClientOptions) { this.client = createMissionsClient(options); }
  list(options: ExperienceListOptions = {}): Promise<ExperiencesResponse> { return this.client.experiences(options); }
  stats(options: ExperienceStatsOptions = {}): Promise<ExperienceStatsResponse> { return this.client.experienceStats(options); }
}

export function createReflectExperiencesClient(options: MissionsClientOptions): ReflectExperiencesClient { return new ReflectExperiencesClient(options); }
