"use client"

import * as React from "react"

import { AppSidebar } from "@/components/dashboard/app-sidebar"
import { SubAccountsTable } from "@/components/dashboard/sub-accounts-table"
import { Topbar } from "@/components/dashboard/topbar"
import { describeApiError, fetchSubAccounts, type SubAccountListResponse } from "@/lib/api"
import { type SubAccount, type SubAccountStatus } from "@/lib/dashboard-data"

// The only status filter values the sub-accounts UI/backend support. Values
// coming from generic strings (e.g. query parameters) are validated against
// this list; anything unknown/malformed falls back to "All".
const SUB_ACCOUNT_STATUSES = ["Active", "Restricted", "Inactive"] as const

export function parseSubAccountStatus(value: string): SubAccountStatus | "All" {
  return (SUB_ACCOUNT_STATUSES as readonly string[]).includes(value)
    ? (value as SubAccountStatus)
    : "All"
}

// Map the proto sub-account status enum to the dashboard-facing label.
function statusLabel(status: string): SubAccount["status"] {
  switch (status) {
    case "SUB_ACCOUNT_STATUS_ACTIVE":
      return "Active"
    case "SUB_ACCOUNT_STATUS_RESTRICTED":
      return "Restricted"
    case "SUB_ACCOUNT_STATUS_INACTIVE":
      return "Inactive"
    default:
      return "Inactive"
  }
}

// Cross-service gaps (balance, last payout date, total processed) arrive as
// empty strings; surface them as "—" rather than fabricating values.
function orDash(value: string): string {
  return value === "" ? "—" : value
}

function rowsFromResponse(response: SubAccountListResponse): SubAccount[] {
  return response.rows.map((row) => ({
    id: row.id,
    initials: row.initials,
    avatarClass: "bg-muted text-muted-foreground",
    name: row.name,
    location: row.location,
    status: statusLabel(row.status),
    balance: orDash(row.balance),
    lastPayoutDate: orDash(row.lastPayoutDate),
    totalProcessed: orDash(row.totalProcessed),
  }))
}

export default function SubAccountsPage() {
  const [query, setQuery] = React.useState("")
  const [status, setStatus] = React.useState<SubAccountStatus | "All">(
    parseSubAccountStatus("All")
  )
  const [page, setPage] = React.useState(1)
  const pageSize = 20

  const [accounts, setAccounts] = React.useState<SubAccount[]>([])
  const [loading, setLoading] = React.useState(true)
  const [loadError, setLoadError] = React.useState<string | null>(null)

  React.useEffect(() => {
    let cancelled = false
    setLoading(true)
    setLoadError(null)

    fetchSubAccounts({
      search: query || undefined,
      status:
        status === "All"
          ? undefined
          : `SUB_ACCOUNT_STATUS_${status.toUpperCase()}`,
      page,
      pageSize,
    })
      .then((response) => {
        if (!cancelled) {
          setAccounts(rowsFromResponse(response))
          setLoading(false)
        }
      })
      .catch((error: unknown) => {
        if (!cancelled) {
          console.warn("[RVPay] sub-accounts fetch failed:", error)
          setLoadError(describeApiError(error, "Sub-account data is currently unavailable."))
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
          <SubAccountsTable
            accounts={accounts}
            loading={loading}
            query={query}
            status={status}
            onQueryChange={(next) => {
              setQuery(next)
              setPage(1)
            }}
            onStatusChange={(next) => {
              setStatus(parseSubAccountStatus(next))
              setPage(1)
            }}
          />
        </main>
      </div>
    </div>
  )
}
