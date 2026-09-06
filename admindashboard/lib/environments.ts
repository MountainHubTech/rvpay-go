// Centralized environment configuration for the RVPAY Admin Dashboard.
//
// This is a temporary operational/debugging mechanism: a single deployed
// Dashboard can switch which backend deployment it talks to at runtime,
// without a rebuild. The selection is a client-side runtime setting
// (localStorage) — never a NEXT_PUBLIC_* build variable — and is resolved
// at REQUEST TIME by the API client (lib/api.ts), so switching takes effect
// on the next request without a reload.
//
// Do NOT hardcode these URLs anywhere else in the application.

export type DashboardEnvironment = "local" | "testing" | "production";

export const DEFAULT_ENVIRONMENT: DashboardEnvironment = "testing";

export const ENVIRONMENT_STORAGE_KEY = "rvpay-dashboard-environment";

export type EnvironmentConfig = {
  label: string;
  clientsBaseUrl: string;
  transactionsBaseUrl: string;
};

export const ENVIRONMENTS: Record<DashboardEnvironment, EnvironmentConfig> = {
  local: {
    label: "Local",
    clientsBaseUrl: "http://localhost:8080",
    transactionsBaseUrl: "http://localhost:8081",
  },
  testing: {
    label: "Testing",
    clientsBaseUrl: "https://api.rvpay.xyz",
    transactionsBaseUrl: "https://api.rvpay.xyz",
  },
  production: {
    label: "Production",
    clientsBaseUrl: "https://api.rvpay.co",
    transactionsBaseUrl: "https://api.rvpay.co",
  },
};

const ENVIRONMENT_IDS: readonly DashboardEnvironment[] = [
  "local",
  "testing",
  "production",
];

// isDashboardEnvironment narrows an arbitrary string (e.g. a value read back
// from localStorage) to a valid environment id; anything unknown falls back
// to the default rather than being cast blindly.
export function isDashboardEnvironment(
  value: unknown
): value is DashboardEnvironment {
  return (
    typeof value === "string" &&
    (ENVIRONMENT_IDS as readonly string[]).includes(value)
  );
}

// getSelectedEnvironment returns the environment currently selected in this
// browser, defaulting to "testing" on first visit. Safe to call during SSR:
// window/localStorage are only touched when they exist, and the module never
// reads storage at import time.
export function getSelectedEnvironment(): DashboardEnvironment {
  if (typeof window === "undefined") {
    return DEFAULT_ENVIRONMENT;
  }
  try {
    const stored = window.localStorage.getItem(ENVIRONMENT_STORAGE_KEY);
    return isDashboardEnvironment(stored) ? stored : DEFAULT_ENVIRONMENT;
  } catch {
    // Storage can be unavailable (private mode, permissions); fail safe.
    return DEFAULT_ENVIRONMENT;
  }
}

// setSelectedEnvironment persists the selection for subsequent visits.
// Returns false when persistence was not possible (SSR or blocked storage);
// the in-memory selection is still honored for the current session.
export function setSelectedEnvironment(environment: DashboardEnvironment): boolean {
  if (typeof window === "undefined") {
    return false;
  }
  try {
    window.localStorage.setItem(ENVIRONMENT_STORAGE_KEY, environment);
    return true;
  } catch {
    return false;
  }
}

// Environment-specific base URL resolvers. The API client calls these PER
// REQUEST so a switch in Settings takes effect immediately — never cache
// their result at module scope.
export function getClientsBaseUrl(): string {
  return ENVIRONMENTS[getSelectedEnvironment()].clientsBaseUrl;
}

export function getTransactionsBaseUrl(): string {
  return ENVIRONMENTS[getSelectedEnvironment()].transactionsBaseUrl;
}