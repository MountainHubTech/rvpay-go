"use client"

import { MoreHorizontal } from "lucide-react"
import { Bar, BarChart, CartesianGrid, Cell, XAxis } from "recharts"

import {
  Card,
  CardAction,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "@/components/ui/chart"
import { type DashboardSnapshot } from "@/lib/dashboard-data"

const chartConfig = {
  revenue: {
    label: "Revenue",
  },
} satisfies ChartConfig

const barShades = [
  "#e5e5e5",
  "#d4d4d4",
  "#b8b8b8",
  "#9a9a9a",
  "#7d7d7d",
  "#4d4d4d",
  "#262626",
]

export function RevenueChart({ data }: { data: DashboardSnapshot["revenueOverTime"] }) {
  return (
    <Card className="h-full">
      <CardHeader>
        <CardTitle>Revenue Over Time</CardTitle>
        <CardAction>
          <button
            type="button"
            aria-label="Chart options"
            className="flex size-7 items-center justify-center rounded-md text-muted-foreground hover:bg-muted"
          >
            <MoreHorizontal className="size-4" />
          </button>
        </CardAction>
      </CardHeader>
      <CardContent>
        <ChartContainer config={chartConfig} className="h-64 w-full">
          <BarChart data={data} barCategoryGap="24%">
            <CartesianGrid vertical={false} stroke="var(--border)" />
            <XAxis
              dataKey="period"
              tickLine={false}
              axisLine={false}
              tickMargin={8}
              tick={{ fontSize: 12, fill: "var(--muted-foreground)" }}
            />
            <ChartTooltip cursor={false} content={<ChartTooltipContent />} />
            <Bar dataKey="revenue" radius={6}>
              {data.map((entry, index) => (
                <Cell key={entry.period} fill={barShades[index % barShades.length]} />
              ))}
            </Bar>
          </BarChart>
        </ChartContainer>
      </CardContent>
    </Card>
  )
}
