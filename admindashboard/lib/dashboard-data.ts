export type Trend = {
  direction: "up" | "down"
  label: string
}

export type StatCard = {
  label: string
  value: string
  icon: "revenue" | "volume" | "subAccounts" | "payouts"
  trend?: Trend
  meta?: string
}

export const statCards: StatCard[] = [
  {
    label: "Total Revenue",
    value: "$1,245,670.00",
    icon: "revenue",
    trend: { direction: "up", label: "+12.5% vs last period" },
  },
  {
    label: "Transaction Volume",
    value: "45,210",
    icon: "volume",
    meta: "4.5k · 510 · 200",
  },
  {
    label: "Active Sub-Accounts",
    value: "1,842",
    icon: "subAccounts",
    meta: "+24 new this week",
  },
  {
    label: "Pending Payouts",
    value: "$342,100.50",
    icon: "payouts",
    meta: "Next payout run tomorrow",
  },
]

export const revenueOverTime = [
  { period: "Week 1", revenue: 68000 },
  { period: "Week 2", revenue: 92000 },
  { period: "Week 3", revenue: 81000 },
  { period: "Week 4", revenue: 118000 },
  { period: "Week 5", revenue: 96000 },
  { period: "Week 6", revenue: 142000 },
  { period: "Week 7", revenue: 168000 },
]

export type AttentionSeverity = "dispute" | "failed-payout"

export type AttentionItem = {
  id: string
  severity: AttentionSeverity
  title: string
  timeAgo: string
  actionLabel: string
}

export const needsAttention: AttentionItem[] = [
  {
    id: "att-1",
    severity: "dispute",
    title: "Arktribe disputed charge $450.00",
    timeAgo: "2h ago",
    actionLabel: "View details",
  },
  {
    id: "att-2",
    severity: "failed-payout",
    title: "Routing error for Applemelon ($1,200)",
    timeAgo: "5h ago",
    actionLabel: "Resolve issue",
  },
  {
    id: "att-3",
    severity: "dispute",
    title: "Adna Cards disputed charge $210.00",
    timeAgo: "1d ago",
    actionLabel: "View details",
  },
]

export type PayoutStatus = "Paid" | "Processing" | "Failed"

export type Payout = {
  subAccount: string
  amount: string
  status: PayoutStatus
  date: string
}

export const recentPayouts: Payout[] = [
  { subAccount: "1050MEDIA", amount: "$12,450.00", status: "Paid", date: "Oct 24, 2023" },
  { subAccount: "Achah Rosine", amount: "$8,200.50", status: "Paid", date: "Oct 24, 2023" },
  { subAccount: "Adna Cards", amount: "$45,100.00", status: "Processing", date: "Oct 24, 2023" },
  { subAccount: "AgriCam AI", amount: "$1,850.00", status: "Paid", date: "Oct 23, 2023" },
  { subAccount: "Applemelon", amount: "$9,420.00", status: "Failed", date: "Oct 23, 2023" },
]

export type SubAccountStatus = "Active" | "Restricted" | "Inactive"

export type SubAccount = {
  id: string
  initials: string
  avatarClass: string
  name: string
  location: string
  status: SubAccountStatus
  balance: string
  lastPayoutDate: string
  totalProcessed: string
}

export const subAccounts: SubAccount[] = [
  {
    id: "sa-1",
    initials: "AC",
    avatarClass: "bg-blue-100 text-blue-700",
    name: "1050MEDIA",
    location: "4439 Lynx, CT",
    status: "Active",
    balance: "$4,250.00",
    lastPayoutDate: "Oct 24, 2023",
    totalProcessed: "$128,450.00",
  },
  {
    id: "sa-2",
    initials: "GW",
    avatarClass: "bg-violet-100 text-violet-700",
    name: "Achah Rosine",
    location: "Buea Cameroon",
    status: "Restricted",
    balance: "$12,040.50",
    lastPayoutDate: "Oct 15, 2023",
    totalProcessed: "$89,200.00",
  },
  {
    id: "sa-3",
    initials: "TS",
    avatarClass: "bg-muted text-muted-foreground",
    name: "Adna Cards",
    location: "Ennovation Factory",
    status: "Inactive",
    balance: "$0.00",
    lastPayoutDate: "Jan 12, 2023",
    totalProcessed: "$5,400.00",
  },
  {
    id: "sa-4",
    initials: "NB",
    avatarClass: "bg-blue-100 text-blue-700",
    name: "Applemelon",
    location: "Bastos, Yde",
    status: "Active",
    balance: "$850.25",
    lastPayoutDate: "Oct 26, 2023",
    totalProcessed: "$42,100.00",
  },
]

export type PayoutOverviewStat = {
  label: string
  value: string
  meta: string
  metaTone: "muted" | "positive"
  icon: "pending" | "cleared" | "failed"
  alert?: string
}

export const payoutOverviewStats: PayoutOverviewStat[] = [
  {
    label: "Total Pending",
    value: "$42,500.00",
    meta: "12 payouts processing",
    metaTone: "muted",
    icon: "pending",
  },
  {
    label: "Total Cleared (MTD)",
    value: "$1.2M",
    meta: "+14% vs last month",
    metaTone: "positive",
    icon: "cleared",
  },
  {
    label: "Failed Payouts",
    value: "2",
    meta: "Totaling $1,450.00",
    metaTone: "muted",
    icon: "failed",
    alert: "Action Needed",
  },
]

export type PayoutOverviewStatus = "Failed" | "In Transit" | "Cleared" | "Pending"

export type PayoutOverviewRow = {
  id: string
  initials: string
  avatarClass: string
  name: string
  location: string
  amount: string
  status: PayoutOverviewStatus
  initiated: string
  expectedOrCleared: string
  expectedOrClearedStrong?: boolean
}

export const payoutOverviewRows: PayoutOverviewRow[] = [
  {
    id: "po-1",
    initials: "AC",
    avatarClass: "bg-blue-100 text-blue-700",
    name: "1050MEDIA",
    location: "4439 Lynx, CT",
    amount: "$1,250.00",
    status: "Failed",
    initiated: "Oct 24, 14:30 EST",
    expectedOrCleared: "-",
  },
  {
    id: "po-2",
    initials: "GW",
    avatarClass: "bg-violet-100 text-violet-700",
    name: "Achah Rosine",
    location: "Buea Cameroon",
    amount: "$8,400.00",
    status: "In Transit",
    initiated: "Oct 25, 09:15 EST",
    expectedOrCleared: "Est. Oct 27",
  },
  {
    id: "po-3",
    initials: "TS",
    avatarClass: "bg-muted text-muted-foreground",
    name: "Adna Cards",
    location: "Ennovation Factory",
    amount: "$12,950.50",
    status: "Cleared",
    initiated: "Oct 22, 16:45 EST",
    expectedOrCleared: "Oct 24, 08:30 EST",
    expectedOrClearedStrong: true,
  },
  {
    id: "po-4",
    initials: "NB",
    avatarClass: "bg-blue-100 text-blue-700",
    name: "AgriCam AI",
    location: "Bastos, Yde",
    amount: "$450.00",
    status: "Pending",
    initiated: "Oct 26, 11:10 EST",
    expectedOrCleared: "Est. Oct 28",
  },
]
