import { Bell, Search } from "lucide-react"

export function Topbar() {
  return (
    <header className="flex h-16 shrink-0 items-center gap-4 border-b bg-white px-6">
      <h2 className="text-sm font-semibold whitespace-nowrap">Payments monitor</h2>

      <div className="relative flex-1 max-w-xl">
        <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
        <input
          type="search"
          placeholder="Search sub-accounts"
          className="h-9 w-full rounded-full border bg-muted/40 pl-9 pr-4 text-sm outline-none placeholder:text-muted-foreground focus:ring-2 focus:ring-ring"
        />
      </div>

      <button
        type="button"
        aria-label="Notifications"
        className="flex size-9 items-center justify-center rounded-full text-muted-foreground hover:bg-muted hover:text-foreground"
      >
        <Bell className="size-4" />
      </button>
    </header>
  )
}
