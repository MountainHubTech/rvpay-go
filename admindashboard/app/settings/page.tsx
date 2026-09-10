"use client"

import * as React from "react"

import { AppSidebar } from "@/components/dashboard/app-sidebar"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { Topbar } from "@/components/dashboard/topbar"
import { cn } from "@/lib/utils"
import {
  DEFAULT_ENVIRONMENT,
  ENVIRONMENTS,
  ENVIRONMENT_STORAGE_KEY,
  getSelectedEnvironment,
  isDashboardEnvironment,
  setSelectedEnvironment,
  type DashboardEnvironment,
} from "@/lib/environments"
import {
  describeApiError,
  fetchUsers,
  probeService,
  type UsersListResponse,
} from "@/lib/api"

// The selected environment is treated as an external store (localStorage) and
// read via useSyncExternalStore so the component stays SSR-safe (the server
// snapshot is the deterministic default) and reacts to changes without a
// setState-in-effect pattern. Storage events keep other tabs in sync.
function subscribeToEnvironment(onChange: () => void): () => void {
  if (typeof window === "undefined") {
    return () => {}
  }
  window.addEventListener("storage", onChange)
  window.addEventListener(ENVIRONMENT_STORAGE_KEY, onChange)
  return () => {
    window.removeEventListener("storage", onChange)
    window.removeEventListener(ENVIRONMENT_STORAGE_KEY, onChange)
  }
}

type ProbeResult = { ok: boolean; detail: string } | null

// Temporary operational/debugging feature: lets one deployed Dashboard
// switch which backend deployment it talks to at runtime. The selection is
// persisted to localStorage and resolved per-request by lib/api.ts, so the
// next API call uses the new environment immediately (no rebuild, no reload).
export default function SettingsPage() {
  const environment = React.useSyncExternalStore(
    subscribeToEnvironment,
    getSelectedEnvironment,
    () => DEFAULT_ENVIRONMENT
  )

  const config = ENVIRONMENTS[environment]
  const [probes, setProbes] = React.useState<Record<"clients" | "transactions", ProbeResult>>({
    clients: null,
    transactions: null,
  })
  const [probing, setProbing] = React.useState(false)

  // Team management list — driven by the EXISTING ListUsers RPC
  // (GET /v1/public/clients/users). Rows come from the backend; the Invite
  // action stays disabled until an invite endpoint lands.
  const [users, setUsers] = React.useState<UsersListResponse["rows"]>([])
  const [usersLoading, setUsersLoading] = React.useState(false)
  const [usersError, setUsersError] = React.useState<string | null>(null)

  // Mixed content: an HTTPS page cannot call HTTP (Local) APIs — the browser
  // blocks the request before it reaches the network. Detect and explain it
  // instead of letting it surface as an opaque "Failed to fetch".
  const isHttpsPage =
    typeof window !== "undefined" && window.location.protocol === "https:"
  const mixedContentWarning =
    isHttpsPage &&
    (config.clientsBaseUrl.startsWith("http:") ||
      config.transactionsBaseUrl.startsWith("http:"))

  function selectEnvironment(next: DashboardEnvironment) {
    if (!isDashboardEnvironment(next)) {
      return
    }
    setSelectedEnvironment(next)
    setProbes({ clients: null, transactions: null })
    // Notify subscribers in this tab (the storage event only fires elsewhere).
    if (typeof window !== "undefined") {
      window.dispatchEvent(new Event(ENVIRONMENT_STORAGE_KEY))
    }
  }

  async function runConnectionTest() {
    setProbing(true)
    const [clients, transactions] = await Promise.all([
      probeService("clients"),
      probeService("transactions"),
    ])
    const describe = (result: { ok: boolean; status: number | null; errorName: string | null }): ProbeResult =>
      result.ok
        ? { ok: true, detail: `reachable (HTTP ${result.status})` }
        : result.status !== null
          ? { ok: false, detail: `HTTP ${result.status}` }
          : {
              ok: false,
              detail: mixedContentWarning
                ? "blocked by the browser (likely mixed content: HTTPS page → HTTP API, or the service is unreachable)"
                : "unreachable from this browser (service down, wrong host, or CORS-blocked)",
            }
    setProbes({ clients: describe(clients), transactions: describe(transactions) })
    setProbing(false)
  }

  // Load the team member list once on mount. The list endpoint is protected
  // by the same admin middleware as the rest of the dashboard.
  React.useEffect(() => {
    let cancelled = false
    setUsersLoading(true)
    setUsersError(null)
    fetchUsers({ page: 1, pageSize: 100 })
      .then((response: UsersListResponse) => {
        if (!cancelled) setUsers(response.rows)
      })
      .catch((error: unknown) => {
        if (!cancelled) {
          console.warn("[RVPay] users fetch failed:", error)
          setUsersError(describeApiError(error, "Team members are currently unavailable."))
          setUsers([])
        }
      })
      .finally(() => {
        if (!cancelled) setUsersLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [])

  return (
    <div className="flex min-h-screen">
      <AppSidebar />

      <div className="flex min-w-0 flex-1 flex-col">
        <Topbar />

        <main className="flex-1 bg-muted/30 p-6">
          <div className="space-y-6">
            <div>
              <h1 className="text-2xl font-semibold tracking-tight">Settings</h1>
              <p className="mt-1 text-sm text-muted-foreground">
                Manage integration health and team access.
              </p>
            </div>

            {/* Team Management (per supplied design). Wired to the EXISTING
                ListUsers RPC (GET /v1/public/clients/users). The Invite action
                stays disabled until an invite endpoint lands. */}
            {usersError && (
              <p className="text-sm text-rose-600" role="alert">{usersError}</p>
            )}
            <Card className="p-6">
              <div className="flex items-start justify-between gap-4">
                <div>
                  <h2 className="text-lg font-semibold">Team Management</h2>
                  <p className="mt-1 text-sm text-muted-foreground">
                    Control access and roles for this integration.
                  </p>
                </div>
                <Button type="button" disabled title="Invite endpoint is not yet available">
                  Invite Member
                </Button>
              </div>

              <div className="mt-4 overflow-hidden rounded-lg border">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b bg-muted/40 text-left text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                      <th className="px-4 py-3">User</th>
                      <th className="px-4 py-3">Role</th>
                      <th className="px-4 py-3">Status</th>
                      <th className="px-4 py-3">Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {users.map((user) => (
                      <tr key={user.id} className="border-b last:border-0">
                        <td className="px-4 py-3">
                          <div className="font-medium">{user.name}</div>
                          <div className="text-xs text-muted-foreground">{user.email}</div>
                        </td>
                        <td className="px-4 py-3 text-muted-foreground">{user.role}</td>
                        <td className="px-4 py-3">
                          <span className={cn(
                            "inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium",
                            user.status === "Active"
                              ? "bg-emerald-100 text-emerald-700"
                              : "bg-muted text-muted-foreground"
                          )}>
                            {user.status}
                          </span>
                        </td>
                        <td className="px-4 py-3 text-muted-foreground">{user.dateJoined}</td>
                      </tr>
                    ))}
                    {usersLoading && (
                      <tr>
                        <td colSpan={4} className="px-4 py-10 text-center text-muted-foreground" role="status">
                          Loading team members…
                        </td>
                      </tr>
                    )}
                    {!usersLoading && users.length === 0 && (
                      <tr>
                        <td colSpan={4} className="px-4 py-10 text-center text-muted-foreground">
                          No team members to display yet.
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            </Card>

            {/* Environment selector — EXISTING functionality, preserved
                verbatim and relocated to the bottom of the Settings page per
                the agent directive. */}
            <Card className="p-8">
              <section>
                <h2 className="text-sm font-semibold">Environment</h2>
                <p className="mt-1 text-sm text-muted-foreground">
                  Select which backend deployment this dashboard communicates
                  with. The change applies to subsequent API requests immediately
                  and persists in this browser.
                </p>

              <div className="mt-4 flex flex-wrap items-center gap-2">
                {(Object.keys(ENVIRONMENTS) as DashboardEnvironment[]).map(
                  (id) => (
                    <Button
                      key={id}
                      type="button"
                      variant={environment === id ? "default" : "outline"}
                      aria-pressed={environment === id}
                      onClick={() => selectEnvironment(id)}
                    >
                      {ENVIRONMENTS[id].label}
                    </Button>
                  )
                )}
              </div>

              <p className="mt-4 text-sm">
                Current environment:{" "}
                <span className="font-semibold">{config.label}</span>
              </p>

              {mixedContentWarning && (
                <p className="mt-4 rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800" role="alert">
                  This Dashboard is served over HTTPS and the selected
                  environment uses an HTTP (localhost) API. Browsers block
                  mixed-content requests, so API calls will fail with
                  &ldquo;Failed to fetch&rdquo;. For Local development, run the
                  Dashboard over HTTP (e.g. http://localhost:3000).
                </p>
              )}

              <dl className="mt-4 space-y-2 text-sm">
                <div className="flex flex-wrap gap-2">
                  <dt className="text-muted-foreground">Clients API:</dt>
                  <dd className="font-mono">{config.clientsBaseUrl}</dd>
                </div>
                <div className="flex flex-wrap gap-2">
                  <dt className="text-muted-foreground">Transactions API:</dt>
                  <dd className="font-mono">{config.transactionsBaseUrl}</dd>
                </div>
              </dl>

              <div className="mt-4 flex flex-wrap items-center gap-3 text-sm">
                <Button type="button" variant="outline" onClick={runConnectionTest} disabled={probing}>
                  {probing ? "Testing…" : "Test Connection"}
                </Button>
                {probes.clients && (
                  <span className={probes.clients.ok ? "text-emerald-600" : "text-rose-600"}>
                    Clients: {probes.clients.ok ? "✓" : "✗"} {probes.clients.detail}
                  </span>
                )}
                {probes.transactions && (
                  <span className={probes.transactions.ok ? "text-emerald-600" : "text-rose-600"}>
                    Transactions: {probes.transactions.ok ? "✓" : "✗"} {probes.transactions.detail}
                  </span>
                )}
              </div>
              </section>
            </Card>
          </div>
        </main>
      </div>
    </div>
  )
}