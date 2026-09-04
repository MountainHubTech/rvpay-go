"use client"

import * as React from "react"

import { AppSidebar } from "@/components/dashboard/app-sidebar"
import { NeedsAttention } from "@/components/dashboard/needs-attention"
import { PeriodSelect } from "@/components/dashboard/period-select"
import { RecentPayouts } from "@/components/dashboard/recent-payouts"
import { RevenueChart } from "@/components/dashboard/revenue-chart"
import { StatCards } from "@/components/dashboard/stat-cards"
import { Topbar } from "@/components/dashboard/topbar"
import {
  getDashboardSnapshot,
  type DashboardPeriod,
  type DashboardSnapshot,
} from "@/lib/dashboard-data"

export function DashboardHome({ initialSnapshot }: { initialSnapshot: DashboardSnapshot }) {
  const [period, setPeriod] = React.useState<DashboardPeriod>("Last 7 Days")
  const snapshot = period === "Last 7 Days" ? initialSnapshot : getDashboardSnapshot(period)

  return (
    <div className="flex min-h-screen">
      <AppSidebar />
      <div className="flex min-w-0 flex-1 flex-col">
        <Topbar />
        <main className="flex-1 space-y-6 bg-muted/30 p-6">
          <div className="flex items-center justify-between">
            <div>
              <h1 className="text-2xl font-semibold tracking-tight">Dashboard Overview</h1>
              <p className="text-sm text-muted-foreground">Your payments infrastructure check</p>
            </div>
            <PeriodSelect value={period} onChange={setPeriod} />
          </div>
          <StatCards cards={snapshot.statCards} />
          <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
            <div className="lg:col-span-2"><RevenueChart data={snapshot.revenueOverTime} /></div>
            <NeedsAttention items={snapshot.needsAttention} />
          </div>
          <RecentPayouts payouts={snapshot.recentPayouts} />
        </main>
      </div>
    </div>
  )
}
