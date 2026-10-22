"use client"

import * as React from "react"
import { ArrowLeft, ArrowRight, Check, Wallet, X } from "lucide-react"
import { useRouter, useSearchParams } from "next/navigation"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Card } from "@/components/ui/card"
import { TransactionsView } from "@/components/dashboard/transactions-view"
import {
  describeApiError,
  fetchOverviewSnapshot,
  fetchTransactions,
  type OverviewSnapshotResponse,
  type TransactionListRow,
  type TransactionsListResponse,
} from "@/lib/api"

const TRANSACTIONS_STATUSES = ["Success", "Failed", "Refunded", "Pending"] as const
type TransactionsStatus = (typeof TRANSACTIONS_STATUSES)[number]

const SAMPLE_TRANSACTIONS: TransactionListRow[] = [
  {
    id: "sample-tx-001",
    shortId: "TX-80421",
    subAccount: "Sample account",
    customer: "Maya Chen",
    customerName: "Maya Chen",
    customerInitials: "MC",
    amount: "$248.00",
    status: "Success",
    gateway: "MTN Mobile Money",
    date: "Oct 22, 2026",
  },
  {
    id: "sample-tx-002",
    shortId: "TX-80420",
    subAccount: "Sample account",
    customer: "Daniel Okafor",
    customerName: "Daniel Okafor",
    customerInitials: "DO",
    amount: "$1,200.00",
    status: "Pending",
    gateway: "Orange Money",
    date: "Oct 22, 2026",
  },
  {
    id: "sample-tx-003",
    shortId: "TX-80419",
    subAccount: "Sample account",
    customer: "Ava Williams",
    customerName: "Ava Williams",
    customerInitials: "AW",
    amount: "$86.50",
    status: "Success",
    gateway: "Airtel Money",
    date: "Oct 21, 2026",
  },
  {
    id: "sample-tx-004",
    shortId: "TX-80418",
    subAccount: "Sample account",
    customer: "Noah Patel",
    customerName: "Noah Patel",
    customerInitials: "NP",
    amount: "$64.00",
    status: "Failed",
    gateway: "MTN Mobile Money",
    date: "Oct 20, 2026",
  },
  {
    id: "sample-tx-005",
    shortId: "TX-80417",
    subAccount: "Sample account",
    customer: "Sofia Mensah",
    customerName: "Sofia Mensah",
    customerInitials: "SM",
    amount: "$312.75",
    status: "Refunded",
    gateway: "Orange Money",
    date: "Oct 19, 2026",
  },
]

function EmbedTransactionsContent() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const locationId = searchParams.get("locationId")?.trim() ?? ""
  const [status, setStatus] = React.useState<"all" | TransactionsStatus>("all")
  const [appliedStatus, setAppliedStatus] = React.useState("all")
  const [rows, setRows] = React.useState<TransactionsListResponse["rows"]>([])
  const [total, setTotal] = React.useState(0)
  const [page, setPage] = React.useState(1)
  const [completedRequestKey, setCompletedRequestKey] = React.useState("")
  const [listError, setListError] = React.useState<{ key: string; message: string } | null>(null)
  const [withdrawalOpen, setWithdrawalOpen] = React.useState(false)
  const [withdrawalStep, setWithdrawalStep] = React.useState<"details" | "review" | "preview">("details")
  const [withdrawalAmount, setWithdrawalAmount] = React.useState("")
  const [payoutMethod, setPayoutMethod] = React.useState("Bank transfer")
  const [overview, setOverview] = React.useState<OverviewSnapshotResponse | null>(null)
  const [overviewError, setOverviewError] = React.useState(false)
  const pageSize = 20
  const requestKey = `${locationId}\n${appliedStatus}\n${page}`

  React.useEffect(() => {
    let cancelled = false
    fetchOverviewSnapshot("30d", { authenticated: false })
      .then((response) => {
        if (!cancelled) setOverview(response)
      })
      .catch((error: unknown) => {
        if (!cancelled) {
          console.warn("[RVPay] embedded transaction overview fetch failed:", error)
          setOverviewError(true)
        }
      })

    return () => {
      cancelled = true
    }
  }, [])

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
  const showingSampleTransactions = !isLoading && rows.length === 0
  const visibleRows = showingSampleTransactions
    ? SAMPLE_TRANSACTIONS.filter((row) => appliedStatus === "all" || row.status === appliedStatus)
    : rows
  const visibleError = locationId
    ? listError?.key === requestKey ? listError.message : null
    : "This transactions view is not configured."

  function openTransaction(row: TransactionListRow) {
    const details = new URLSearchParams({
      shortId: row.shortId,
      customer: row.customerName || row.customer,
      amount: row.amount,
      status: row.status,
      provider: row.gateway,
      date: row.date,
      sample: row.id.startsWith("sample-tx-") ? "true" : "false",
    })
    if (locationId) details.set("locationId", locationId)
    router.push(`/embed/transactions/${encodeURIComponent(row.id)}?${details.toString()}`)
  }

  function closeWithdrawal() {
    setWithdrawalOpen(false)
    setWithdrawalStep("details")
  }

  React.useEffect(() => {
    if (!withdrawalOpen) return

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") closeWithdrawal()
    }

    window.addEventListener("keydown", handleKeyDown)
    return () => window.removeEventListener("keydown", handleKeyDown)
  }, [withdrawalOpen])

  return (
    <main className="min-h-screen bg-muted/30 p-4 sm:p-6">
      {visibleError && (
        <p className="mb-4 text-sm text-rose-600" role="alert">
          {visibleError}
        </p>
      )}
      <div className="mb-5 flex justify-end">
        <Button type="button" onClick={() => setWithdrawalOpen(true)}>
          <Wallet data-icon="inline-start" />
          Withdraw funds
        </Button>
      </div>
      <section aria-labelledby="embed-overview-title" className="mb-6 space-y-3">
        <div className="flex flex-wrap items-baseline justify-between gap-2">
          <h2 id="embed-overview-title" className="text-base font-semibold">Account overview</h2>
          <p className="text-xs text-muted-foreground">Last 30 days · all sub-accounts</p>
        </div>
        <div className="grid gap-3 sm:grid-cols-3">
          <Card className="p-4">
            <p className="text-sm text-muted-foreground">Total revenue</p>
            <p className="mt-2 text-xl font-semibold">
              {overview
                ? new Intl.NumberFormat("en-US", {
                    style: "currency",
                    currency: overview.revenueCurrency || "USD",
                  }).format(overview.totalRevenue)
                : overviewError ? "Unavailable" : "Loading…"}
            </p>
          </Card>
          <Card className="p-4">
            <p className="text-sm text-muted-foreground">Transactions</p>
            <p className="mt-2 text-xl font-semibold">
              {overview ? overview.transactionVolume.toLocaleString("en-US") : overviewError ? "Unavailable" : "Loading…"}
            </p>
          </Card>
          <Card className="p-4">
            <p className="text-sm text-muted-foreground">Pending payouts</p>
            <p className="mt-2 text-xl font-semibold">
              {overview
                ? new Intl.NumberFormat("en-US", {
                    style: "currency",
                    currency: overview.revenueCurrency || "USD",
                  }).format(overview.pendingPayouts)
                : overviewError ? "Unavailable" : "Loading…"}
            </p>
          </Card>
        </div>
      </section>
      {showingSampleTransactions && (
        <p className="mb-4 rounded-md border border-amber-300 bg-amber-50 px-3 py-2 text-sm text-amber-900" role="status">
          Sample transactions are shown for visualisation only. They are not real payments.
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
        rows={isLoading ? [] : visibleRows}
        loading={isLoading}
        total={showingSampleTransactions ? visibleRows.length : total}
        page={page}
        pageSize={pageSize}
        onPageChange={setPage}
        onTransactionClick={openTransaction}
      />
      {withdrawalOpen && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
          role="presentation"
          onMouseDown={(event) => {
            if (event.target === event.currentTarget) closeWithdrawal()
          }}
        >
          <section
            role="dialog"
            aria-modal="true"
            aria-labelledby="withdrawal-title"
            className="w-full max-w-md rounded-lg border bg-background p-5 shadow-xl sm:p-6"
          >
            <div className="flex items-start justify-between gap-4">
              <div>
                <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                  {withdrawalStep === "preview" ? "Preview" : withdrawalStep === "review" ? "Review" : "Payout"}
                </p>
                <h2 id="withdrawal-title" className="mt-1 text-lg font-semibold">
                  {withdrawalStep === "preview" ? "Request preview" : "Withdraw funds"}
                </h2>
              </div>
              <Button type="button" variant="ghost" size="icon" aria-label="Close withdrawal dialog" onClick={closeWithdrawal}>
                <X />
              </Button>
            </div>

            {withdrawalStep === "details" && (
              <form
                className="mt-5 space-y-4"
                onSubmit={(event) => {
                  event.preventDefault()
                  setWithdrawalStep("review")
                }}
              >
                <div className="rounded-md border bg-muted/30 px-3 py-2.5">
                  <p className="text-xs text-muted-foreground">Available balance</p>
                  <p className="mt-0.5 text-sm font-medium">Unavailable</p>
                </div>
                <p className="text-xs text-muted-foreground">
                  Balance details will appear when the payout service is connected.
                </p>
                <div className="space-y-1.5">
                  <label htmlFor="withdrawal-amount" className="text-sm font-medium">Amount</label>
                  <Input
                    id="withdrawal-amount"
                    type="number"
                    min="0.01"
                    step="0.01"
                    inputMode="decimal"
                    placeholder="0.00"
                    value={withdrawalAmount}
                    onChange={(event) => setWithdrawalAmount(event.target.value)}
                    required
                  />
                </div>
                <div className="space-y-1.5">
                  <label htmlFor="withdrawal-method" className="text-sm font-medium">Payout method</label>
                  <select
                    id="withdrawal-method"
                    value={payoutMethod}
                    onChange={(event) => setPayoutMethod(event.target.value)}
                    className="h-9 w-full rounded-md border bg-background px-3 text-sm outline-none focus:ring-2 focus:ring-ring"
                  >
                    <option>Bank transfer</option>
                    <option>Mobile money</option>
                  </select>
                </div>
                <div className="flex justify-end pt-1">
                  <Button type="submit">
                    Continue
                    <ArrowRight data-icon="inline-end" />
                  </Button>
                </div>
              </form>
            )}

            {withdrawalStep === "review" && (
              <div className="mt-5 space-y-4">
                <dl className="divide-y rounded-md border">
                  <div className="flex justify-between gap-4 px-3 py-3 text-sm">
                    <dt className="text-muted-foreground">Amount</dt>
                    <dd className="font-semibold">{withdrawalAmount}</dd>
                  </div>
                  <div className="flex justify-between gap-4 px-3 py-3 text-sm">
                    <dt className="text-muted-foreground">Payout method</dt>
                    <dd className="font-medium">{payoutMethod}</dd>
                  </div>
                </dl>
                <p className="rounded-md bg-amber-50 px-3 py-2.5 text-sm text-amber-800">
                  UI preview only. No withdrawal will be submitted.
                </p>
                <div className="flex justify-between gap-3">
                  <Button type="button" variant="outline" onClick={() => setWithdrawalStep("details")}>
                    <ArrowLeft data-icon="inline-start" />
                    Back
                  </Button>
                  <Button type="button" onClick={() => setWithdrawalStep("preview")}>
                    Preview request
                  </Button>
                </div>
              </div>
            )}

            {withdrawalStep === "preview" && (
              <div className="mt-5 space-y-4">
                <div className="flex items-start gap-3 rounded-md border border-emerald-200 bg-emerald-50 p-3 text-emerald-800">
                  <Check className="mt-0.5 size-4 shrink-0" />
                  <div>
                    <p className="text-sm font-semibold">Preview ready</p>
                    <p className="mt-1 text-sm">This is a UI-only preview. No payout request was sent.</p>
                  </div>
                </div>
                <Button type="button" className="w-full" onClick={closeWithdrawal}>Done</Button>
              </div>
            )}
          </section>
        </div>
      )}
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