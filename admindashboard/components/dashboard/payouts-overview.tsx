"use client"

import { useMemo, useState } from "react"
import {
  AlertTriangle,
  ArrowUpRight,
  CalendarDays,
  CheckCircle2,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  Clock,
  Download,
  ListFilter,
  Search,
  SlidersHorizontal,
} from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { cn } from "@/lib/utils"
import {
  payoutOverviewRows,
  payoutOverviewStats,
  type PayoutOverviewRow,
  type PayoutOverviewStat,
  type PayoutOverviewStatus,
} from "@/lib/dashboard-data"

const statIcons = {
  pending: Clock,
  cleared: CheckCircle2,
  failed: AlertTriangle,
} as const

const statusStyles: Record<PayoutOverviewStatus, string> = {
  Failed: "bg-rose-100 text-rose-700",
  "In Transit": "bg-amber-100 text-amber-700",
  Cleared: "bg-emerald-100 text-emerald-700",
  Pending: "bg-muted text-muted-foreground",
}

function StatCard({ stat }: { stat: PayoutOverviewStat }) {
  const Icon = statIcons[stat.icon]
  const isAlert = Boolean(stat.alert)

  return (
    <Card
      className={cn(
        "p-5",
        isAlert && "bg-rose-50/60 ring-rose-200"
      )}
    >
      <div className="flex items-start justify-between">
        <span className="flex items-center gap-1.5 text-sm text-muted-foreground">
          <Icon className="size-4" />
          {stat.label}
        </span>
        {stat.alert && (
          <Badge className="border-0 bg-rose-100 text-rose-700">
            {stat.alert}
          </Badge>
        )}
      </div>
      <p className="mt-2 text-2xl font-semibold tracking-tight">{stat.value}</p>
      <p
        className={cn(
          "mt-2 flex items-center gap-1 text-xs",
          stat.metaTone === "positive"
            ? "font-medium text-emerald-600"
            : "text-muted-foreground"
        )}
      >
        {stat.metaTone === "positive" && <ArrowUpRight className="size-3.5" />}
        {stat.meta}
      </p>
    </Card>
  )
}

export function PayoutsOverview({ rows = payoutOverviewRows }: { rows?: PayoutOverviewRow[] }) {
  const [query, setQuery] = useState("")
  const [status, setStatus] = useState<PayoutOverviewStatus | "All">("All")
  const [page, setPage] = useState(1)
  const pageSize = 3
  const filteredRows = useMemo(() => rows.filter((row) => {
    const matchesQuery = `${row.name} ${row.location}`.toLowerCase().includes(query.toLowerCase())
    return matchesQuery && (status === "All" || row.status === status)
  }), [query, rows, status])
  const pageCount = Math.max(1, Math.ceil(filteredRows.length / pageSize))
  const visibleRows = filteredRows.slice((page - 1) * pageSize, page * pageSize)
  const pageNumbers = Array.from({ length: pageCount }, (_, index) => index + 1)

  function exportCsv() {
    const csv = ["Sub-account,Amount,Status,Initiated", ...filteredRows.map((row) => `${row.name},${row.amount},${row.status},${row.initiated}`)].join("\n")
    const url = URL.createObjectURL(new Blob([csv], { type: "text/csv" }))
    const link = document.createElement("a")
    link.href = url
    link.download = "rvpay-payouts.csv"
    link.click()
    URL.revokeObjectURL(url)
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">
            Payouts Overview
          </h1>
          <p className="text-sm text-muted-foreground">
            Monitor funds transferring from master account to sub-accounts.
          </p>
        </div>
        <Button size="lg" onClick={exportCsv}>
          <Download data-icon="inline-start" />
          Export CSV
        </Button>
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        {payoutOverviewStats.map((stat) => (
          <StatCard key={stat.label} stat={stat} />
        ))}
      </div>

      <div className="flex items-center gap-3">
        <div className="relative w-56">
          <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
          <input
            type="search"
            value={query}
            onChange={(event) => { setQuery(event.target.value); setPage(1) }}
            placeholder="Sub-account..."
            className="h-9 w-full rounded-lg border bg-white pl-9 pr-4 text-sm outline-none placeholder:text-muted-foreground focus:ring-2 focus:ring-ring"
          />
        </div>
        <button
          type="button"
          className="flex items-center gap-2 rounded-lg border bg-white px-3 py-2 text-sm font-medium hover:bg-muted"
        >
          <CalendarDays className="size-4 text-muted-foreground" />
          Last 7 Days
          <ChevronDown className="size-4 text-muted-foreground" />
        </button>
        <button
          type="button"
          className="flex items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium text-muted-foreground hover:bg-muted hover:text-foreground"
        >
          <SlidersHorizontal className="size-4" />
          More Filters
        </button>
        <div className="flex-1" />
        <label className="flex items-center gap-2 rounded-lg border bg-white px-3 py-2 text-sm font-medium">
          <ListFilter className="size-4 text-muted-foreground" />
          <select value={status} onChange={(event) => { setStatus(event.target.value as PayoutOverviewStatus | "All"); setPage(1) }} className="bg-transparent outline-none">
            <option value="All">All Statuses</option><option>Failed</option><option>In Transit</option><option>Cleared</option><option>Pending</option>
          </select>
          <ChevronDown className="size-4 text-muted-foreground" />
        </label>
      </div>

      <div className="rounded-xl border bg-white">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Sub-Account</TableHead>
              <TableHead>Amount</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Initiated</TableHead>
              <TableHead>Expected/Cleared</TableHead>
              <TableHead className="w-8" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {visibleRows.map((row) => (
              <TableRow key={row.id}>
                <TableCell>
                  <div className="flex items-center gap-3">
                    <div
                      className={cn(
                        "flex size-8 shrink-0 items-center justify-center rounded-full text-xs font-semibold",
                        row.avatarClass
                      )}
                    >
                      {row.initials}
                    </div>
                    <div className="min-w-0">
                      <p className="truncate font-medium">{row.name}</p>
                      <p className="truncate text-xs text-muted-foreground">
                        {row.location}
                      </p>
                    </div>
                  </div>
                </TableCell>
                <TableCell>{row.amount}</TableCell>
                <TableCell>
                  <Badge className={cn("border-0", statusStyles[row.status])}>
                    {row.status}
                  </Badge>
                </TableCell>
                <TableCell className="text-muted-foreground">
                  {row.initiated}
                </TableCell>
                <TableCell
                  className={
                    row.expectedOrClearedStrong
                      ? "font-medium"
                      : "text-muted-foreground"
                  }
                >
                  {row.expectedOrCleared}
                </TableCell>
                <TableCell>
                  <button
                    type="button"
                    aria-label={`Expand ${row.name} payout`}
                    className="flex size-7 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground"
                  >
                    <ChevronDown className="size-4" />
                  </button>
                </TableCell>
              </TableRow>
            ))}
            {visibleRows.length === 0 && <TableRow><TableCell colSpan={6} className="py-10 text-center text-muted-foreground">No payouts match your filters.</TableCell></TableRow>}
          </TableBody>
        </Table>

        <div className="flex items-center justify-end gap-1 border-t px-4 py-3">
          <button
            type="button"
            aria-label="Previous page"
            disabled={page === 1}
            onClick={() => setPage((current) => Math.max(1, current - 1))}
            className="flex size-7 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground"
          >
            <ChevronLeft className="size-4" />
          </button>
          {pageNumbers.map((pageNumber) => (
            <button
              key={pageNumber}
              type="button"
              onClick={() => setPage(pageNumber)}
              className={cn(
                "flex size-7 items-center justify-center rounded-md text-sm font-medium",
                pageNumber === page ? "bg-blue-600 text-white" : "text-muted-foreground hover:bg-muted hover:text-foreground"
              )}
            >
              {pageNumber}
            </button>
          ))}
          <button
            type="button"
            aria-label="Next page"
            disabled={page >= pageCount}
            onClick={() => setPage((current) => Math.min(pageCount, current + 1))}
            className="flex size-7 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground"
          >
            <ChevronRight className="size-4" />
          </button>
        </div>
      </div>
    </div>
  )
}
