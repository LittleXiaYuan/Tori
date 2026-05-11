/** Lightweight goal-state SDK facade over the State slice. */
import {
  createStateClient,
  StateClient,
  StateClientError,
  type StateClientOptions,
  type StateGoal,
  type StateGoalMutationResponse,
  type StateGoalsResponse,
} from "./state.js";

export type {
  StateClientOptions as GoalStateClientOptions,
  StateGoal,
  StateGoalMutationResponse,
  StateGoalsResponse,
};

export { StateClientError as GoalStateClientError };

export class GoalStateClient {
  private readonly client: StateClient;

  constructor(options: StateClientOptions) {
    this.client = createStateClient(options);
  }

  list(): Promise<StateGoalsResponse> {
    return this.client.goals();
  }

  save(goal: StateGoal): Promise<StateGoalMutationResponse> {
    return this.client.saveGoal(goal);
  }

  delete(id: string): Promise<StateGoalMutationResponse> {
    return this.client.deleteGoal(id);
  }
}

export function createGoalStateClient(options: StateClientOptions): GoalStateClient {
  return new GoalStateClient(options);
}
