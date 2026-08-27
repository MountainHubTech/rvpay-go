import { DashboardHome } from "@/components/dashboard/dashboard-home"
import { getDashboardSnapshot } from "@/lib/dashboard-data"

export default function Home() {
  return <DashboardHome initialSnapshot={getDashboardSnapshot("Last 7 Days")} />
}
