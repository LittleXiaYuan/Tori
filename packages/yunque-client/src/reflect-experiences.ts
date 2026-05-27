/** Lightweight reflect-experiences SDK facade over the Missions reflection slice. */
import {
  MissionsClient,
  MissionsClientError,
  createMissionsClient,
  type ExperienceListOptions,
  type ExperienceCreateResponse,
  type ExperienceOutcome,
  type ExperienceSource,
  type ExperienceStatsOptions,
  type ExperienceStatsResponse,
  type ExperiencesResponse,
  type MissionsClientOptions,
  type ReflectExperience,
  type WorkloadFeedbackStatsOptions,
  type WorkloadFeedbackStatsResponse,
} from "./missions";

export type {
  ExperienceListOptions as ReflectExperiencesListOptions,
  ExperienceCreateResponse as ReflectExperienceCreateResponse,
  ExperienceOutcome,
  ExperienceSource,
  ExperienceStatsOptions as ReflectExperiencesStatsOptions,
  ExperienceStatsResponse as ReflectExperiencesStatsResponse,
  ExperiencesResponse as ReflectExperiencesResponse,
  MissionsClientOptions as ReflectExperiencesClientOptions,
  ReflectExperience,
  WorkloadFeedbackStatsOptions,
  WorkloadFeedbackStatsResponse,
};

export { MissionsClientError as ReflectExperiencesClientError };

export class ReflectExperiencesClient {
  private readonly client: MissionsClient;

  constructor(options: MissionsClientOptions) { this.client = createMissionsClient(options); }
  list(options: ExperienceListOptions = {}): Promise<ExperiencesResponse> { return this.client.experiences(options); }
  add(experience: ReflectExperience): Promise<ExperienceCreateResponse> { return this.client.addExperience(experience); }
  stats(options: ExperienceStatsOptions = {}): Promise<ExperienceStatsResponse> { return this.client.experienceStats(options); }
  workloadFeedbackStats(options: WorkloadFeedbackStatsOptions = {}): Promise<WorkloadFeedbackStatsResponse> { return this.client.workloadFeedbackStats(options); }
}

export function createReflectExperiencesClient(options: MissionsClientOptions): ReflectExperiencesClient { return new ReflectExperiencesClient(options); }
