"use client"

import * as React from "react"

import { AppSidebar } from "@/components/dashboard/app-sidebar"
import { AuthGuard } from "@/lib/auth"
import { Topbar } from "@/components/dashboard/topbar"
import { DisputesView } from "@/components/dashboard/disputes-view"
import {
  describeApiError,
  fetchDisputes,
  fetchDisputeStats,
  fetchSubAccounts,
  type DisputesListResponse,
  type SubAccountListResponse,
} from "@/lib/api"

// initialsFromField derives two-letter initials from a display field for the
// dashboard dispute-row avatar chip.
function initialsFromField(value: string): string {
  const cleaned = value.replace(/[-_\s.]+/g, " ").trim()
  const parts = cleaned.split(" ").filter(Boolean)
  if (parts.length === 0) return "--"
  if (parts.length === 1) return parts[0]!.slice(0, 2).toUpperCase()
  return (parts[0]![0]! + parts[1]![0]!).toUpperCase()
}

export default function DisputesPage() {
  // Disputes capability is served by the EXISTING Transactions service
  // (GET /v1/public/transactions/disputes*). Stats, rows and sub-account
  // filters come from the backend — no mock data.
  const [needsResponse, setNeedsResponse] = React.useState(0)
  const [underReview, setUnderReview] = React.useState(0)
  const [subAccounts, setSubAccounts] = React.useState<
    SubAccountListResponse["rows"]
  >([])
  const [rows, setRows] = React.useState<DisputesListResponse["rows"]>([])
  const [loading, setLoading] = React.useState(false)
  const [error, setError] = React.useState<string | null>(null)

  React.useEffect(() => {
    let cancelled = false
    fetchDisputeStats()
      .then((stats) => {
        if (!cancelled) {
          setNeedsResponse(stats.needsResponse)
          setUnderReview(stats.underReview)
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          console.warn("[RVPay] dispute stats fetch failed:", err)
          setError(describeApiError(err, "Dispute statistics are currently unavailable."))
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
      .catch((err: unknown) => {
        console.warn("[RVPay] sub-accounts fetch failed:", err)
      })
    return () => {
      cancelled = true
    }
  }, [])

  React.useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError(null)
    fetchDisputes({ page: 1, pageSize: 100 })
      .then((response: DisputesListResponse) => {
        if (!cancelled) setRows(response.rows)
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          console.warn("[RVPay] disputes fetch failed:", err)
          setError(describeApiError(err, "Disputes are currently unavailable."))
          setRows([])
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [])

  return (
    <AuthGuard>
      <div className="flex min-h-screen">
        <AppSidebar />
        <div className="flex min-w-0 flex-1 flex-col">
          <Topbar />
          <main className="flex-1 bg-muted/30 p-6">
            {error && (
              <p className="mb-4 text-sm text-rose-600" role="alert">{error}</p>
            )}
            <DisputesView
              backendAvailable={true}
              needsResponse={needsResponse}
              underReview={underReview}
              subAccounts={subAccounts.map((row) => ({ id: row.id, name: row.name }))}
              rows={rows.map((row) => ({
                id: row.id,
                initials: initialsFromField(row.subAccount),
                subAccount: row.subAccount,
                location: row.subAccount,
                type: row.type,
                amount: row.amount,
                dateOpened: row.dateOpened,
                status: row.status,
                dueIn: row.dueIn,
              }))}
              loading={loading}
            />
          </main>
        </div>
      </div>
    </AuthGuard>
  )
}