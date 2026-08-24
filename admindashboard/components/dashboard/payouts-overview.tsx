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

export function PayoutsOverview() {
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
        <Button size="lg">
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
        <button
          type="button"
          className="flex items-center gap-2 rounded-lg border bg-white px-3 py-2 text-sm font-medium hover:bg-muted"
        >
          <ListFilter className="size-4 text-muted-foreground" />
          All Statuses
          <ChevronDown className="size-4 text-muted-foreground" />
        </button>
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
            {payoutOverviewRows.map((row) => (
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
          </TableBody>
        </Table>

        <div className="flex items-center justify-end gap-1 border-t px-4 py-3">
          <button
            type="button"
            aria-label="Previous page"
            className="flex size-7 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground"
          >
            <ChevronLeft className="size-4" />
          </button>
          <button
            type="button"
            className="flex size-7 items-center justify-center rounded-md bg-blue-600 text-sm font-medium text-white"
          >
            1
          </button>
          <button
            type="button"
            className="flex size-7 items-center justify-center rounded-md text-sm font-medium text-muted-foreground hover:bg-muted hover:text-foreground"
          >
            2
          </button>
          <button
            type="button"
            className="flex size-7 items-center justify-center rounded-md text-sm font-medium text-muted-foreground hover:bg-muted hover:text-foreground"
          >
            3
          </button>
          <span className="px-1 text-sm text-muted-foreground">...</span>
          <button
            type="button"
            aria-label="Next page"
            className="flex size-7 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground"
          >
            <ChevronRight className="size-4" />
          </button>
        </div>
      </div>
    </div>
  )
}
