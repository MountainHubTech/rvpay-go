"use client"

import * as React from "react"
import { ArrowLeft } from "lucide-react"
import { useParams, useRouter, useSearchParams } from "next/navigation"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { useEffect } from "react"

const statusStyles: Record<string, string> = {
  Success: "bg-emerald-100 text-emerald-700",
  Pending: "bg-muted text-muted-foreground",
  Failed: "bg-rose-100 text-rose-700",
  Refunded: "bg-amber-100 text-amber-700",
}

export default function EmbedTransactionDetailsPage() {
  const router = useRouter()
  const params = useParams<{ transactionId: string }>()
  const searchParams = useSearchParams()
  const transactionId = params.transactionId
  const locationId = searchParams.get("locationId")
  const shortId = searchParams.get("shortId") || transactionId
  const customer = searchParams.get("customer") || "Unavailable"
  const amount = searchParams.get("amount") || "Unavailable"
  const status = searchParams.get("status") || "Unavailable"
  const provider = searchParams.get("provider") || "Unavailable"
  const date = searchParams.get("date") || "Unavailable"
  const isSample = searchParams.get("sample") === "true"

  function goBack() {
    const query = new URLSearchParams()
    if (locationId) query.set("locationId", locationId)
    router.push(`/embed/transactions${query.size ? `?${query.toString()}` : ""}`)
  }

  useEffect(() => {
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        const query = new URLSearchParams()
        if (locationId) query.set("locationId", locationId)
        router.push(`/embed/transactions${query.size ? `?${query.toString()}` : ""}`)
      }
    }

    window.addEventListener("keydown", handleKeyDown)
    return () => window.removeEventListener("keydown", handleKeyDown)
  }, [locationId, router])

  return (
    <main className="min-h-screen bg-muted/30 p-4 sm:p-6">
      <div className="mx-auto max-w-3xl space-y-5">
        <Button type="button" variant="ghost" onClick={goBack}>
          <ArrowLeft data-icon="inline-start" />
          Back to transactions
        </Button>

        {isSample && (
          <p className="rounded-md border border-amber-300 bg-amber-50 px-3 py-2 text-sm text-amber-900" role="status">
            Sample transaction details for visualisation only. This is not a real payment.
          </p>
        )}

        <Card className="overflow-hidden">
          <div className="flex flex-wrap items-start justify-between gap-4 border-b p-5 sm:p-6">
            <div>
              <p className="text-sm text-muted-foreground">Transaction</p>
              <h1 className="mt-1 font-mono text-xl font-semibold">{shortId}</h1>
            </div>
            <Badge className={`border-0 ${statusStyles[status] ?? "bg-muted text-muted-foreground"}`}>
              {status}
            </Badge>
          </div>

          <div className="grid gap-x-8 sm:grid-cols-2">
            <Detail label="Amount" value={amount} prominent />
            <Detail label="Customer" value={customer} />
            <Detail label="Mobile money provider" value={provider} />
            <Detail label="Date" value={date} />
            <Detail label="Transaction ID" value={transactionId} mono />
          </div>
        </Card>
      </div>
    </main>
  )
}

function Detail({ label, value, prominent = false, mono = false }: {
  label: string
  value: string
  prominent?: boolean
  mono?: boolean
}) {
  return (
    <div className="border-b px-5 py-4 sm:px-6">
      <p className="text-xs font-medium text-muted-foreground">{label}</p>
      <p className={`mt-1 break-all ${prominent ? "text-xl font-semibold" : "text-sm font-medium"} ${mono ? "font-mono" : ""}`}>
        {value}
      </p>
    </div>
  )
}