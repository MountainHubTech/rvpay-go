"use client"

import Link from "next/link"
import { usePathname } from "next/navigation"
import {
  LayoutDashboard,
  Users,
  ArrowLeftRight,
  Wallet,
  AlertTriangle,
  Settings,
  LogOut,
  type LucideIcon,
} from "lucide-react"

import { cn } from "@/lib/utils"

type NavItem = {
  label: string
  icon: LucideIcon
  href: string
}

const navItems: NavItem[] = [
  { label: "Dashboard", icon: LayoutDashboard, href: "/" },
  { label: "Sub-Accounts", icon: Users, href: "/sub-accounts" },
  { label: "Transactions", icon: ArrowLeftRight, href: "/transactions" },
  { label: "Payouts", icon: Wallet, href: "/payouts" },
  { label: "Disputes & Errors", icon: AlertTriangle, href: "/disputes" },
  { label: "Settings", icon: Settings, href: "/settings" },
]

export function AppSidebar() {
  const pathname = usePathname()

  return (
    <aside className="flex h-full w-56 shrink-0 flex-col border-r bg-white">
      <div className="flex h-16 items-center px-6">
        <span className="text-lg font-bold tracking-tight">RVPAY</span>
      </div>

      <nav className="flex flex-1 flex-col gap-1 px-3">
        {navItems.map((item) => {
          const Icon = item.icon
          const active = pathname === item.href
          const content = (
            <span
              className={cn(
                "flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors",
                active
                  ? "bg-blue-50 text-blue-600"
                  : "text-muted-foreground hover:bg-muted hover:text-foreground"
              )}
            >
              <Icon className="size-4" />
              {item.label}
            </span>
          )

          return <Link key={item.label} href={item.href}>{content}</Link>
        })}
      </nav>

      <div className="flex items-center gap-3 border-t px-4 py-4">
        <div className="size-8 shrink-0 rounded-full border bg-muted" />
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-medium">Admin User</p>
          <p className="truncate text-xs text-muted-foreground">
            admin@rvpay.com
          </p>
        </div>
        <button
          type="button"
          aria-label="Log out"
          className="flex size-7 shrink-0 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground"
        >
          <LogOut className="size-4" />
        </button>
      </div>
    </aside>
  )
}
