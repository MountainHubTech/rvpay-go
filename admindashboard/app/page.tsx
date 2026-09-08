import { AuthGuard } from "@/lib/auth"
import { DashboardHome } from "@/components/dashboard/dashboard-home"

export default function Home() {
  return (
    <AuthGuard>
      <DashboardHome />
    </AuthGuard>
  )
}
