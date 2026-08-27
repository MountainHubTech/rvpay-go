import { AlertTriangle } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { cn } from "@/lib/utils"
import { type AttentionItem, type AttentionSeverity } from "@/lib/dashboard-data"

const severityStyles: Record<
  AttentionSeverity,
  { badge: string; label: string }
> = {
  dispute: {
    badge: "bg-rose-100 text-rose-700",
    label: "DISPUTES",
  },
  "failed-payout": {
    badge: "bg-amber-100 text-amber-700",
    label: "FAILED PAYOUT",
  },
}

export function NeedsAttention({ items }: { items: AttentionItem[] }) {
  return (
    <Card className="h-full border-l-4 border-l-rose-400">
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle className="flex items-center gap-2">
            <AlertTriangle className="size-4 text-rose-500" />
            Needs Attention
          </CardTitle>
          <Badge variant="outline">{items.length} items</Badge>
        </div>
      </CardHeader>
      <CardContent className="flex flex-col divide-y">
        {items.map((item) => {
          const styles = severityStyles[item.severity]
          return (
            <div key={item.id} className="flex flex-col gap-1.5 py-3 first:pt-0 last:pb-0">
              <div className="flex items-center justify-between gap-2">
                <Badge className={cn("border-0", styles.badge)}>
                  {styles.label}
                </Badge>
                <span className="text-xs text-muted-foreground">
                  {item.timeAgo}
                </span>
              </div>
              <p className="text-sm font-medium">{item.title}</p>
              <button
                type="button"
                className="w-fit text-xs font-medium text-blue-600 hover:underline"
              >
                {item.actionLabel}
              </button>
            </div>
          )
        })}
      </CardContent>
    </Card>
  )
}
