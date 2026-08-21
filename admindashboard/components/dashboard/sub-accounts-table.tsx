import { Calendar, ChevronDown, Plus, Search } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { cn } from "@/lib/utils"
import { subAccounts, type SubAccountStatus } from "@/lib/dashboard-data"

const statusStyles: Record<SubAccountStatus, string> = {
  Active: "bg-emerald-100 text-emerald-700",
  Restricted: "bg-amber-100 text-amber-700",
  Inactive: "bg-muted text-muted-foreground",
}

export function SubAccountsTable() {
  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">
            Sub-Accounts
          </h1>
          <p className="text-sm text-muted-foreground">
            Manage and monitor connected payment accounts.
          </p>
        </div>
        <Button size="lg">
          <Plus data-icon="inline-start" />
          Add Account
        </Button>
      </div>

      <div className="flex items-center gap-3">
        <div className="relative flex-1">
          <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
          <input
            type="search"
            placeholder="Search by name, ID, or email..."
            className="h-9 w-full rounded-lg border bg-white pl-9 pr-4 text-sm outline-none placeholder:text-muted-foreground focus:ring-2 focus:ring-ring"
          />
        </div>
        <button
          type="button"
          className="flex items-center gap-2 rounded-lg border bg-white px-3 py-2 text-sm font-medium hover:bg-muted"
        >
          Status: All
          <ChevronDown className="size-4 text-muted-foreground" />
        </button>
        <button
          type="button"
          className="flex items-center gap-2 rounded-lg border bg-white px-3 py-2 text-sm font-medium hover:bg-muted"
        >
          <Calendar className="size-4 text-muted-foreground" />
          Date Joined
        </button>
      </div>

      <div className="rounded-xl border bg-white">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Sub-Account Name</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Current Balance</TableHead>
              <TableHead>Last Payout Date</TableHead>
              <TableHead>Total Processed (Lifetime)</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {subAccounts.map((account) => {
              const inactive = account.status === "Inactive"
              return (
                <TableRow key={account.id}>
                  <TableCell>
                    <div className="flex items-center gap-3">
                      <div
                        className={cn(
                          "flex size-8 shrink-0 items-center justify-center rounded-full text-xs font-semibold",
                          account.avatarClass
                        )}
                      >
                        {account.initials}
                      </div>
                      <div className="min-w-0">
                        <p
                          className={cn(
                            "truncate font-medium",
                            inactive && "text-muted-foreground"
                          )}
                        >
                          {account.name}
                        </p>
                        <p className="truncate text-xs text-muted-foreground">
                          {account.location}
                        </p>
                      </div>
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge
                      className={cn("border-0", statusStyles[account.status])}
                    >
                      {account.status}
                    </Badge>
                  </TableCell>
                  <TableCell
                    className={inactive ? "text-muted-foreground" : undefined}
                  >
                    {account.balance}
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {account.lastPayoutDate}
                  </TableCell>
                  <TableCell
                    className={inactive ? "text-muted-foreground" : undefined}
                  >
                    {account.totalProcessed}
                  </TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}
