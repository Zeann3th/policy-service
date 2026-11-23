import type {
  CustomPolicy,
  DryRunRequest,
  DryRunResponse,
  Page,
  Policy,
  PolicyGroup,
  PolicyType,
  PolicyValue,
  ReloadResponse,
  Rule,
} from "./types";

export const API_BASE = process.env.NEXT_PUBLIC_POLICY_API ?? "http://localhost:8080/policy-service/api";

// Every write is attributed to this actor until a gateway supplies the user.
export const ACTOR = "admin";

// The service caps page size at 100.
export const PAGE_LIMIT = 100;

export interface LogEntry {
  id: number;
  method: string;
  path: string;
  status: number;
  ms: number;
  at: Date;
  request?: unknown;
  response?: unknown;
}

type Listener = (entry: LogEntry) => void;
const listeners = new Set<Listener>();
let sequence = 0;

/** Subscribes to every API call; returns the unsubscribe function. */
export function onRequest(listener: Listener): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export class ApiFailure extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
  ) {
    super(message);
  }
}

async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
  const started = performance.now();
  let status = 0;
  let payload: unknown;
  try {
    const response = await fetch(`${API_BASE}${path}`, {
      method,
      headers: body === undefined ? undefined : { "Content-Type": "application/json" },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    status = response.status;
    const text = await response.text();
    payload = text ? JSON.parse(text) : undefined;
    if (!response.ok) {
      const error = payload as { code?: string; message?: string } | undefined;
      throw new ApiFailure(status, error?.code ?? "ERROR", error?.message ?? response.statusText);
    }
    return payload as T;
  } catch (error) {
    if (!(error instanceof ApiFailure)) {
      payload = { message: String(error) };
      throw new ApiFailure(0, "NETWORK", `Cannot reach the policy service at ${API_BASE}. Is it running, and does CORS_ALLOWED_ORIGINS list this origin?`);
    }
    throw error;
  } finally {
    const entry: LogEntry = { id: ++sequence, method, path, status, ms: Math.round(performance.now() - started), at: new Date(), request: body, response: payload };
    console.info(`[policy-api] ${method} ${path} -> ${status}`, { request: body, response: payload });
    listeners.forEach((listener) => listener(entry));
  }
}

const actor = { updatedBy: ACTOR };
const page = `?page=1&limit=${PAGE_LIMIT}`;
const enc = encodeURIComponent;

export interface PolicyTypeInput {
  code: string;
  description?: string;
  kind: string;
  active: boolean;
}

export interface PolicyUpdate {
  base?: string;
  operator?: string;
  value: string;
  effectiveFrom: string;
  effectiveTo?: string;
  reason?: string;
}

export const api = {
  types: {
    list: () => call<Page<PolicyType>>("GET", `/v1/policies/types${page}`),
    create: (input: PolicyTypeInput) => call<PolicyType>("POST", "/v1/policies/types", { ...input, ...actor }),
    update: (id: number, input: { description?: string; active: boolean }) => call<PolicyType>("PUT", `/v1/policies/types/${id}`, { ...input, ...actor }),
    remove: (id: number) => call<void>("DELETE", `/v1/policies/types/${id}`, actor),
  },
  groups: {
    list: () => call<Page<PolicyGroup>>("GET", `/v1/policies/groups${page}`),
    get: (id: number) => call<PolicyGroup>("GET", `/v1/policies/groups/${id}`),
    create: (input: { walletType: string; currency: string; tier: string }) => call<PolicyGroup>("POST", "/v1/policies/groups", { ...input, ...actor }),
    remove: (id: number) => call<void>("DELETE", `/v1/policies/groups/${id}`, actor),
  },
  policies: {
    list: (groupId: number) => call<Page<Policy>>("GET", `/v1/policies/groups/${groupId}/policies${page}`),
    create: (groupId: number, input: Partial<PolicyValue>) => call<Policy>("POST", `/v1/policies/groups/${groupId}/policies`, { ...input, ...actor }),
    update: (groupId: number, id: number, input: PolicyUpdate) => call<Policy>("PUT", `/v1/policies/groups/${groupId}/policies/${id}`, { ...input, ...actor }),
    remove: (groupId: number, id: number) => call<void>("DELETE", `/v1/policies/groups/${groupId}/policies/${id}`, actor),
  },
  custom: {
    list: (walletId: string) => call<Page<CustomPolicy>>("GET", `/v1/policies/custom/${enc(walletId)}${page}`),
    create: (walletId: string, input: Partial<PolicyValue> & { reason?: string }) => call<CustomPolicy>("POST", `/v1/policies/custom/${enc(walletId)}`, { ...input, ...actor }),
    update: (walletId: string, id: number, input: PolicyUpdate) => call<CustomPolicy>("PUT", `/v1/policies/custom/${enc(walletId)}/${id}`, { ...input, ...actor }),
    remove: (walletId: string, id: number) => call<void>("DELETE", `/v1/policies/custom/${enc(walletId)}/${id}`, actor),
  },
  config: (query: { walletType: string; currency: string; tier: string; walletId: string; transactionType?: string }) => {
    const params = new URLSearchParams(Object.entries(query).filter(([, value]) => value) as [string, string][]);
    return call<{ rules: Rule[] }>("GET", `/v1/policies?${params}`);
  },
  cache: {
    reload: (input: { key?: string; namespaces?: string[] }) => call<ReloadResponse>("POST", "/v1/cache/reload", { ...input, ...actor }),
  },
  dryRun: (request: DryRunRequest) => call<DryRunResponse>("POST", "/v1/reservations/dry-run", request),
};
