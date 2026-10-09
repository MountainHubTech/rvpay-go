"use client"

import * as React from "react"

import { AppSidebar } from "@/components/dashboard/app-sidebar"
import { AuthGuard } from "@/lib/auth"
import { Topbar } from "@/components/dashboard/topbar"
import { TransactionsView } from "@/components/dashboard/transactions-view"
import {
  describeApiError,
  fetchOverviewSnapshot,
  fetchSubAccounts,
  fetchTransactions,
  type SubAccountListResponse,
  type TransactionsListResponse,
} from "@/lib/api"

const TRANSACTIONS_STATUSES = ["Success", "Failed", "Refunded", "Pending"] as const
type TransactionsStatus = (typeof TRANSACTIONS_STATUSES)[number]

// Wire a TransactionsOverviewSnapshot stat into the USD presentation format.
// Formatting happens here (presentation layer) — the backend value is never
// altered.
function formatUsd(amount: number): string {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
  }).format(amount)
}

export default function TransactionsPage() {
  // Stat cards come from the EXISTING overview snapshot endpoint (30d period
  // matches the design's "Total Cleared (30d)"); no new backend is invented.
  const [totalPending, setTotalPending] = React.useState<number | null>(null)
  const [totalCleared, setTotalCleared] = React.useState<number | null>(null)
  const [currency, setCurrency] = React.useState("USD")
  const [statsError, setStatsError] = React.useState<string | null>(null)

  // Sub-Account filter options come from the EXISTING sub-accounts endpoint.
  const [subAccounts, setSubAccounts] = React.useState<
    SubAccountListResponse["rows"]
  >([])
  const [subAccount, setSubAccount] = React.useState("all")
  const [status, setStatus] = React.useState<"all" | TransactionsStatus>("all")
  const [dateRange, setDateRange] = React.useState("Last 30 Days")
  const [applied, setApplied] = React.useState<{ status: string; dateRange: string }>({
    status: "all",
    dateRange: "Last 30 Days",
  })

  // Transaction list state — driven by the EXISTING ListTransactions RPC
  // (GET /v1/public/transactions). Rows, total and loading come from the
  // backend; pagination is client-driven and re-fetches on filter/page change.
  const [rows, setRows] = React.useState<TransactionsListResponse["rows"]>([])
  const [loading, setLoading] = React.useState(false)
  const [total, setTotal] = React.useState(0)
  const [page, setPage] = React.useState(1)
  const pageSize = 20
  const [listError, setListError] = React.useState<string | null>(null)

  React.useEffect(() => {
    let cancelled = false
    fetchOverviewSnapshot("30d")
      .then((snapshot) => {
        if (cancelled) return
        setTotalPending(snapshot.pendingPayouts)
        setTotalCleared(snapshot.totalRevenue)
        if (snapshot.revenueCurrency) setCurrency(snapshot.revenueCurrency)
      })
      .catch((error: unknown) => {
        if (!cancelled) {
          console.warn("[RVPay] transactions stats fetch failed:", error)
          setStatsError(describeApiError(error, "Transaction statistics are currently unavailable."))
        }
      })
    return () => {
      cancelled = true
    }
  }, [])

  React.useEffect(() => {
    let cancelled = false
    fetchSubAccounts({ page: 1, pageSize: 100 })
      .then((response) => {
        if (!cancelled) setSubAccounts(response.rows)
      })
      .catch((error: unknown) => {
        console.warn("[RVPay] sub-accounts fetch failed:", error)
      })
    return () => {
      cancelled = true
    }
  }, [])

  // The backend sub_account filter matches by client_name, but the view's
  // selector emits the sub-account id — resolve id -> name before fetching.
  const subAccountNameById = React.useMemo(
    () => new Map(subAccounts.map((row) => [row.id, row.name] as const)),
    [subAccounts],
  )

  // Fetch the transaction list whenever the page or the applied filters
  // change. Resetting to page 1 on a filter change avoids stale pages.
  React.useEffect(() => {
    let cancelled = false
    setLoading(true)
    setListError(null)
    fetchTransactions({
      status: applied.status !== "all" ? applied.status : undefined,
      subAccount:
        subAccount !== "all" ? subAccountNameById.get(subAccount) ?? subAccount : undefined,
      page,
      pageSize,
    })
      .then((response: TransactionsListResponse) => {
        if (cancelled) return
        setRows(response.rows)
        setTotal(response.total)
      })
      .catch((error: unknown) => {
        if (!cancelled) {
          console.warn("[RVPay] transactions list fetch failed:", error)
          setListError(describeApiError(error, "Transactions are currently unavailable."))
          setRows([])
          setTotal(0)
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [applied.status, subAccount, page])

  function applyFilters() {
    setApplied({ status, dateRange })
    setPage(1)
  }

  const activeFilters = [
    ...(applied.status !== "all"
      ? [{ label: `Status: ${applied.status}`, clear: () => { setStatus("all"); setApplied({ ...applied, status: "all" }) } }]
      : []),
    ...(applied.dateRange !== "Last 30 Days"
      ? [{ label: `Date Range: ${applied.dateRange}`, clear: () => { setDateRange("Last 30 Days"); setApplied({ ...applied, dateRange: "Last 30 Days" }) } }]
      : []),
  ]

  function clearAllFilters() {
    setStatus("all")
    setDateRange("Last 30 Days")
    setSubAccount("all")
    setApplied({ status: "all", dateRange: "Last 30 Days" })
  }

  return (
    <AuthGuard>
      <div className="flex min-h-screen">
        <AppSidebar />
        <div className="flex min-w-0 flex-1 flex-col">
          <Topbar />
          <main className="flex-1 bg-muted/30 p-6">
            {statsError && (
              <p className="mb-4 text-sm text-rose-600" role="alert">{statsError}</p>
            )}
            {listError && (
              <p className="mb-4 text-sm text-rose-600" role="alert">{listError}</p>
            )}
            <TransactionsView
              stats={[
                {
                  label: "Total Pending",
                  value:
                    totalPending === null
                      ? "—"
                      : formatUsd(totalPending).replace("USD", currency || "USD"),
                  trend: null,
                },
                {
                  label: "Total Cleared (30d)",
                  value:
                    totalCleared === null
                      ? "—"
                      : formatUsd(totalCleared).replace("USD", currency || "USD"),
                  trend: { direction: "up", label: "Stable volume" },
                },
              ]}
              subAccounts={subAccounts.map((row) => ({ id: row.id, name: row.name }))}
              subAccount={subAccount}
              onSubAccountChange={setSubAccount}
              status={status}
              onStatusChange={(next) =>
                setStatus(
                  (TRANSACTIONS_STATUSES as readonly string[]).includes(next)
                    ? (next as TransactionsStatus)
                    : "all"
                )
              }
              dateRange={dateRange}
              onDateRangeChange={setDateRange}
              onApplyFilters={applyFilters}
              activeFilters={activeFilters}
              onClearAllFilters={clearAllFilters}
              rows={rows}
              loading={loading}
              total={total}
              page={page}
              pageSize={pageSize}
              onPageChange={setPage}
            />
          </main>
        </div>
      </div>
    </AuthGuard>
  )
}