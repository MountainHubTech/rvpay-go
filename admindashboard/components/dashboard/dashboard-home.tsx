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
  fetchOverviewSnapshot,
  periodToWire,
  type OverviewSnapshotResponse,
} from "@/lib/api"
import type {
  DashboardPeriod,
  DashboardSnapshot,
  PayoutStatus,
  StatCard,
} from "@/lib/dashboard-data"

// Map the live overview snapshot to the dashboard's snapshot shape. Fields the
// backend documents as cross-service gaps (active sub-accounts, needs
// attention) surface as "—" / empty — never fabricated.
function snapshotFromResponse(response: OverviewSnapshotResponse): DashboardSnapshot {
  const currency = response.revenueCurrency || ""
  const statCards: StatCard[] = [
    {
      label: "Total Revenue",
      value: `${currency} ${response.totalRevenue.toLocaleString("en-US")}`,
      icon: "revenue",
    },
    {
      label: "Transaction Volume",
      value: response.transactionVolume.toLocaleString("en-US"),
      icon: "volume",
    },
    {
      label: "Active Sub-Accounts",
      value: "—",
      icon: "subAccounts",
      meta: "Requires cross-service aggregation",
    },
    {
      label: "Pending Payouts",
      value: response.pendingPayouts.toLocaleString("en-US"),
      icon: "payouts",
    },
  ]

  const revenueOverTime = response.revenueOverTime.map((bucket) => ({
    period: bucket.periodLabel,
    revenue: bucket.revenue,
  }))

  const recentPayouts = response.recentPayouts.map((payout) => ({
    subAccount: payout.subAccount,
    amount: payout.amount,
    status: payout.status as PayoutStatus,
    date: payout.date,
  }))

  return {
    statCards,
    revenueOverTime,
    needsAttention: [],
    recentPayouts,
  }
}

const emptySnapshot: DashboardSnapshot = {
  statCards: [],
  revenueOverTime: [],
  needsAttention: [],
  recentPayouts: [],
}

export function DashboardHome() {
  const [period, setPeriod] = React.useState<DashboardPeriod>("Last 7 Days")
  const [snapshot, setSnapshot] =
    React.useState<DashboardSnapshot>(emptySnapshot)
  const [loading, setLoading] = React.useState(true)
  const [loadError, setLoadError] = React.useState<string | null>(null)

  React.useEffect(() => {
    let cancelled = false
    setLoading(true)
    setLoadError(null)

    fetchOverviewSnapshot(periodToWire(period))
      .then((response) => {
        if (!cancelled) {
          setSnapshot(snapshotFromResponse(response))
          setLoading(false)
        }
      })
      .catch((error: unknown) => {
        if (!cancelled) {
          console.warn("[RVPay] overview snapshot fetch failed:", error)
          setLoadError("Overview data is currently unavailable.")
          setLoading(false)
        }
      })

    return () => {
      cancelled = true
    }
  }, [period])

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
          {loadError && (
            <p className="text-sm text-rose-600" role="alert">{loadError}</p>
          )}
          {loading ? (
            <p className="text-sm text-muted-foreground" role="status">Loading overview…</p>
          ) : (
            <>
              <StatCards cards={snapshot.statCards} />
              <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
                <div className="lg:col-span-2"><RevenueChart data={snapshot.revenueOverTime} /></div>
                <NeedsAttention items={snapshot.needsAttention} />
              </div>
              <RecentPayouts payouts={snapshot.recentPayouts} />
            </>
          )}
        </main>
      </div>
    </div>
  )
}
