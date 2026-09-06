"use client"

import * as React from "react"

import { AppSidebar } from "@/components/dashboard/app-sidebar"
import { PayoutsOverview } from "@/components/dashboard/payouts-overview"
import { Topbar } from "@/components/dashboard/topbar"
import {
  describeApiError,
  fetchPayoutOverviewStats,
  fetchPayouts,
  type PayoutListResponse,
  type PayoutStatsResponse,
} from "@/lib/api"
import type {
  PayoutOverviewRow,
  PayoutOverviewStat,
  PayoutOverviewStatus,
} from "@/lib/dashboard-data"

// The only status filter values the payouts UI/backend support. Values coming
// from generic strings (e.g. query parameters) are validated against this list;
// anything unknown/malformed falls back to "All" instead of being cast blindly.
const PAYOUT_STATUSES = ["Failed", "In Transit", "Cleared", "Pending"] as const

function parsePayoutStatus(value: string): PayoutOverviewStatus | "All" {
  return (PAYOUT_STATUSES as readonly string[]).includes(value)
    ? (value as PayoutOverviewStatus)
    : "All"
}

// Map server stats to the dashboard stat-card shape. Icon/metaTone are
// derived by position (pending, cleared, failed); the failed card carries the
// action-needed alert only when the server reports failures.
function statsFromResponse(response: PayoutStatsResponse): PayoutOverviewStat[] {
  const icons = ["pending", "cleared", "failed"] as const
  return response.stats.map((stat, index) => ({
    label: stat.label,
    value: stat.value,
    meta: stat.meta,
    metaTone: index === 1 ? ("positive" as const) : ("muted" as const),
    icon: icons[index] ?? "pending",
    ...(index === 2 && stat.value !== "0" ? { alert: "Action Needed" } : {}),
  }))
}

function rowsFromResponse(response: PayoutListResponse): PayoutOverviewRow[] {
  return response.rows.map((row) => ({
    id: row.id,
    initials: row.initials,
    avatarClass: "bg-muted text-muted-foreground",
    name: row.name,
    location: "",
    amount: row.amount,
    status: row.status as PayoutOverviewRow["status"],
    initiated: row.initiated,
    expectedOrCleared: row.expectedOrCleared,
    expectedOrClearedStrong: row.expectedOrClearedStrong,
  }))
}

export default function PayoutsPage() {
  const [query, setQuery] = React.useState("")
  const [status, setStatus] = React.useState<PayoutOverviewStatus | "All">(
    parsePayoutStatus("All")
  )
  const [page, setPage] = React.useState(1)
  const pageSize = 20

  const [stats, setStats] = React.useState<PayoutOverviewStat[]>([])
  const [rows, setRows] = React.useState<PayoutOverviewRow[]>([])
  const [total, setTotal] = React.useState(0)
  const [loading, setLoading] = React.useState(true)
  const [loadError, setLoadError] = React.useState<string | null>(null)

  React.useEffect(() => {
    let cancelled = false
    fetchPayoutOverviewStats()
      .then((response) => {
        if (!cancelled) setStats(statsFromResponse(response))
      })
      .catch((error: unknown) => {
        console.warn("[RVPay] payout stats fetch failed:", error)
      })
    return () => {
      cancelled = true
    }
  }, [])

  React.useEffect(() => {
    let cancelled = false
    setLoading(true)
    setLoadError(null)

    fetchPayouts({
      search: query || undefined,
      status: status === "All" ? undefined : status,
      page,
      pageSize,
    })
      .then((response) => {
        if (!cancelled) {
          setRows(rowsFromResponse(response))
          setTotal(response.total)
          setLoading(false)
        }
      })
      .catch((error: unknown) => {
        if (!cancelled) {
          console.warn("[RVPay] payouts fetch failed:", error)
          setLoadError(describeApiError(error, "Payout data is currently unavailable."))
          setLoading(false)
        }
      })

    return () => {
      cancelled = true
    }
  }, [query, status, page])

  return (
    <div className="flex min-h-screen">
      <AppSidebar />

      <div className="flex min-w-0 flex-1 flex-col">
        <Topbar />

        <main className="flex-1 bg-muted/30 p-6">
          {loadError && (
            <p className="mb-4 text-sm text-rose-600" role="alert">{loadError}</p>
          )}
          <PayoutsOverview
            stats={stats}
            rows={rows}
            total={total}
            page={page}
            pageSize={pageSize}
            loading={loading}
            query={query}
            status={status}
            onQueryChange={(next) => {
              setQuery(next)
              setPage(1)
            }}
            onStatusChange={(next) => {
              setStatus(parsePayoutStatus(next))
              setPage(1)
            }}
            onPageChange={setPage}
          />
        </main>
      </div>
    </div>
  )
}
