import { auth } from "@/auth"
import PermissionTreeEditor from "@/components/PermissionTreeEditor"
import {
  type HomeMember,
  type PermissionScopeRef,
  type PermissionTree,
  solCore,
} from "@/lib/sol-core"
import { IconArrowLeft, IconShieldCheck, IconShieldLock } from "@tabler/icons-react"
import Link from "next/link"
import { redirect, unstable_rethrow } from "next/navigation"

export const dynamic = "force-dynamic"

type SearchParams = Promise<{ notice?: string; error?: string }>

function permissionsHref(homeId: string, userId: string, params: Record<string, string | undefined>) {
  const query = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (!value) continue
    query.set(key, value)
  }
  const encoded = query.toString()
  const base = `/dashboard/homes/${homeId}/members/${userId}/permissions`
  return encoded ? `${base}?${encoded}` : base
}

function membersHref(homeId: string, params: Record<string, string | undefined>) {
  const query = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (!value) continue
    query.set(key, value)
  }
  const encoded = query.toString()
  const base = `/dashboard/homes/${homeId}/members`
  return encoded ? `${base}?${encoded}` : base
}

function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message
  return "Something went wrong"
}

export default async function PermissionsPage({
  params,
  searchParams,
}: {
  params: Promise<{ homeId: string; userId: string }>
  searchParams: SearchParams
}) {
  const session = await auth()
  if (!session) redirect("/")

  const { homeId, userId } = await params
  const query = await searchParams

  const home = await solCore.homes.get(homeId).catch(() => null)
  if (!home || !home.id) {
    redirect(`/dashboard?error=${encodeURIComponent("Home not found")}`)
  }
  if (home.my_role !== "owner" && home.my_role !== "admin") {
    redirect(membersHref(homeId, { error: "Forbidden" }))
  }

  const tree = await solCore.homes.permissions.get(homeId, userId).catch((err) => {
    return { error: errorMessage(err) } as { error: string }
  })

  if ("error" in tree) {
    redirect(membersHref(homeId, { error: tree.error }))
  }

  // Find the target member's display info from the members list (small, single page).
  const membersPage = await solCore.homes
    .listMembers(homeId, { limit: 100 })
    .catch(() => ({ data: [] as HomeMember[], next_cursor: null, has_more: false }))
  const target = membersPage.data.find((m) => m.user_id === userId)
  const targetLabel = target?.user_name?.trim() || target?.user_email || userId

  async function saveAction(formData: FormData) {
    "use server"
    const raw = String(formData.get("grants") ?? "[]")
    let grants: PermissionScopeRef[] = []
    try {
      const parsed = JSON.parse(raw)
      if (Array.isArray(parsed)) {
        grants = parsed.filter(
          (g): g is PermissionScopeRef =>
            g &&
            typeof g === "object" &&
            (g.type === "room" || g.type === "device" || g.type === "appliance") &&
            typeof g.id === "string" &&
            g.id.length > 0,
        )
      }
    } catch {
      redirect(permissionsHref(homeId, userId, { error: "Invalid grant payload" }))
    }
    const rawManage = String(formData.get("manage_rooms") ?? "[]")
    let manageRooms: string[] = []
    try {
      const parsed = JSON.parse(rawManage)
      if (Array.isArray(parsed)) {
        manageRooms = parsed.filter(
          (id): id is string => typeof id === "string" && id.length > 0,
        )
      }
    } catch {
      /* ignore malformed manage_rooms — treat as empty */
    }
    try {
      await solCore.homes.permissions.set(homeId, userId, grants, manageRooms)
      redirect(membersHref(homeId, { notice: `Permissions updated for ${targetLabel}` }))
    } catch (error) {
      unstable_rethrow(error)
      redirect(permissionsHref(homeId, userId, { error: errorMessage(error) }))
    }
  }

  return (
    <div className="mx-auto max-w-4xl px-4 py-8">
      <div className="mb-4">
        <Link
          href={`/dashboard/homes/${homeId}/members`}
          className="inline-flex items-center gap-1 text-xs font-semibold text-on-surface-variant transition hover:text-on-surface"
        >
          <IconArrowLeft size={14} />
          Back to members
        </Link>
      </div>

      {query.notice ? (
        <p className="mb-4 rounded-2xl border border-white/40 bg-secondary-container/70 px-4 py-2.5 text-sm text-on-secondary-container">
          {query.notice}
        </p>
      ) : null}
      {query.error ? (
        <p className="mb-4 rounded-2xl border border-error/40 bg-error-container/70 px-4 py-2.5 text-sm text-error">
          {query.error}
        </p>
      ) : null}

      <header className="mb-8 px-2">
        <p className="text-xs font-semibold uppercase tracking-[0.18em] text-primary">
          {home.name}
        </p>
        <h2 className="mt-1 flex items-center gap-2 font-display text-[28px] font-bold tracking-[-0.02em] text-on-surface">
          <IconShieldLock size={26} className="text-primary" />
          Permissions
        </h2>
        <p className="mt-1 text-sm text-on-surface-variant">
          Choose which rooms, switchboards, and appliances{" "}
          <span className="font-semibold text-on-surface">{targetLabel}</span> can access.
        </p>
      </header>

      {tree.all_access ? (
        <section className="rounded-3xl border border-white/40 bg-surface-container p-6 shadow-[8px_8px_16px_rgba(27,28,25,0.06),-8px_-8px_16px_rgba(255,255,255,0.9)]">
          <div className="flex items-center gap-3">
            <IconShieldCheck size={28} className="text-primary" />
            <div>
              <h3 className="font-display text-lg font-semibold text-on-surface">
                Full access
              </h3>
              <p className="mt-1 text-sm text-on-surface-variant">
                {targetLabel} is {tree.role === "owner" ? "the owner" : "an admin"} of this
                home and can see and control everything. Demote them to a member first to
                set scoped permissions.
              </p>
            </div>
          </div>
        </section>
      ) : (
        <PermissionTreeEditor tree={tree as PermissionTree} action={saveAction} />
      )}
    </div>
  )
}
