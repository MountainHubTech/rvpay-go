"use client"

// Minimal administrator authentication context for the RVPAY Admin Dashboard.
//
// Design (deliberately small, no new dependencies):
// - Sign-in exchanges email/password for an opaque access/refresh token pair
//   issued by the Clients service (POST /v1/public/auth/sign-in).
// - The access token is held in memory and mirrored to sessionStorage so a
//   reload does not force a re-sign-in. No cookies; tokens are never written
//   to localStorage (they do not survive a browser restart) and never logged.
// - Expired access tokens are refreshed via POST /v1/public/auth/refresh by
//   the API client (lib/api.ts); refresh tokens are single-use and rotate.
// - Sign-out revokes the session server-side and clears the local state.

import * as React from "react"
import { useRouter } from "next/navigation"

export type AdminUser = {
  id: string
  name: string
  email: string
  userRole: string
}

type StoredSession = {
  accessToken: string
  refreshToken: string
  user: AdminUser
}

export const SESSION_STORAGE_KEY = "rvpay-dashboard-auth"

// readSession reads the stored session for this browser tab. Safe during SSR
// (sessionStorage is only touched when it exists).
export function readSession(): StoredSession | null {
  if (typeof window === "undefined") {
    return null
  }
  try {
    const raw = window.sessionStorage.getItem(SESSION_STORAGE_KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw) as Partial<StoredSession>
    if (
      typeof parsed.accessToken !== "string" ||
      typeof parsed.refreshToken !== "string" ||
      typeof parsed.user !== "object" ||
      parsed.user === null
    ) {
      return null
    }
    return parsed as StoredSession
  } catch {
    return null
  }
}

export function writeSession(session: StoredSession): void {
  if (typeof window === "undefined") return
  try {
    window.sessionStorage.setItem(SESSION_STORAGE_KEY, JSON.stringify(session))
  } catch {
    // Storage unavailable (private mode); the in-memory session still works.
  }
}

export function clearSession(): void {
  if (typeof window === "undefined") return
  try {
    window.sessionStorage.removeItem(SESSION_STORAGE_KEY)
  } catch {
    // Storage unavailable; nothing to clear.
  }
}

// getAccessToken returns the current access token for the API client. The
// sessionStorage mirror is authoritative for API calls made outside the
// React context (keeps lib/api.ts free of React imports).
export function getAccessToken(): string {
  return readSession()?.accessToken ?? ""
}

// getRefreshToken returns the current refresh token for the API client.
export function getRefreshToken(): string {
  return readSession()?.refreshToken ?? ""
}

// setTokens persists a rotated token pair (called by the API client after a
// successful refresh).
export function setTokens(accessToken: string, refreshToken: string): void {
  const session = readSession()
  if (!session) return
  writeSession({ ...session, accessToken, refreshToken })
}

// dropSession clears stored auth state without navigation (sign-out, 401).
export function dropSession(): void {
  clearSession()
}

type AuthContextValue = {
  user: AdminUser | null
  loading: boolean
  signIn: (email: string, password: string) => Promise<void>
  signOut: () => Promise<void>
}

const AuthContext = React.createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = React.useState<AdminUser | null>(null)
  const [loading, setLoading] = React.useState(true)

  React.useEffect(() => {
    setUser(readSession()?.user ?? null)
    setLoading(false)
  }, [])

  const signIn = React.useCallback(async (email: string, password: string) => {
    // Imported lazily to keep this module's dependency on the API layer
    // one-directional (api.ts imports session storage helpers from here).
    const { signInRequest } = await import("@/lib/api")
    const response = await signInRequest(email, password)
    const session: StoredSession = {
      accessToken: response.accessToken,
      refreshToken: response.refreshToken,
      user: response.user,
    }
    writeSession(session)
    setUser(session.user)
  }, [])

  const signOut = React.useCallback(async () => {
    try {
      const { signOutRequest } = await import("@/lib/api")
      await signOutRequest()
    } catch {
      // The server may already be unreachable or the session gone; local
      // cleanup happens regardless.
    }
    dropSession()
    setUser(null)
  }, [])

  const value = React.useMemo<AuthContextValue>(
    () => ({ user, loading, signIn, signOut }),
    [user, loading, signIn, signOut]
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth(): AuthContextValue {
  const value = React.useContext(AuthContext)
  if (!value) {
    throw new Error("useAuth must be used within an AuthProvider")
  }
  return value
}

// AuthGuard blocks protected pages until an administrator is signed in.
// Server middleware enforces authorization; this guard only prevents
// rendering protected UI for unauthenticated browsers.
export function AuthGuard({ children }: { children: React.ReactNode }) {
  const { user, loading } = useAuth()
  const router = useRouter()

  React.useEffect(() => {
    if (!loading && !user) {
      router.replace("/sign-in")
    }
  }, [loading, user, router])

  if (loading || !user) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-muted/30">
        <p className="text-sm text-muted-foreground">Checking session…</p>
      </div>
    )
  }

  return <>{children}</>
}