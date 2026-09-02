import { auth } from "@/auth"
import { IconBell, IconMenu2, IconSearch } from "@tabler/icons-react"
import UserMenu from "./UserMenu"
import { redirect } from "next/navigation"
import { federatedLogout } from "@/app/actions"
import { zitadelAccount } from "@/lib/zitadel-account"

interface DashboardTopbarProps {
  homeId?: string
}

export default async function DashboardTopbar({ homeId }: DashboardTopbarProps) {
  const session = await auth()
  if (!session) redirect("/")

  function isMeaningful(v: unknown): v is string {
    if (typeof v !== "string") return false
    const t = v.trim()
    return t.length > 0 && t !== "undefined" && t !== "undefined undefined"
  }

  let name: string | null | undefined = isMeaningful(session.user?.name) ? session.user!.name : undefined
  let email = session.user?.email
  let image = session.user?.image

  if (!name && session.accessToken) {
    try {
      const issuer = process.env.AUTH_ZITADEL_ISSUER
      const res = await fetch(`${issuer}/oidc/v1/userinfo`, {
        headers: { Authorization: `Bearer ${session.accessToken}` },
      })
      if (res.ok) {
        const info = await res.json()
        const composed = [info.given_name, info.family_name]
          .filter((s: unknown) => isMeaningful(s))
          .join(" ")
          .trim()
        name =
          (isMeaningful(info.name) ? info.name : "") ||
          composed ||
          (isMeaningful(info.preferred_username) ? info.preferred_username : "") ||
          (isMeaningful(info.email) ? info.email : "") ||
          undefined
        if (isMeaningful(info.email)) email = info.email
        if (isMeaningful(info.picture)) image = info.picture
      }
    } catch (err) {
      console.error("Failed to fetch user info fallback:", err)
    }
  }

  const manageMembersUrl = homeId ? `/dashboard/homes/${homeId}/members` : undefined

  return (
    <header className="fixed top-4 left-4 right-4 z-50 flex h-16 items-center justify-between rounded-full bg-orange-50/50 px-6 shadow-[inset_0_2px_4px_rgba(255,255,255,0.8),0_10px_30px_rgba(255,126,103,0.1)] backdrop-blur-md">
      <div className="flex items-center gap-3">
        <button className="p-1.5 text-stone-400 md:hidden" type="button">
          <IconMenu2 size={20} />
        </button>
        <h1 className="text-xl font-black italic tracking-tight text-orange-600">Zynix Systems</h1>
      </div>

      <div className="flex items-center gap-4">
        <div className="hidden items-center rounded-full border border-white/40 bg-white/60 px-4 py-1.5 shadow-[inset_2px_2px_4px_rgba(0,0,0,0.05),inset_-2px_-2px_4px_rgba(255,255,255,0.8)] sm:flex">
          <IconSearch size={14} className="mr-2 text-stone-400" />
          <input className="w-32 bg-transparent text-sm text-stone-600 outline-none transition-all placeholder:text-stone-400 focus:w-48" placeholder="Search devices..." type="text" />
        </div>

        <div className="flex items-center gap-2">
          <button className="relative rounded-full p-1.5 text-stone-400 shadow-[2px_2px_4px_rgba(0,0,0,0.05),-2px_-2px_4px_rgba(255,255,255,0.8)]" type="button">
            <IconBell size={18} />
            <span className="absolute right-1.5 top-1.5 h-1.5 w-1.5 rounded-full bg-primary" />
          </button>
        </div>

        <UserMenu
          name={name}
          email={email}
          image={image}
          accountSettingsUrl="/dashboard/account"
          manageMembersUrl={manageMembersUrl}
          signOutAction={federatedLogout}
        />
      </div>
    </header>
  )
}
