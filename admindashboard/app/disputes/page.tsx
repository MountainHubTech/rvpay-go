"use client"

import * as React from "react"

import { AppSidebar } from "@/components/dashboard/app-sidebar"
import { AuthGuard } from "@/lib/auth"
import { Topbar } from "@/components/dashboard/topbar"
import { DisputesView } from "@/components/dashboard/disputes-view"

export default function DisputesPage() {
  // NO mock data: the Transactions service has no disputes capability yet
  // (documented in agents/new-pages-endpoints). The view renders truthful
  // zero/empty states until that backend work lands.
  return (
    <AuthGuard>
      <div className="flex min-h-screen">
        <AppSidebar />
        <div className="flex min-w-0 flex-1 flex-col">
          <Topbar />
          <main className="flex-1 bg-muted/30 p-6">
            <DisputesView backendAvailable={false} />
          </main>
        </div>
      </div>
    </AuthGuard>
  )
}