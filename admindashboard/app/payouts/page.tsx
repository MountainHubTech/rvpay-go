import { AppSidebar } from "@/components/dashboard/app-sidebar"
import { PayoutsOverview } from "@/components/dashboard/payouts-overview"
import { Topbar } from "@/components/dashboard/topbar"
import { payoutOverviewRows } from "@/lib/dashboard-data"

export default function PayoutsPage() {
  return (
    <div className="flex min-h-screen">
      <AppSidebar />

      <div className="flex min-w-0 flex-1 flex-col">
        <Topbar />

        <main className="flex-1 bg-muted/30 p-6">
          <PayoutsOverview rows={payoutOverviewRows} />
        </main>
      </div>
    </div>
  )
}
