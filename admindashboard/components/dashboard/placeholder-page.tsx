import { AppSidebar } from "@/components/dashboard/app-sidebar"
import { Topbar } from "@/components/dashboard/topbar"

export function PlaceholderPage({ title, description }: { title: string; description: string }) {
  return (
    <div className="flex min-h-screen">
      <AppSidebar />
      <div className="flex min-w-0 flex-1 flex-col">
        <Topbar />
        <main className="flex-1 bg-muted/30 p-6">
          <div className="rounded-xl border bg-white p-8">
            <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
            <p className="mt-2 text-sm text-muted-foreground">{description}</p>
          </div>
        </main>
      </div>
    </div>
  )
}