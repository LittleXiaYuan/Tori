/**
 * Lightweight Persona SDK slice.
 *
 * This keeps persona identity, skills, and preset management usable without
 * importing the full generated OpenAPI SDK:
 *
 *   import { createPersonaClient } from "yunque-client/persona";
 */

export type PersonaSkill = {
  name?: string;
  description?: string;
  content?: string;
  [key: string]: unknown;
};

export type PersonaState = {
  identity?: string;
  soul?: string;
  skills?: PersonaSkill[];
  [key: string]: unknown;
};

export type UpdatePersonaRequest = {
  identity?: string;
  soul?: string;
};

export type PersonaStatusResponse = {
  status?: string;
  [key: string]: unknown;
};

export type PersonaSkillsResponse = {
  skills: PersonaSkill[];
};

export type AddPersonaSkillRequest = {
  name: string;
  description?: string;
  content?: string;
};

export type DeletePersonaSkillRequest = {
  name: string;
};

export type PersonaPreset = {
  id?: string;
  name?: string;
  description?: string;
  tone?: string;
  style?: string;
  greeting?: string;
  system_note?: string;
  features?: Record<string, boolean>;
  custom?: boolean;
  [key: string]: unknown;
};

export type ListPersonaPresetsResponse = {
  presets: PersonaPreset[];
  active: string;
};

export type SwitchPersonaPresetRequest = {
  id: string;
};

export type SwitchPersonaPresetResponse = PersonaStatusResponse & {
  active?: string;
};

export type AddCustomPersonaPresetRequest = {
  id: string;
  name: string;
  description?: string;
  tone?: string;
  style?: string;
  greeting?: string;
  system_note?: string;
  features?: Record<string, boolean>;
};

export type AddCustomPersonaPresetResponse = PersonaStatusResponse & {
  id?: string;
};

export type DeleteCustomPersonaPresetRequest = {
  id: string;
};

export type UpdatePersonaPresetFeaturesRequest = {
  id: string;
  features: Record<string, boolean>;
};

export type PersonaClientOptions = {
  baseUrl: string;
  token?: string;
  apiKey?: string;
  headers?: HeadersInit;
  fetch?: typeof fetch;
};

export class PersonaClientError extends Error {
  readonly status: number;
  readonly body: unknown;

  constructor(status: number, body: unknown, message?: string) {
    super(message || `Persona request failed with HTTP ${status}`);
    this.name = "PersonaClientError";
    this.status = status;
    this.body = body;
  }
}

function trimBaseUrl(baseUrl: string): string {
  return baseUrl.replace(/\/+$/, "");
}

function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers {
  const headers = new Headers(base);
  if (!extra) return headers;
  new Headers(extra).forEach((value, key) => headers.set(key, value));
  return headers;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function messageFromErrorBody(body: unknown): string | undefined {
  if (typeof body === "string" && body.trim()) return body.trim();
  if (!isRecord(body)) return undefined;
  for (const key of ["message", "detail", "error", "reason"]) {
    const value = body[key];
    if (typeof value === "string" && value.trim()) return value;
    if (key === "error" && isRecord(value)) {
      const nested = messageFromErrorBody(value);
      if (nested) return nested;
    }
  }
  return undefined;
}

async function parseResponse(response: Response): Promise<unknown> {
  const text = await response.text();
  if (!text) return undefined;
  try {
    return JSON.parse(text);
  } catch {
    return text;
  }
}

export class PersonaClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: PersonaClientOptions) {
    if (!options.baseUrl) throw new Error("PersonaClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("PersonaClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  get(): Promise<PersonaState> {
    return this.request<PersonaState>("GET", "/v1/persona");
  }

  update(body: UpdatePersonaRequest): Promise<PersonaStatusResponse> {
    return this.request<PersonaStatusResponse>("PUT", "/v1/persona", body);
  }

  skills(): Promise<PersonaSkillsResponse> {
    return this.request<PersonaSkillsResponse>("GET", "/v1/persona/skills");
  }

  addSkill(body: AddPersonaSkillRequest): Promise<PersonaStatusResponse> {
    return this.request<PersonaStatusResponse>("POST", "/v1/persona/skills", body);
  }

  deleteSkill(body: DeletePersonaSkillRequest): Promise<PersonaStatusResponse> {
    return this.request<PersonaStatusResponse>("DELETE", "/v1/persona/skills", body);
  }

  presets(): Promise<ListPersonaPresetsResponse> {
    return this.request<ListPersonaPresetsResponse>("GET", "/v1/persona/presets");
  }

  switchPreset(body: SwitchPersonaPresetRequest): Promise<SwitchPersonaPresetResponse> {
    return this.request<SwitchPersonaPresetResponse>("POST", "/v1/persona/presets", body);
  }

  addCustomPreset(body: AddCustomPersonaPresetRequest): Promise<AddCustomPersonaPresetResponse> {
    return this.request<AddCustomPersonaPresetResponse>("POST", "/v1/persona/presets/custom", body);
  }

  deleteCustomPreset(body: DeleteCustomPersonaPresetRequest): Promise<PersonaStatusResponse> {
    return this.request<PersonaStatusResponse>("DELETE", "/v1/persona/presets/custom", body);
  }

  updatePresetFeatures(body: UpdatePersonaPresetFeaturesRequest): Promise<PersonaStatusResponse> {
    return this.request<PersonaStatusResponse>("PUT", "/v1/persona/presets/features", body);
  }

  private async request<T>(method: "GET" | "PUT" | "POST" | "DELETE", path: string, body?: unknown): Promise<T> {
    const url = new URL(`${this.baseUrl}${path}`);
    const headers = mergeHeaders(this.headers);
    if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`);
    if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey);

    const init: RequestInit = { method, headers };
    if (body !== undefined) {
      headers.set("Content-Type", "application/json");
      init.body = JSON.stringify(body);
    }

    const response = await this.fetchImpl(url, init);
    const parsed = await parseResponse(response);
    if (!response.ok) throw new PersonaClientError(response.status, parsed, messageFromErrorBody(parsed));
    return parsed as T;
  }
}

export function createPersonaClient(options: PersonaClientOptions): PersonaClient {
  return new PersonaClient(options);
}
