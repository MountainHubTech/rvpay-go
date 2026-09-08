"use client"

// Administrator sign-in for the RVPAY Admin Dashboard. Credentials go only
// to the Clients service sign-in endpoint over HTTPS; they are never logged.

import * as React from "react"
import { useRouter } from "next/navigation"

import { useAuth } from "@/lib/auth"
import { describeApiError } from "@/lib/api"

export default function SignInPage() {
  const { user, loading, signIn } = useAuth()
  const router = useRouter()

  const [email, setEmail] = React.useState("")
  const [password, setPassword] = React.useState("")
  const [submitting, setSubmitting] = React.useState(false)
  const [error, setError] = React.useState<string | null>(null)

  React.useEffect(() => {
    if (!loading && user) {
      router.replace("/")
    }
  }, [loading, user, router])

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (submitting) return
    setSubmitting(true)
    setError(null)
    try {
      await signIn(email, password)
      router.replace("/")
    } catch (caught: unknown) {
      setError(describeApiError(caught, "Sign-in failed. Check your credentials and try again."))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-muted/30 px-4">
      <div className="w-full max-w-sm rounded-xl border bg-white p-8 shadow-sm">
        <div className="mb-6 text-center">
          <p className="text-lg font-bold tracking-tight">RVPAY</p>
          <h1 className="mt-2 text-xl font-semibold">Administrator sign-in</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Access to the payments monitor is restricted.
          </p>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-1">
            <label htmlFor="email" className="text-sm font-medium">Email</label>
            <input
              id="email"
              type="email"
              autoComplete="username"
              required
              value={email}
              onChange={(event) => setEmail(event.target.value)}
              className="h-9 w-full rounded-lg border bg-background px-3 text-sm outline-none focus:ring-2 focus:ring-ring"
            />
          </div>
          <div className="space-y-1">
            <label htmlFor="password" className="text-sm font-medium">Password</label>
            <input
              id="password"
              type="password"
              autoComplete="current-password"
              required
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              className="h-9 w-full rounded-lg border bg-background px-3 text-sm outline-none focus:ring-2 focus:ring-ring"
            />
          </div>

          {error && (
            <p className="text-sm text-rose-600" role="alert">{error}</p>
          )}

          <button
            type="submit"
            disabled={submitting}
            className="h-9 w-full rounded-lg bg-blue-600 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {submitting ? "Signing in…" : "Sign in"}
          </button>
        </form>
      </div>
    </div>
  )
}