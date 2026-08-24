"use client"

import { useState } from "react"
import { ChevronDown } from "lucide-react"

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"

const periods = ["Last 7 Days", "Last 30 Days", "Last 90 Days", "This Year"]

export function PeriodSelect() {
  const [period, setPeriod] = useState(periods[0])

  return (
    <DropdownMenu>
      <DropdownMenuTrigger className="flex items-center gap-2 rounded-lg border bg-white px-3 py-2 text-sm font-medium hover:bg-muted">
        {period}
        <ChevronDown className="size-4 text-muted-foreground" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        {periods.map((option) => (
          <DropdownMenuItem key={option} onClick={() => setPeriod(option)}>
            {option}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
