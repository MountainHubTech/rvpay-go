"use client"

import * as React from "react"

import { AppSidebar } from "@/components/dashboard/app-sidebar"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { Topbar } from "@/components/dashboard/topbar"
import {
  DEFAULT_ENVIRONMENT,
  ENVIRONMENTS,
  ENVIRONMENT_STORAGE_KEY,
  getSelectedEnvironment,
  isDashboardEnvironment,
  setSelectedEnvironment,
  type DashboardEnvironment,
} from "@/lib/environments"

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

  function selectEnvironment(next: DashboardEnvironment) {
    if (!isDashboardEnvironment(next)) {
      return
    }
    setSelectedEnvironment(next)
    // Notify subscribers in this tab (the storage event only fires elsewhere).
    if (typeof window !== "undefined") {
      window.dispatchEvent(new Event(ENVIRONMENT_STORAGE_KEY))
    }
  }

  return (
    <div className="flex min-h-screen">
      <AppSidebar />

      <div className="flex min-w-0 flex-1 flex-col">
        <Topbar />

        <main className="flex-1 bg-muted/30 p-6">
          <Card className="p-8">
            <h1 className="text-2xl font-semibold tracking-tight">Settings</h1>
            <p className="mt-2 text-sm text-muted-foreground">
              Manage your RVPAY workspace and integration preferences.
            </p>

            <section className="mt-8 border-t pt-6">
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
            </section>
          </Card>
        </main>
      </div>
    </div>
  )
}