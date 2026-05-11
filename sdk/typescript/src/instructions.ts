/** Lightweight user-instructions SDK facade over the interactions slice. */
import {
  createInteractionsClient,
  InteractionsClient,
  InteractionsClientError,
  type InstructionStatusResponse,
  type InstructionsResponse,
  type InteractionsClientOptions,
  type UserInstruction,
} from "./interactions.js";

export type {
  InstructionStatusResponse,
  InstructionsResponse,
  InteractionsClientOptions as InstructionsClientOptions,
  UserInstruction,
};

export { InteractionsClientError as InstructionsClientError };

export class InstructionsClient {
  private readonly client: InteractionsClient;

  constructor(options: InteractionsClientOptions) {
    this.client = createInteractionsClient(options);
  }

  list(category?: string): Promise<InstructionsResponse> {
    return this.client.instructions(category);
  }

  create(instruction: UserInstruction): Promise<UserInstruction> {
    return this.client.createInstruction(instruction);
  }

  update(instruction: UserInstruction): Promise<InstructionStatusResponse> {
    return this.client.updateInstruction(instruction);
  }

  delete(id: string): Promise<InstructionStatusResponse> {
    return this.client.deleteInstruction(id);
  }

  reorder(ids: string[]): Promise<InstructionStatusResponse> {
    return this.client.reorderInstructions(ids);
  }
}

export function createInstructionsClient(options: InteractionsClientOptions): InstructionsClient {
  return new InstructionsClient(options);
}
