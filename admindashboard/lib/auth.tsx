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

// readSession reads the stored session for this browser tab with a STABLE
// identity per stored value (required for useSyncExternalStore snapshots).
// Safe during SSR (sessionStorage is only touched when it exists).
let cachedRaw: string | null = null
let cachedSession: StoredSession | null = null

function parseSession(raw: string): StoredSession | null {
  try {
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

export function readSession(): StoredSession | null {
  if (typeof window === "undefined") {
    return null
  }
  const raw = window.sessionStorage.getItem(SESSION_STORAGE_KEY)
  if (raw === cachedRaw) {
    return cachedSession
  }
  cachedRaw = raw
  cachedSession = raw === null ? null : parseSession(raw)
  return cachedSession
}

// External-store plumbing: subscribers are notified whenever the session
// changes (local writes, cross-tab storage events). React subscribes via
// useSyncExternalStore — no effect ever calls setState (react-hooks
// set-state-in-effect rule).
const listeners = new Set<() => void>()

function notify(): void {
  for (const listener of listeners) {
    listener()
  }
}

export function subscribeToSession(listener: () => void): () => void {
  listeners.add(listener)
  window.addEventListener("storage", listener)
  return () => {
    listeners.delete(listener)
    window.removeEventListener("storage", listener)
  }
}

function getServerSnapshot(): StoredSession | null {
  return null
}

export function writeSession(session: StoredSession): void {
  if (typeof window === "undefined") return
  try {
    window.sessionStorage.setItem(SESSION_STORAGE_KEY, JSON.stringify(session))
    notify()
  } catch {
    // Storage unavailable (private mode); the in-memory session still works.
  }
}

export function clearSession(): void {
  if (typeof window === "undefined") return
  try {
    window.sessionStorage.removeItem(SESSION_STORAGE_KEY)
    notify()
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
  signIn: (email: string, password: string) => Promise<void>
  signOut: () => Promise<void>
}

const AuthContext = React.createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: React.ReactNode }) {
  // The session is an EXTERNAL STORE (sessionStorage + subscribers), read
  // with useSyncExternalStore: SSR-safe and never setState-in-effect.
  const session = React.useSyncExternalStore(
    subscribeToSession,
    readSession,
    getServerSnapshot
  )
  const user = session?.user ?? null

  const signIn = React.useCallback(async (email: string, password: string) => {
    // Imported lazily to keep this module's dependency on the API layer
    // one-directional (api.ts imports session storage helpers from here).
    const { signInRequest } = await import("@/lib/api")
    const response = await signInRequest(email, password)
    writeSession({
      accessToken: response.accessToken,
      refreshToken: response.refreshToken,
      user: response.user,
    })
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
  }, [])

  const value = React.useMemo<AuthContextValue>(
    () => ({ user, signIn, signOut }),
    [user, signIn, signOut]
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
// rendering protected UI for unauthenticated browsers. During SSR/hydration
// the session snapshot is null, so the placeholder renders first and the
// client store resolves immediately after.
export function AuthGuard({ children }: { children: React.ReactNode }) {
  const { user } = useAuth()
  const router = useRouter()

  React.useEffect(() => {
    if (!user) {
      router.replace("/sign-in")
    }
  }, [user, router])

  if (!user) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-muted/30">
        <p className="text-sm text-muted-foreground">Checking session…</p>
      </div>
    )
  }

  return <>{children}</>
}