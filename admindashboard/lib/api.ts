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
import {
  dropSession,
  getAccessToken,
  getRefreshToken,
  setTokens,
  type AdminUser,
} from "@/lib/auth"

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

// TransactionListRow is a single row of the transactions list
// (GET /v1/public/transactions). All fields come from the Transactions
// service's ListTransactions RPC; nothing is fabricated.
export type TransactionListRow = {
  id: string;
  shortId: string;
  subAccount: string;
  customer: string;
  customerInitials: string;
  amount: string;
  status: string;
  gateway: string;
  date: string;
};

// TransactionsListResponse carries a page of transactions (deposits).
export type TransactionsListResponse = {
  rows: TransactionListRow[];
  total: number;
  page: number;
  pageSize: number;
};

// UserRow is a single user row from the team-management list
// (GET /v1/public/clients/users). Password and token material are never
// returned by the backend.
export type UserRow = {
  id: string;
  name: string;
  email: string;
  role: string;
  status: string;
  dateJoined: string;
};

// UsersListResponse carries a page of users.
export type UsersListResponse = {
  rows: UserRow[];
  total: number;
  page: number;
  pageSize: number;
};

// DisputeStatsResponse carries the Needs Response / Under Review counters.
export type DisputeStatsResponse = {
  needsResponse: number;
  underReview: number;
};

// DisputeRow is a single row of the disputes list
// (GET /v1/public/transactions/disputes). All fields come from the
// Transactions service's ListDisputes RPC; nothing is fabricated.
export type DisputeRow = {
  id: string;
  subAccount: string;
  type: string;
  amount: string;
  status: string;
  dateOpened: string;
  dueIn: string;
};

// DisputesListResponse carries a page of disputes.
export type DisputesListResponse = {
  rows: DisputeRow[];
  total: number;
  page: number;
  pageSize: number;
};

async function getJson<T>(service: ApiService, path: string, baseUrl: string): Promise<T> {
  return requestJson<T>(service, "GET", `${baseUrl}${path}`, path, baseUrl, false)
}

// requestJson is the shared transport. It attaches the bearer access token
// when one is held, and on a 401 attempts a single token refresh and retries
// once before surfacing the failure. Tokens and Authorization headers are
// never logged.
async function requestJson<T>(
  service: ApiService,
  method: "GET" | "POST",
  url: string,
  path: string,
  baseUrl: string,
  isRetry: boolean,
  body?: unknown
): Promise<T> {
  const environment = getSelectedEnvironment()
  const environmentLabel = ENVIRONMENTS[environment].label
  logApiRequest(environmentLabel, service, method, url)

  const startedAt = Date.now()
  let response: Response
  try {
    const headers: Record<string, string> = { "Content-Type": "application/json" }
    const token = getAccessToken()
    if (token) {
      headers.Authorization = `Bearer ${token}`
    }
    response = await fetch(url, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
    })
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

  if (response.status === 401 && !isRetry && path !== "/v1/public/auth/sign-in") {
    // Access token missing/expired: attempt one rotation and retry once.
    const refreshed = await tryRefresh()
    if (refreshed) {
      return requestJson<T>(service, method, url, path, baseUrl, true, body)
    }
    dropSession()
  }

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

// tryRefresh rotates the access token using the stored refresh token.
// Returns true when the session was rotated successfully; on any failure the
// stored session is dropped so guards route to sign-in.
let refreshInFlight: Promise<boolean> | null = null

function tryRefresh(): Promise<boolean> {
  if (!refreshInFlight) {
    refreshInFlight = performRefresh().finally(() => {
      refreshInFlight = null
    })
  }
  return refreshInFlight
}

async function performRefresh(): Promise<boolean> {
  const refreshToken = getRefreshToken()
  if (!refreshToken) return false
  const url = `${getClientsBaseUrl()}/v1/public/auth/refresh`
  try {
    const response = await fetch(url, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refreshToken }),
    })
    if (!response.ok) return false
    const data = (await response.json()) as {
      user: AdminUser
      accessToken: string
      refreshToken: string
    }
    if (!data.accessToken || !data.refreshToken) return false
    setTokens(data.accessToken, data.refreshToken)
    return true
  } catch {
    return false
  }
}

// signInRequest authenticates an administrator against the Clients service.
export type SignInWireResponse = {
  user: AdminUser
  accessToken: string
  refreshToken: string
  accessTokenExpiresAt: string
}

export async function signInRequest(email: string, password: string): Promise<SignInWireResponse> {
  return requestJson<SignInWireResponse>(
    "clients",
    "POST",
    `${getClientsBaseUrl()}/v1/public/auth/sign-in`,
    "/v1/public/auth/sign-in",
    getClientsBaseUrl(),
    false,
    { email, password }
  )
}

// signOutRequest revokes the presented access token server-side. Best-effort
// from the caller's perspective.
export async function signOutRequest(): Promise<void> {
  const refreshToken = getRefreshToken()
  try {
    await requestJson<{ [key: string]: never }>(
      "clients",
      "POST",
      `${getClientsBaseUrl()}/v1/public/auth/sign-out`,
      "/v1/public/auth/sign-out",
      getClientsBaseUrl(),
      false,
      { refreshToken }
    )
  } catch {
    // Sign-out is best-effort: local cleanup always continues.
  }
}

// Transactions-service endpoints (overview snapshot, payout stats, payout
// list): resolved against the selected environment's transactionsBaseUrl.
export function fetchOverviewSnapshot(
  period: OverviewPeriod
): Promise<OverviewSnapshotResponse> {
  return getJson(
    "transactions",
    `/v1/public/transactions/overview/snapshot?period=${encodeURIComponent(period)}`,
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
  // :8080 for Clients and :8081 for Transactions). The route lives under the
  // permitted /v1/public/clients* ALB prefix.
  return getJson("clients", `/v1/public/clients/sub-accounts?${query.toString()}`, getClientsBaseUrl());
}

// fetchTransactions loads a paginated, searchable, status-filtered list of
// customer deposits (the Transactions service's transaction records) for the
// Admin Dashboard transactions page. The route lives under the permitted
// /v1/public/transactions* ALB prefix and is protected by the admin middleware.
// Query params use the gRPC-gateway proto field names (page_size, sub_account).
export function fetchTransactions(params: {
  search?: string;
  status?: string;
  subAccount?: string;
  page?: number;
  pageSize?: number;
}): Promise<TransactionsListResponse> {
  const query = new URLSearchParams();
  if (params.search) query.set("search", params.search);
  if (params.status) query.set("status", params.status);
  if (params.subAccount) query.set("sub_account", params.subAccount);
  query.set("page", String(params.page ?? 1));
  query.set("page_size", String(params.pageSize ?? 20));
  return getJson("transactions", `/v1/public/transactions?${query.toString()}`, getTransactionsBaseUrl());
}

// fetchUsers loads a paginated, searchable, role-filtered list of
// database-managed users for the Admin Dashboard Settings team-management
// page. The route lives under the permitted /v1/public/clients* ALB prefix
// and is protected by the admin middleware. Query params use the gRPC-gateway
// proto field names (page_size, role).
export function fetchUsers(params: {
  search?: string;
  role?: string;
  page?: number;
  pageSize?: number;
}): Promise<UsersListResponse> {
  const query = new URLSearchParams();
  if (params.search) query.set("search", params.search);
  if (params.role) query.set("role", params.role);
  query.set("page", String(params.page ?? 1));
  query.set("page_size", String(params.pageSize ?? 20));
  return getJson("clients", `/v1/public/clients/users?${query.toString()}`, getClientsBaseUrl());
}

// fetchDisputeStats loads the Needs Response / Under Review counters for the
// Admin Dashboard disputes page. The route lives under the permitted
// /v1/public/transactions* ALB prefix and is protected by the admin middleware.
export function fetchDisputeStats(): Promise<DisputeStatsResponse> {
  return getJson("transactions", "/v1/public/transactions/disputes/stats", getTransactionsBaseUrl());
}

// fetchDisputes loads a paginated, searchable, status-filtered list of
// disputes for the Admin Dashboard disputes page. The route lives under the
// permitted /v1/public/transactions* ALB prefix and is protected by the admin
// middleware. Query params use the gRPC-gateway proto field names.
export function fetchDisputes(params: {
  search?: string;
  status?: string;
  page?: number;
  pageSize?: number;
}): Promise<DisputesListResponse> {
  const query = new URLSearchParams();
  if (params.search) query.set("search", params.search);
  if (params.status) query.set("status", params.status);
  query.set("page", String(params.page ?? 1));
  query.set("page_size", String(params.pageSize ?? 20));
  return getJson("transactions", `/v1/public/transactions/disputes?${query.toString()}`, getTransactionsBaseUrl());
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