"use client"

import {
  ArrowUpRight,
  ChevronRight,
  Download,
  Wallet,
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

// Shared types with the page. Stats/rows are always real backend data — the
// view never fabricates values (agent directive: no mock production data).
export type TransactionsStat = {
  label: string
  value: string
  trend: { direction: "up" | "down"; label: string } | null
}

export type ActiveFilter = {
  label: string
  clear: () => void
}

const statusStyles: Record<string, string> = {
  Success: "bg-emerald-100 text-emerald-700",
  Failed: "bg-rose-100 text-rose-700",
  Refunded: "bg-amber-100 text-amber-700",
  Pending: "bg-muted text-muted-foreground",
}

function StatCard({ stat }: { stat: TransactionsStat }) {
  return (
    <Card className="p-5">
      <span className="flex items-center gap-1.5 text-sm text-muted-foreground">
        {stat.label.startsWith("Total Pending") ? (
          <Wallet className="size-4" />
        ) : (
          <ArrowUpRight className="size-4" />
        )}
        {stat.label}
      </span>
      <p className="mt-2 text-2xl font-semibold tracking-tight">{stat.value}</p>
      {stat.trend && (
        <p
          className={cn(
            "mt-2 flex items-center gap-1 text-xs",
            stat.trend.direction === "up"
              ? "font-medium text-emerald-600"
              : "text-muted-foreground"
          )}
        >
          {stat.trend.direction === "up" && <ArrowUpRight className="size-3.5" />}
          {stat.trend.label}
        </p>
      )}
    </Card>
  )
}

// Transactions view following the supplied design: two stat cards, a filter
// bar, active-filter chips, and a paginated transaction table. Filters are
// wired to real sub-account options; transaction rows come from the
// Transactions service once the list endpoint exists (see
// agents/new-pages-endpoints). Until then the table renders a truthful empty
// state — no mock rows.
export function TransactionsView({
  stats = [],
  subAccounts = [],
  subAccount = "all",
  onSubAccountChange,
  status = "all",
  onStatusChange,
  dateRange = "Last 30 Days",
  onDateRangeChange,
  onApplyFilters,
  activeFilters = [],
  onClearAllFilters,
  rows = [],
  loading = false,
  total = 0,
  page = 1,
  pageSize = 20,
  onPageChange,
}: {
  stats?: TransactionsStat[]
  subAccounts?: Array<{ id: string; name: string }>
  subAccount?: string
  onSubAccountChange?: (next: string) => void
  status?: string
  onStatusChange?: (next: string) => void
  dateRange?: string
  onDateRangeChange?: (next: string) => void
  onApplyFilters?: () => void
  activeFilters?: ActiveFilter[]
  onClearAllFilters?: () => void
  rows?: Array<{
    id: string
    shortId: string
    subAccount: string
    customer: string
    customerInitials: string
    amount: string
    status: string
    gateway: string
    date: string
  }>
  loading?: boolean
  total?: number
  page?: number
  pageSize?: number
  onPageChange?: (next: number) => void
}) {
  const pageCount = Math.max(1, Math.ceil(total / pageSize))
  const hasRows = rows.length > 0

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Transactions</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Manage payments across all registered sub-accounts.
          </p>
        </div>
        <Button type="button" variant="outline">
          <Download className="size-4" />
          Export
        </Button>
      </div>
      <div className="grid gap-4 lg:grid-cols-2">
        {stats.map((stat) => (
          <StatCard key={stat.label} stat={stat} />
        ))}
      </div>

      <Card className="p-4">
        <div className="grid gap-4 lg:grid-cols-4">
          <div>
            <label className="text-xs font-semibold" htmlFor="tx-sub-account">
              Sub-Account
            </label>
            <select
              id="tx-sub-account"
              value={subAccount}
              onChange={(event) => onSubAccountChange?.(event.target.value)}
              className="mt-1.5 h-9 w-full rounded-md border bg-white px-3 text-sm outline-none focus:ring-2 focus:ring-ring"
            >
              <option value="all">All Sub-Accounts</option>
              {subAccounts.map((account) => (
                <option key={account.id} value={account.id}>
                  {account.name}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label className="text-xs font-semibold" htmlFor="tx-status">
              Status
            </label>
            <select
              id="tx-status"
              value={status}
              onChange={(event) => onStatusChange?.(event.target.value)}
              className="mt-1.5 h-9 w-full rounded-md border bg-white px-3 text-sm outline-none focus:ring-2 focus:ring-ring"
            >
              <option value="all">All Statuses</option>
              <option value="Success">Success</option>
              <option value="Failed">Failed</option>
              <option value="Refunded">Refunded</option>
              <option value="Pending">Pending</option>
            </select>
          </div>
          <div>
            <label className="text-xs font-semibold" htmlFor="tx-date-range">
              Date Range
            </label>
            <select
              id="tx-date-range"
              value={dateRange}
              onChange={(event) => onDateRangeChange?.(event.target.value)}
              className="mt-1.5 h-9 w-full rounded-md border bg-white px-3 text-sm outline-none focus:ring-2 focus:ring-ring"
            >
              <option>Last 30 Days</option>
              <option>Last 7 Days</option>
              <option>Last 90 Days</option>
              <option>This Year</option>
            </select>
          </div>
          <div className="flex items-end">
            <Button type="button" className="w-full" onClick={onApplyFilters}>
              Apply Filters
            </Button>
          </div>
        </div>
      </Card>

      {activeFilters.length > 0 && (
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
            Active filters:
          </span>
          {activeFilters.map((filter) => (
            <button
              key={filter.label}
              type="button"
              onClick={filter.clear}
              className="flex items-center gap-1 rounded-full bg-blue-50 px-3 py-1 text-xs font-medium text-blue-700 hover:bg-blue-100"
            >
              {filter.label}
              <span aria-hidden="true">×</span>
            </button>
          ))}
          <button
            type="button"
            onClick={onClearAllFilters}
            className="text-xs font-medium text-muted-foreground hover:text-foreground"
          >
            Clear all
          </button>
        </div>
      )}
      <Card className="overflow-hidden">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Transaction ID</TableHead>
              <TableHead>Sub-Account</TableHead>
              <TableHead>Customer</TableHead>
              <TableHead className="text-right">Amount</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Gateway</TableHead>
              <TableHead>Date</TableHead>
              <TableHead aria-label="Details" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {hasRows &&
              rows.map((row) => (
                <TableRow key={row.id}>
                  <TableCell className="font-mono text-xs">{row.shortId}</TableCell>
                  <TableCell className="font-medium">{row.subAccount}</TableCell>
                  <TableCell>
                    <span className="flex items-center gap-2">
                      <span className="flex size-6 items-center justify-center rounded-full bg-muted text-[10px] font-semibold text-muted-foreground">
                        {row.customerInitials}
                      </span>
                      {row.customer}
                    </span>
                  </TableCell>
                  <TableCell className="text-right">{row.amount}</TableCell>
                  <TableCell>
                    <Badge className={cn("border-0", statusStyles[row.status])}>
                      {row.status}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-muted-foreground">{row.gateway}</TableCell>
                  <TableCell className="text-muted-foreground">{row.date}</TableCell>
                  <TableCell>
                    <ChevronRight className="size-4 text-muted-foreground" />
                  </TableCell>
                </TableRow>
              ))}
            {loading && (
              <TableRow>
                <TableCell colSpan={8} className="py-10 text-center text-muted-foreground" role="status">
                  Loading transactions…
                </TableCell>
              </TableRow>
            )}
            {!loading && !hasRows && (
              <TableRow>
                <TableCell colSpan={8} className="py-10 text-center text-muted-foreground">
                  No transactions to display yet. The transactions list endpoint is
                  not yet available in the Transactions service (tracked in
                  agents/new-pages-endpoints), so live rows cannot be loaded.
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>

        <div className="flex items-center justify-end gap-1 border-t px-4 py-3">
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={page === 1}
            onClick={() => onPageChange?.(Math.max(1, page - 1))}
          >
            Previous
          </Button>
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={page >= pageCount}
            onClick={() => onPageChange?.(Math.min(pageCount, page + 1))}
          >
            Next
          </Button>
        </div>
      </Card>
    </div>
  )
}