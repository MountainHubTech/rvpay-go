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

import { getClientsBaseUrl, getTransactionsBaseUrl } from "@/lib/environments"

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

async function getJson<T>(path: string, baseUrl: string): Promise<T> {
  const response = await fetch(`${baseUrl}${path}`, { method: "GET" });
  if (!response.ok) {
    throw new Error(`Request failed (${response.status}): ${path}`);
  }
  return response.json() as Promise<T>;
}

// Transactions-service endpoints (overview snapshot, payout stats, payout
// list): resolved against the selected environment's transactionsBaseUrl.
export function fetchOverviewSnapshot(
  period: OverviewPeriod
): Promise<OverviewSnapshotResponse> {
  return getJson(
    `/v1/public/overview/snapshot?period=${encodeURIComponent(period)}`,
    getTransactionsBaseUrl()
  );
}

export function fetchPayoutOverviewStats(): Promise<PayoutStatsResponse> {
  return getJson(
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
  return getJson(`/v1/public/payouts?${query.toString()}`, getTransactionsBaseUrl());
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
  return getJson(`/v1/public/sub-accounts?${query.toString()}`, getClientsBaseUrl());
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