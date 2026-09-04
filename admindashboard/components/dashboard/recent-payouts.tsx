import { Eye } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { cn } from "@/lib/utils"
import { type DashboardSnapshot, type PayoutStatus } from "@/lib/dashboard-data"

const statusStyles: Record<PayoutStatus, string> = {
  Paid: "bg-emerald-100 text-emerald-700",
  Processing: "bg-amber-100 text-amber-700",
  Failed: "bg-rose-100 text-rose-700",
}

export function RecentPayouts({ payouts }: { payouts: DashboardSnapshot["recentPayouts"] }) {
  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle>Recent Payouts</CardTitle>
          <button
            type="button"
            className="rounded-lg border px-3 py-1.5 text-sm font-medium hover:bg-muted"
          >
            View All
          </button>
        </div>
      </CardHeader>
      <CardContent>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Sub-Account</TableHead>
              <TableHead>Amount</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Date</TableHead>
              <TableHead className="text-right">Action</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {payouts.map((payout) => (
              <TableRow key={payout.subAccount}>
                <TableCell className="font-medium">
                  {payout.subAccount}
                </TableCell>
                <TableCell>{payout.amount}</TableCell>
                <TableCell>
                  <Badge className={cn("border-0", statusStyles[payout.status])}>
                    {payout.status}
                  </Badge>
                </TableCell>
                <TableCell className="text-muted-foreground">
                  {payout.date}
                </TableCell>
                <TableCell className="text-right">
                  <button
                    type="button"
                    aria-label={`View ${payout.subAccount} payout`}
                    className="inline-flex size-7 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground"
                  >
                    <Eye className="size-4" />
                  </button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  )
}
