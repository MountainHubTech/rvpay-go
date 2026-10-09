"use client"

import * as React from "react"

import { Upload, X } from "lucide-react"

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

const disputeStatusStyles: Record<string, string> = {
  "NEEDS RESPONSE": "bg-rose-100 text-rose-700",
  "Under Review": "bg-blue-100 text-blue-700",
  RESOLVED: "bg-emerald-100 text-emerald-700",
}

export type DisputeRow = {
  id: string
  initials: string
  subAccount: string
  location: string
  type: string
  amount: string
  dateOpened: string
  status: string
  dueIn?: string
}

// Disputes view following the supplied design: two stat cards (Needs Response
// with progress bar, Under Review), filter bar, and a dispute table with an
// expandable Submit Evidence panel. All data is expected from the Transactions
// service once the disputes capability exists (see agents/new-pages-endpoints).
// Until then truthful zero/empty states are rendered — no mock rows.
export function DisputesView({
  backendAvailable = false,
  needsResponse = 0,
  underReview = 0,
  subAccounts = [],
  rows = [],
  loading = false,
}: {
  backendAvailable?: boolean
  needsResponse?: number
  underReview?: number
  subAccounts?: Array<{ id: string; name: string }>
  rows?: DisputeRow[]
  loading?: boolean
}) {
  const [expandedId, setExpandedId] = React.useState<string | null>(null)
  const progressPercent = needsResponse > 0 ? Math.min(100, needsResponse * 20) : 0

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Disputes</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Manage active chargebacks and payout issues requiring attention.
          </p>
        </div>
        <Button type="button" variant="outline">
          Export
        </Button>
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <Card className="p-5">
          <p className="text-sm text-muted-foreground">Needs Response</p>
          <p className="mt-2 text-2xl font-semibold tracking-tight">
            {needsResponse}{" "}
            <span className="text-sm font-normal text-muted-foreground">Due today</span>
          </p>
          <div className="mt-4 h-1.5 w-full rounded-full bg-muted">
            <div
              className="h-1.5 rounded-full bg-rose-600"
              style={{ width: `${progressPercent}%` }}
            />
          </div>
        </Card>
        <Card className="p-5">
          <p className="text-sm text-muted-foreground">Under Review</p>
          <p className="mt-2 text-2xl font-semibold tracking-tight">{underReview}</p>
        </Card>
      </div>

      <Card className="p-4">
        <div className="grid gap-4 lg:grid-cols-2">
          <div>
            <label className="text-xs font-semibold" htmlFor="dispute-sub-account">
              Sub account
            </label>
            <select
              id="dispute-sub-account"
              className="mt-1.5 h-9 w-full rounded-md border bg-white px-3 text-sm outline-none focus:ring-2 focus:ring-ring"
            >
              <option>Sub account</option>
              {subAccounts.map((account) => (
                <option key={account.id} value={account.id}>
                  {account.name}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label className="text-xs font-semibold" htmlFor="dispute-status">
              Status
            </label>
            <select
              id="dispute-status"
              className="mt-1.5 h-9 w-full rounded-md border bg-white px-3 text-sm outline-none focus:ring-2 focus:ring-ring"
            >
              <option>Action Required</option>
              <option>Under Review</option>
              <option>Resolved</option>
            </select>
          </div>
        </div>
      </Card>
      <Card className="overflow-hidden">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Sub-Account</TableHead>
              <TableHead>Type</TableHead>
              <TableHead className="text-right">Amount</TableHead>
              <TableHead>Date Opened</TableHead>
              <TableHead>Status</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((row) => (
              <React.Fragment key={row.id}>
                <TableRow
                  className="cursor-pointer"
                  onClick={() => setExpandedId(expandedId === row.id ? null : row.id)}
                >
                  <TableCell>
                    <span className="flex items-center gap-2">
                      <span className="flex size-7 items-center justify-center rounded-md bg-muted text-[10px] font-semibold text-muted-foreground">
                        {row.initials}
                      </span>
                      <span>
                        <span className="block font-medium">{row.subAccount}</span>
                        <span className="block font-mono text-xs text-muted-foreground">
                          {row.location}
                        </span>
                      </span>
                    </span>
                  </TableCell>
                  <TableCell>{row.type}</TableCell>
                  <TableCell className="text-right">{row.amount}</TableCell>
                  <TableCell className="text-muted-foreground">{row.dateOpened}</TableCell>
                  <TableCell>
                    <Badge className={cn("border-0", disputeStatusStyles[row.status])}>
                      {row.status}
                    </Badge>
                    {row.dueIn && (
                      <span className="mt-1 block text-xs text-muted-foreground">{row.dueIn}</span>
                    )}
                  </TableCell>
                </TableRow>
                {expandedId === row.id && (
                  <TableRow className="bg-muted/40">
                    <TableCell colSpan={5}>
                      <div className="rounded-lg border bg-white p-4">
                        <div className="flex items-start justify-between">
                          <div>
                            <p className="text-sm font-semibold">Submit Evidence</p>
                            <p className="mt-0.5 text-xs text-muted-foreground">
                              Provide tracking or communication proof to challenge this dispute.
                            </p>
                          </div>
                          <button
                            type="button"
                            aria-label="Close evidence form"
                            onClick={() => setExpandedId(null)}
                            className="text-muted-foreground hover:text-foreground"
                          >
                            <X className="size-4" />
                          </button>
                        </div>
                        <textarea
                          placeholder="Add notes..."
                          className="mt-3 min-h-16 w-full rounded-md border p-3 text-sm outline-none focus:ring-2 focus:ring-ring"
                        />
                        <div className="mt-3 flex items-center justify-between">
                          <button
                            type="button"
                            className="flex items-center gap-1.5 text-sm font-medium hover:underline"
                          >
                            <Upload className="size-4" />
                            Upload Documents
                          </button>
                          <div className="flex items-center gap-2">
                            <Button type="button" variant="outline" size="sm">
                              Accept Loss
                            </Button>
                            <Button type="button" size="sm">Submit</Button>
                          </div>
                        </div>
                      </div>
                    </TableCell>
                  </TableRow>
                )}
              </React.Fragment>
            ))}
            {loading && (
              <TableRow>
                <TableCell colSpan={5} className="py-10 text-center text-muted-foreground" role="status">
                  Loading disputes…
                </TableCell>
              </TableRow>
            )}
            {!loading && rows.length === 0 && (
              <TableRow>
                <TableCell colSpan={5} className="py-10 text-center text-muted-foreground">
                  {backendAvailable
                    ? "No disputes match your filters."
                    : "No disputes to display yet. The disputes capability is not yet available in the Transactions service (tracked in agents/new-pages-endpoints), so live rows cannot be loaded."}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </Card>
    </div>
  )
}