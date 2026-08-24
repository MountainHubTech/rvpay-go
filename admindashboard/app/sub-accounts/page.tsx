import { AppSidebar } from "@/components/dashboard/app-sidebar"
import { SubAccountsTable } from "@/components/dashboard/sub-accounts-table"
import { Topbar } from "@/components/dashboard/topbar"

export default function SubAccountsPage() {
  return (
    <div className="flex min-h-screen">
      <AppSidebar />

      <div className="flex min-w-0 flex-1 flex-col">
        <Topbar />

        <main className="flex-1 bg-muted/30 p-6">
          <SubAccountsTable />
        </main>
      </div>
    </div>
  )
}
