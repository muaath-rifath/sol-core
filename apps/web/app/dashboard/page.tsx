import { auth } from "@/auth"
import { solCore } from "@/lib/sol-core"
import { cookies } from "next/headers"
import { redirect } from "next/navigation"

export const dynamic = "force-dynamic"

export default async function DashboardPage() {
  const session = await auth()
  if (!session) {
    redirect("/")
  }

  const cookieStore = await cookies()
  const lastHome = cookieStore.get("last_home")?.value

  if (lastHome) {
    redirect(`/dashboard/homes/${lastHome}`)
  }

  try {
    const homes = await solCore.homes.list({ limit: 1 })
    if (homes.data[0]) {
      redirect(`/dashboard/homes/${homes.data[0].id}`)
    }
  } catch {
    // fall through to empty state
  }

  redirect("/dashboard/homes/none")
}
