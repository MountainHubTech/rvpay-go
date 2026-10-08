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

export type DashboardEnvironment =
  | "local"
  | "testing"
  | "production"
  | "contabo-testing"
  | "contabo-production";

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
  // Contabo VPS environments, served under temporary sslip.io hostnames
  // until the real domains are pointed at them (see deploy/contabo/SETUP.md).
  "contabo-testing": {
    label: "Contabo Testing",
    clientsBaseUrl: "https://api.testing.75-119-147-69.sslip.io",
    transactionsBaseUrl: "https://api.testing.75-119-147-69.sslip.io",
  },
  "contabo-production": {
    label: "Contabo Production",
    clientsBaseUrl: "https://api.production.75-119-147-69.sslip.io",
    transactionsBaseUrl: "https://api.production.75-119-147-69.sslip.io",
  },
};

const ENVIRONMENT_IDS: readonly DashboardEnvironment[] = [
  "local",
  "testing",
  "production",
  "contabo-testing",
  "contabo-production",
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

// The environment each known dashboard hostname talks to when the visitor
// has not picked one in Settings, so e.g. admindashboard.rvpay.co works
// without a manual switch. localStorage is per-origin, so a choice made on
// one hostname never leaks into another.
const HOSTNAME_ENVIRONMENTS: Record<string, DashboardEnvironment> = {
  "admindashboard.rvpay.co": "production",
  "admindashboard.rvpay.xyz": "testing",
  "admindashboard.testing.75-119-147-69.sslip.io": "contabo-testing",
  "admindashboard.production.75-119-147-69.sslip.io": "contabo-production",
  localhost: "local",
  "127.0.0.1": "local",
};

// defaultEnvironmentForHost returns the environment matching the hostname
// the dashboard is served from, or DEFAULT_ENVIRONMENT for unknown hosts.
export function defaultEnvironmentForHost(hostname: string): DashboardEnvironment {
  return HOSTNAME_ENVIRONMENTS[hostname.toLowerCase()] ?? DEFAULT_ENVIRONMENT;
}

// getSelectedEnvironment returns the environment currently selected in this
// browser, defaulting on first visit to the one matching the current hostname
// (see HOSTNAME_ENVIRONMENTS). Safe to call during SSR: window/localStorage
// are only touched when they exist, and the module never reads storage at
// import time.
export function getSelectedEnvironment(): DashboardEnvironment {
  if (typeof window === "undefined") {
    return DEFAULT_ENVIRONMENT;
  }
  const hostDefault = defaultEnvironmentForHost(window.location.hostname);
  try {
    const stored = window.localStorage.getItem(ENVIRONMENT_STORAGE_KEY);
    return isDashboardEnvironment(stored) ? stored : hostDefault;
  } catch {
    // Storage can be unavailable (private mode, permissions); fail safe.
    return hostDefault;
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