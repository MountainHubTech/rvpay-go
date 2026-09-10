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
  type SubAccountListResponse,
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

  function applyFilters() {
    setApplied({ status, dateRange })
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
            />
          </main>
        </div>
      </div>
    </AuthGuard>
  )
}