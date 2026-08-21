import { AppSidebar } from "@/components/dashboard/app-sidebar"
import { NeedsAttention } from "@/components/dashboard/needs-attention"
import { PeriodSelect } from "@/components/dashboard/period-select"
import { RecentPayouts } from "@/components/dashboard/recent-payouts"
import { RevenueChart } from "@/components/dashboard/revenue-chart"
import { StatCards } from "@/components/dashboard/stat-cards"
import { Topbar } from "@/components/dashboard/topbar"

export default function Home() {
  return (
    <div className="flex min-h-screen">
      <AppSidebar />

      <div className="flex min-w-0 flex-1 flex-col">
        <Topbar />

        <main className="flex-1 space-y-6 bg-muted/30 p-6">
          <div className="flex items-center justify-between">
            <div>
              <h1 className="text-2xl font-semibold tracking-tight">
                Dashboard Overview
              </h1>
              <p className="text-sm text-muted-foreground">
                Your payments infrastructure check
              </p>
            </div>
            <PeriodSelect />
          </div>

          <StatCards />

          <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
            <div className="lg:col-span-2">
              <RevenueChart />
            </div>
            <NeedsAttention />
          </div>

          <RecentPayouts />
        </main>
      </div>
    </div>
  )
}
