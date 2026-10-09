"use client"

import * as React from "react"
import { useSearchParams } from "next/navigation"

import { TransactionsView } from "@/components/dashboard/transactions-view"
import {
  describeApiError,
  fetchTransactions,
  type TransactionsListResponse,
} from "@/lib/api"

const TRANSACTIONS_STATUSES = ["Success", "Failed", "Refunded", "Pending"] as const
type TransactionsStatus = (typeof TRANSACTIONS_STATUSES)[number]

function EmbedTransactionsContent() {
  const searchParams = useSearchParams()
  const locationId = searchParams.get("locationId")?.trim() ?? ""
  const [status, setStatus] = React.useState<"all" | TransactionsStatus>("all")
  const [appliedStatus, setAppliedStatus] = React.useState("all")
  const [rows, setRows] = React.useState<TransactionsListResponse["rows"]>([])
  const [total, setTotal] = React.useState(0)
  const [page, setPage] = React.useState(1)
  const [completedRequestKey, setCompletedRequestKey] = React.useState("")
  const [listError, setListError] = React.useState<{ key: string; message: string } | null>(null)
  const pageSize = 20
  const requestKey = `${locationId}\n${appliedStatus}\n${page}`

  React.useEffect(() => {
    if (!locationId) return

    let cancelled = false
    fetchTransactions({
      status: appliedStatus !== "all" ? appliedStatus : undefined,
      locationId,
      page,
      pageSize,
    }, { authenticated: false })
      .then((response: TransactionsListResponse) => {
        if (cancelled) return
        setRows(response.rows)
        setTotal(response.total)
        setCompletedRequestKey(requestKey)
      })
      .catch((error: unknown) => {
        if (!cancelled) {
          console.warn("[RVPay] embedded transactions fetch failed:", error)
          setListError({
            key: requestKey,
            message: describeApiError(error, "Transactions are currently unavailable."),
          })
          setRows([])
          setTotal(0)
          setCompletedRequestKey(requestKey)
        }
      })

    return () => {
      cancelled = true
    }
  }, [appliedStatus, locationId, page, requestKey])

  function applyFilters() {
    setAppliedStatus(status)
    setPage(1)
  }

  const activeFilters =
    appliedStatus !== "all"
      ? [{
          label: `Status: ${appliedStatus}`,
          clear: () => {
            setStatus("all")
            setAppliedStatus("all")
            setPage(1)
          },
        }]
      : []
  const isLoading = Boolean(locationId) && completedRequestKey !== requestKey
  const visibleError = locationId
    ? listError?.key === requestKey ? listError.message : null
    : "This transactions view is not configured."

  return (
    <main className="min-h-screen bg-muted/30 p-4 sm:p-6">
      {visibleError && (
        <p className="mb-4 text-sm text-rose-600" role="alert">
          {visibleError}
        </p>
      )}
      <TransactionsView
        description="Recent payment activity."
        showSubAccountFilter={false}
        showSubAccountColumn={false}
        showDateRangeFilter={false}
        status={status}
        onStatusChange={(next) =>
          setStatus(
            (TRANSACTIONS_STATUSES as readonly string[]).includes(next)
              ? (next as TransactionsStatus)
              : "all",
          )
        }
        onApplyFilters={applyFilters}
        activeFilters={activeFilters}
        onClearAllFilters={() => {
          setStatus("all")
          setAppliedStatus("all")
          setPage(1)
        }}
        rows={locationId && !isLoading ? rows : []}
        loading={isLoading}
        total={locationId && !isLoading ? total : 0}
        page={page}
        pageSize={pageSize}
        onPageChange={setPage}
      />
    </main>
  )
}

export default function EmbedTransactionsPage() {
  return (
    <React.Suspense
      fallback={
        <main className="min-h-screen bg-muted/30 p-4 text-sm text-muted-foreground sm:p-6">
          Loading transactions…
        </main>
      }
    >
      <EmbedTransactionsContent />
    </React.Suspense>
  )
}