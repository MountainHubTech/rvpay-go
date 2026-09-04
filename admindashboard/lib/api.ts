// Server connection layer for the RVPAY Admin Dashboard.
//
// All requests target the existing Transactions (overview, payouts) and
// Clients (sub-accounts) grpc-gateway endpoints. The API base URL follows the
// existing convention (hardcoded, same as app/payment/page.tsx).
//
// No data is fabricated: fields the backend documents as cross-service gaps
// are surfaced as "—" by the calling pages, never invented.

export const API_BASE = "https://api.rvpay.xyz";

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

async function getJson<T>(path: string): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, { method: "GET" });
  if (!response.ok) {
    throw new Error(`Request failed (${response.status}): ${path}`);
  }
  return response.json() as Promise<T>;
}

export function fetchOverviewSnapshot(
  period: OverviewPeriod
): Promise<OverviewSnapshotResponse> {
  return getJson(
    `/v1/public/overview/snapshot?period=${encodeURIComponent(period)}`
  );
}

export function fetchPayoutOverviewStats(): Promise<PayoutStatsResponse> {
  return getJson(`/v1/public/payouts/overview/stats`);
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
  return getJson(`/v1/public/payouts?${query.toString()}`);
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
  return getJson(`/v1/public/sub-accounts?${query.toString()}`);
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