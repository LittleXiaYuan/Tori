/** Lightweight cron-read SDK facade over the Cron slice. */
import {
  CronClient,
  CronClientError,
  createCronClient,
  type CronClientOptions,
  type CronDeliveryMode,
  type CronJob,
  type CronListResponse,
  type CronPayload,
  type CronPayloadKind,
  type CronRunRecord,
  type CronRunStatus,
  type CronSchedule,
  type CronScheduleType,
} from "./cron.js";

export type {
  CronClientOptions as CronReadClientOptions,
  CronDeliveryMode,
  CronJob,
  CronListResponse,
  CronPayload,
  CronPayloadKind,
  CronRunRecord,
  CronRunStatus,
  CronSchedule,
  CronScheduleType,
};

export { CronClientError as CronReadClientError };

export class CronReadClient {
  private readonly client: CronClient;

  constructor(options: CronClientOptions) {
    this.client = createCronClient(options);
  }

  list(): Promise<CronListResponse> {
    return this.client.list();
  }
}

export function createCronReadClient(options: CronClientOptions): CronReadClient {
  return new CronReadClient(options);
}
