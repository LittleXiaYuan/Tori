/** Lightweight skills-suggestions SDK facade over session skill suggestions. */
import {
  SkillsClient,
  SkillsClientError,
  createSkillsClient,
  type SkillSuggestion,
  type SkillSuggestionsResponse,
  type SkillsClientOptions,
} from "./skills.js";

export type {
  SkillSuggestion,
  SkillSuggestionsResponse,
  SkillsClientOptions as SkillsSuggestionsClientOptions,
};

export { SkillsClientError as SkillsSuggestionsClientError };

export class SkillsSuggestionsClient {
  private readonly client: SkillsClient;

  constructor(options: SkillsClientOptions) { this.client = createSkillsClient(options); }
  suggestions(sessionId?: string): Promise<SkillSuggestionsResponse> { return this.client.suggestions(sessionId); }
}

export function createSkillsSuggestionsClient(options: SkillsClientOptions): SkillsSuggestionsClient { return new SkillsSuggestionsClient(options); }
