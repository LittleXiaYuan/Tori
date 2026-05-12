/** Lightweight cron-control SDK facade over the Cron slice. */
import {
  CronClient,
  CronClientError,
  createCronClient,
  type CronAddRequest,
  type CronAddResponse,
  type CronClientOptions,
  type CronDeliveryMode,
  type CronPayload,
  type CronPayloadKind,
  type CronRemoveResponse,
  type CronRunRecord,
  type CronRunResponse,
  type CronRunStatus,
  type CronSchedule,
  type CronScheduleType,
} from "./cron.js";

export type {
  CronAddRequest,
  CronAddResponse,
  CronClientOptions as CronControlClientOptions,
  CronDeliveryMode,
  CronPayload,
  CronPayloadKind,
  CronRemoveResponse,
  CronRunRecord,
  CronRunResponse,
  CronRunStatus,
  CronSchedule,
  CronScheduleType,
};

export { CronClientError as CronControlClientError };

export class CronControlClient {
  private readonly client: CronClient;

  constructor(options: CronClientOptions) {
    this.client = createCronClient(options);
  }

  add(request: CronAddRequest): Promise<CronAddResponse> {
    return this.client.add(request);
  }

  remove(id: string): Promise<CronRemoveResponse> {
    return this.client.remove(id);
  }

  run(id: string): Promise<CronRunResponse> {
    return this.client.run(id);
  }
}

export function createCronControlClient(options: CronClientOptions): CronControlClient {
  return new CronControlClient(options);
}
