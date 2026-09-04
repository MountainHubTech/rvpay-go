import {
  ArrowUpRight,
  Banknote,
  Clock,
  Folders,
  TrendingUp,
} from "lucide-react"

import { Card } from "@/components/ui/card"
import { type StatCard } from "@/lib/dashboard-data"

const icons = {
  revenue: Banknote,
  volume: TrendingUp,
  subAccounts: Folders,
  payouts: Clock,
} as const

function StatCardItem({ card }: { card: StatCard }) {
  const Icon = icons[card.icon]

  return (
    <Card className="p-5">
      <div className="flex items-start justify-between">
        <span className="text-sm text-muted-foreground">{card.label}</span>
        <Icon className="size-4 text-muted-foreground" />
      </div>
      <p className="mt-2 text-2xl font-semibold tracking-tight">{card.value}</p>

      {card.trend && (
        <div className="mt-2 flex items-center gap-1 text-xs font-medium text-emerald-600">
          <ArrowUpRight className="size-3.5" />
          {card.trend.label}
        </div>
      )}

      {card.meta && !card.trend && (
        <p className="mt-2 flex items-center gap-1 text-xs text-muted-foreground">
          {card.icon === "payouts" && <Clock className="size-3.5" />}
          {card.meta}
        </p>
      )}
    </Card>
  )
}

export function StatCards({ cards }: { cards: StatCard[] }) {
  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
      {cards.map((card) => (
        <StatCardItem key={card.label} card={card} />
      ))}
    </div>
  )
}
