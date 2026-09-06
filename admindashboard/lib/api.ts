// Server connection layer for the RVPAY Admin Dashboard.
//
// All requests target the existing Transactions (overview, payouts) and
// Clients (sub-accounts) grpc-gateway endpoints. Base URLs come from the
// centralized environment configuration (lib/environments.ts) and are
// resolved PER REQUEST, so the environment selected on the Settings page
// takes effect on the next request without a rebuild or reload.
//
// No data is fabricated: fields the backend documents as cross-service gaps
// are surfaced as "—" by the calling pages, never invented.

import {
  ENVIRONMENTS,
  getClientsBaseUrl,
  getSelectedEnvironment,
  getTransactionsBaseUrl,
} from "@/lib/environments"

// Diagnostic logging for the Dashboard ↔ backend connection. Request-start
// logs are development-only so production consoles stay quiet; failures are
// always logged (console.warn) because they are exactly what we need when a
// deployed dashboard shows empty data. Never logs secrets or payload data —
// only environment name, service name, URL, method, status and error identity.
type ApiService = "clients" | "transactions"

const API_DEBUG = process.env.NODE_ENV !== "production"

function logApiRequest(environment: string, service: ApiService, method: string, url: string): void {
  if (API_DEBUG) {
    console.info(`[RVPay API] environment=${environment} service=${service}`)
    console.info(`[RVPay API] ${method} ${url}`)
  }
}

function logApiResponse(status: number, durationMs: number): void {
  if (API_DEBUG) {
    console.info(`[RVPay API] response status=${status} duration_ms=${durationMs}`)
  }
}

function logApiFailure(environment: string, service: ApiService, url: string, errorName: string, message: string): void {
  console.warn(
    `[RVPay API] request failed environment=${environment} service=${service} url=${url} error=${errorName} message=${message}`
  )
}

// Why a request failed. The browser collapses several very different problems
// into "TypeError: Failed to fetch"; this classification keeps them apart:
//   network — fetch itself threw (server unreachable, CORS-blocked response,
//             or mixed-content blocked). No HTTP status exists.
//   http    — the server answered with a non-2xx status.
//   parse   — the server answered 2xx but the body was not valid JSON.
export type ApiErrorKind = "network" | "http" | "parse"

export class ApiRequestError extends Error {
  readonly kind: ApiErrorKind
  readonly url: string
  readonly service: ApiService
  readonly status?: number

  constructor(kind: ApiErrorKind, url: string, service: ApiService, message: string, status?: number) {
    super(message)
    this.name = "ApiRequestError"
    this.kind = kind
    this.url = url
    this.service = service
    this.status = status
  }
}

export type OverviewPeriod = "7d" | "30d" | "90d" | "ytd";

export type OverviewSnapshotResponse = {
  totalRevenue: number;
  revenueCurrency: string;
  transactionVolume: number;
  activeSubAccounts: number;
  pendingPayouts: number;
  needsAttention: string[];
  revenueOverTime: Array<{ periodLabel: string; revenue: number }>;
  recentPayouts: Array<{
    subAccount: string;
    amount: string;
    status: string;
    date: string;
  }>;
};

export type PayoutStatsResponse = {
  stats: Array<{ label: string; value: string; meta: string }>;
};

export type PayoutListResponse = {
  rows: Array<{
    id: string;
    initials: string;
    name: string;
    amount: string;
    status: string;
    initiated: string;
    expectedOrCleared: string;
    expectedOrClearedStrong: boolean;
  }>;
  total: number;
  page: number;
  pageSize: number;
};

export type SubAccountListResponse = {
  rows: Array<{
    id: string;
    name: string;
    location: string;
    initials: string;
    status: string;
    balance: string;
    lastPayoutDate: string;
    totalProcessed: string;
  }>;
  total: number;
  page: number;
  pageSize: number;
};

async function getJson<T>(service: ApiService, path: string, baseUrl: string): Promise<T> {
  const url = `${baseUrl}${path}`
  const environment = getSelectedEnvironment()
  const environmentLabel = ENVIRONMENTS[environment].label
  logApiRequest(environmentLabel, service, "GET", url)

  const startedAt = Date.now()
  let response: Response
  try {
    response = await fetch(url, { method: "GET" })
  } catch (error) {
    const errorName = error instanceof Error ? error.name : "UnknownError"
    const message = error instanceof Error ? error.message : String(error)
    logApiFailure(environmentLabel, service, url, errorName, message)
    // The browser throws TypeError for: unreachable server, blocked
    // mixed-content, or a CORS-rejected response. No HTTP status exists, so
    // surface the URL and a truthful hint rather than guessing one cause.
    throw new ApiRequestError(
      "network",
      url,
      service,
      `Network request to ${service} failed (${errorName}). The service may be unreachable, the URL may be blocked (mixed content or CORS), or the environment selection may point at a host this browser cannot reach.`,
      undefined
    )
  }
  logApiResponse(response.status, Date.now() - startedAt)

  if (!response.ok) {
    throw new ApiRequestError(
      "http",
      url,
      service,
      `${service} request failed with HTTP ${response.status}: ${path}`,
      response.status
    )
  }

  try {
    return (await response.json()) as T
  } catch (error) {
    const errorName = error instanceof Error ? error.name : "UnknownError"
    const message = error instanceof Error ? error.message : String(error)
    logApiFailure(environmentLabel, service, url, errorName, message)
    throw new ApiRequestError(
      "parse",
      url,
      service,
      `${service} returned HTTP ${response.status} but the body was not valid JSON.`,
      response.status
    )
  }
}

// Transactions-service endpoints (overview snapshot, payout stats, payout
// list): resolved against the selected environment's transactionsBaseUrl.
export function fetchOverviewSnapshot(
  period: OverviewPeriod
): Promise<OverviewSnapshotResponse> {
  return getJson(
    "transactions",
    `/v1/public/overview/snapshot?period=${encodeURIComponent(period)}`,
    getTransactionsBaseUrl()
  );
}

export function fetchPayoutOverviewStats(): Promise<PayoutStatsResponse> {
  return getJson(
    "transactions",
    `/v1/public/payouts/overview/stats`,
    getTransactionsBaseUrl()
  );
}

export function fetchPayouts(params: {
  search?: string;
  status?: string;
  page?: number;
  pageSize?: number;
}): Promise<PayoutListResponse> {
  const query = new URLSearchParams();
  if (params.search) query.set("search", params.search);
  if (params.status) query.set("status", params.status);
  query.set("page", String(params.page ?? 1));
  query.set("pageSize", String(params.pageSize ?? 20));
  return getJson("transactions", `/v1/public/payouts?${query.toString()}`, getTransactionsBaseUrl());
}

export function fetchSubAccounts(params: {
  search?: string;
  status?: string;
  sort?: string;
  order?: string;
  page?: number;
  pageSize?: number;
}): Promise<SubAccountListResponse> {
  const query = new URLSearchParams();
  if (params.search) query.set("search", params.search);
  if (params.status) query.set("status", params.status);
  if (params.sort) query.set("sort", params.sort);
  if (params.order) query.set("order", params.order);
  query.set("page", String(params.page ?? 1));
  query.set("pageSize", String(params.pageSize ?? 20));
  // Clients-service endpoint: resolved against the selected environment's
  // clientsBaseUrl (independent from the transactionsBaseUrl — Local uses
  // :8080 for Clients and :8081 for Transactions).
  return getJson("clients", `/v1/public/sub-accounts?${query.toString()}`, getClientsBaseUrl());
}

// Lightweight reachability probe used by the Settings "Test Connection"
// buttons. Uses the services' EXISTING healthcheck endpoints — no new backend
// endpoints. Returns the HTTP status, or null status when the browser could
// not complete the request at all (unreachable, CORS-blocked, mixed-content).
export async function probeService(
  service: ApiService
): Promise<{ ok: boolean; status: number | null; errorName: string | null; message: string | null }> {
  const baseUrl = service === "clients" ? getClientsBaseUrl() : getTransactionsBaseUrl()
  const path =
    service === "clients"
      ? "/v1/public/clients/healthcheck"
      : "/v1/public/transactions/healthcheck"
  const url = `${baseUrl}${path}`
  const environmentLabel = ENVIRONMENTS[getSelectedEnvironment()].label
  logApiRequest(environmentLabel, service, "GET", url)
  const controller = new AbortController()
  const timeout = setTimeout(() => controller.abort(), 5000)
  try {
    const response = await fetch(url, { method: "GET", signal: controller.signal })
    logApiResponse(response.status, 0)
    return { ok: response.ok, status: response.status, errorName: null, message: null }
  } catch (error) {
    const errorName = error instanceof Error ? error.name : "UnknownError"
    const message = error instanceof Error ? error.message : String(error)
    logApiFailure(environmentLabel, service, url, errorName, message)
    return { ok: false, status: null, errorName, message }
  } finally {
    clearTimeout(timeout)
  }
}

// describeApiError converts an ApiRequestError into a short, truthful
// user-facing reason appended to a page's existing error banner. Non-API
// errors fall back to the unchanged banner text.
export function describeApiError(error: unknown, fallback: string): string {
  if (error instanceof ApiRequestError) {
    if (error.kind === "http") {
      return `${fallback} (HTTP ${error.status} from ${error.service})`
    }
    if (error.kind === "parse") {
      return `${fallback} (${error.service} returned an invalid JSON body)`
    }
    return `${fallback} (${error.service} could not be reached from this browser — check the selected environment in Settings, that the service is running, and mixed-content/CORS restrictions)`
  }
  return fallback
}

// DashboardPeriod (UI label) → overview period wire value.
export function periodToWire(
  period: "Last 7 Days" | "Last 30 Days" | "Last 90 Days" | "This Year"
): OverviewPeriod {
  switch (period) {
    case "Last 30 Days":
      return "30d";
    case "Last 90 Days":
      return "90d";
    case "This Year":
      return "ytd";
    default:
      return "7d";
  }
}