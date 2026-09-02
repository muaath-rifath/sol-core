import { auth } from "@/auth"
import MemberRoleSelect from "@/components/MemberRoleSelect"
import { type HomeInvitation, type HomeMember, type MemberRole, solCore } from "@/lib/sol-core"
import {
  IconCrown,
  IconMail,
  IconShieldLock,
  IconUserMinus,
  IconUserPlus,
} from "@tabler/icons-react"
import Link from "next/link"
import { redirect, unstable_rethrow } from "next/navigation"

export const dynamic = "force-dynamic"

type MembersPageSearchParams = Promise<{
  notice?: string
  error?: string
}>

function membersHref(homeID: string, params: Record<string, string | undefined>) {
  const query = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (!value) continue
    query.set(key, value)
  }
  const encoded = query.toString()
  return encoded
    ? `/dashboard/homes/${homeID}/members?${encoded}`
    : `/dashboard/homes/${homeID}/members`
}

function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message
  return "Something went wrong"
}

function initials(name: string | null | undefined, fallback: string | null | undefined): string {
  const source = ((name ?? "").trim() || (fallback ?? "").trim()).trim()
  if (!source) return "?"
  return (
    source
      .split(/\s+/)
      .slice(0, 2)
      .map((w) => w[0]?.toUpperCase() ?? "")
      .join("") || "?"
  )
}

function formatDate(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" })
}

function roleBadgeClass(role: MemberRole): string {
  switch (role) {
    case "owner":
      return "bg-primary-container text-on-primary-container"
    case "admin":
      return "bg-secondary-fixed text-on-secondary-fixed"
    default:
      return "bg-surface-container-high text-on-surface-variant"
  }
}

export default async function MembersPage({
  params,
  searchParams,
}: {
  params: Promise<{ homeId: string }>
  searchParams: MembersPageSearchParams
}) {
  const session = await auth()
  if (!session) redirect("/")

  const { homeId } = await params
  const query = await searchParams

  const home = await solCore.homes.get(homeId).catch(() => null)
  if (!home || !home.id) {
    redirect(`/dashboard?error=${encodeURIComponent("Home not found")}`)
  }

  const myRole: MemberRole = home.my_role ?? "member"
  const canManage = myRole === "owner" || myRole === "admin"
  const isOwner = myRole === "owner"

  const [membersPage, invitationsPage] = await Promise.all([
    solCore.homes.listMembers(homeId, { limit: 100 }).catch(() => ({
      data: [] as HomeMember[],
      next_cursor: null,
      has_more: false,
    })),
    canManage
      ? solCore.homes
          .listInvitations(homeId, { status: "pending", limit: 100 })
          .catch(() => ({ data: [] as HomeInvitation[], next_cursor: null, has_more: false }))
      : Promise.resolve({ data: [] as HomeInvitation[], next_cursor: null, has_more: false }),
  ])

  const sessionEmail = (session.user?.email ?? "").trim().toLowerCase()
  const me = sessionEmail
    ? membersPage.data.find(
        (m) => (m.user_email ?? "").trim().toLowerCase() === sessionEmail,
      )
    : undefined
  const myUserId = me?.user_id ?? ""

  async function inviteAction(formData: FormData) {
    "use server"
    const email = String(formData.get("email") ?? "").trim()
    if (!email) {
      redirect(membersHref(homeId, { error: "Email is required" }))
    }
    try {
      await solCore.homes.inviteByEmail(homeId, { email })
      redirect(membersHref(homeId, { notice: `Invitation sent to ${email}` }))
    } catch (error) {
      unstable_rethrow(error)
      redirect(membersHref(homeId, { error: errorMessage(error) }))
    }
  }

  async function cancelInvitationAction(formData: FormData) {
    "use server"
    const invitationId = String(formData.get("invitation_id") ?? "").trim()
    if (!invitationId) {
      redirect(membersHref(homeId, { error: "Missing invitation" }))
    }
    try {
      await solCore.homes.cancelInvitation(homeId, invitationId)
      redirect(membersHref(homeId, { notice: "Invitation cancelled" }))
    } catch (error) {
      unstable_rethrow(error)
      redirect(membersHref(homeId, { error: errorMessage(error) }))
    }
  }

  async function removeMemberAction(formData: FormData) {
    "use server"
    const userId = String(formData.get("user_id") ?? "").trim()
    const isSelf = String(formData.get("is_self") ?? "") === "1"
    if (!userId) {
      redirect(membersHref(homeId, { error: "Missing user" }))
    }
    try {
      await solCore.homes.removeMember(homeId, userId)
      if (isSelf) {
        redirect(`/dashboard?notice=${encodeURIComponent("You left the home")}`)
      }
      redirect(membersHref(homeId, { notice: "Member removed" }))
    } catch (error) {
      unstable_rethrow(error)
      redirect(membersHref(homeId, { error: errorMessage(error) }))
    }
  }

  async function updateRoleAction(formData: FormData) {
    "use server"
    const userId = String(formData.get("user_id") ?? "").trim()
    const role = String(formData.get("role") ?? "").trim() as MemberRole
    if (!userId || (role !== "admin" && role !== "member")) {
      redirect(membersHref(homeId, { error: "Invalid role change" }))
    }
    try {
      await solCore.homes.updateMemberRole(homeId, userId, role)
      redirect(membersHref(homeId, { notice: "Role updated" }))
    } catch (error) {
      unstable_rethrow(error)
      redirect(membersHref(homeId, { error: errorMessage(error) }))
    }
  }

  async function transferOwnershipAction(formData: FormData) {
    "use server"
    const userId = String(formData.get("user_id") ?? "").trim()
    if (!userId) {
      redirect(membersHref(homeId, { error: "Missing user" }))
    }
    try {
      await solCore.homes.transferOwnership(homeId, userId)
      redirect(membersHref(homeId, { notice: "Ownership transferred" }))
    } catch (error) {
      unstable_rethrow(error)
      redirect(membersHref(homeId, { error: errorMessage(error) }))
    }
  }

  return (
    <div className="flex-1 overflow-y-auto p-6 md:p-8">
      <div className="mx-auto w-full max-w-5xl">
        {query.notice ? (
          <p className="mb-4 rounded-2xl border border-tertiary-fixed-dim bg-tertiary-fixed px-4 py-3 text-sm text-on-tertiary-fixed">
            {query.notice}
          </p>
        ) : null}
        {query.error ? (
          <p className="mb-4 rounded-2xl border border-error bg-error-container px-4 py-3 text-sm text-on-error-container">
            {query.error}
          </p>
        ) : null}

        <header className="mb-8 px-2">
          <p className="text-xs font-semibold uppercase tracking-[0.18em] text-primary">
            {home.name}
          </p>
          <h2 className="mt-1 font-display text-[32px] font-bold tracking-[-0.02em] text-on-surface">
            Manage members
          </h2>
          <p className="mt-1 text-sm text-on-surface-variant">
            {canManage
              ? "Invite people by email, change roles, and remove members."
              : "View who has access to this home."}
          </p>
        </header>

        {canManage ? (
          <section className="mb-8 rounded-3xl border border-white/40 bg-surface-container p-6 shadow-[8px_8px_16px_rgba(27,28,25,0.06),-8px_-8px_16px_rgba(255,255,255,0.9)]">
            <div className="mb-4 flex items-center gap-2 text-on-surface">
              <IconUserPlus size={18} className="text-primary" />
              <h3 className="font-display text-lg font-semibold">Invite by email</h3>
            </div>
            <form action={inviteAction} className="flex flex-col gap-3 sm:flex-row sm:items-center">
              <input
                type="email"
                name="email"
                required
                placeholder="person@example.com"
                className="clay-inset flex-1 rounded-xl border border-white/55 px-3 py-2.5 text-sm text-on-surface placeholder:text-on-surface-variant/70"
              />
              <button
                type="submit"
                className="btn-primary rounded-xl px-4 py-2.5 text-sm font-semibold"
              >
                Send invitation
              </button>
            </form>
            <p className="mt-2 text-xs text-on-surface-variant">
              The invitee receives a link valid for 7 days. They can sign up if they don&apos;t have an account yet.
            </p>
          </section>
        ) : null}

        <section className="mb-8 rounded-3xl border border-white/40 bg-surface-container p-6 shadow-[8px_8px_16px_rgba(27,28,25,0.06),-8px_-8px_16px_rgba(255,255,255,0.9)]">
          <div className="mb-4 flex items-baseline justify-between">
            <h3 className="font-display text-lg font-semibold text-on-surface">Members</h3>
            <span className="text-xs text-on-surface-variant">
              {membersPage.data.length} {membersPage.data.length === 1 ? "person" : "people"}
            </span>
          </div>
          <ul className="flex flex-col gap-3">
            {membersPage.data.map((m) => {
              const isSelf = m.user_id === myUserId
              const targetIsOwner = m.role === "owner"
              const canEditRole = isOwner && !targetIsOwner && !isSelf
              const canRemoveOther = canManage && !targetIsOwner && !isSelf
              const canTransfer = isOwner && !targetIsOwner && !isSelf
              const canLeave = isSelf && !targetIsOwner
              const canEditPermissions = canManage && !targetIsOwner && !isSelf
              const hasActions = canEditPermissions || canTransfer || canRemoveOther || canLeave
              const memberLabel = m.user_name?.trim() || m.user_email || "Unnamed user"
              return (
                <li
                  key={m.user_id}
                  className="flex flex-col gap-3 rounded-2xl border border-white/45 bg-surface-container-low p-4 shadow-[inset_2px_2px_5px_rgba(27,28,25,0.05),inset_-2px_-2px_5px_rgba(255,255,255,0.7)] sm:flex-row sm:items-center sm:justify-between sm:gap-6"
                >
                  <div className="flex min-w-0 items-center gap-3 sm:flex-1">
                    <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-full bg-surface-container-high text-sm font-semibold text-on-surface shadow-[inset_2px_2px_4px_rgba(27,28,25,0.05),inset_-2px_-2px_4px_rgba(255,255,255,0.6)]">
                      {initials(m.user_name, m.user_email)}
                    </div>
                    <div className="min-w-0">
                      <p className="truncate text-sm font-semibold text-on-surface">
                        {memberLabel}
                        {isSelf ? (
                          <span className="ml-2 text-xs font-normal text-on-surface-variant">(you)</span>
                        ) : null}
                      </p>
                      {m.user_email ? (
                        <p className="truncate text-xs text-on-surface-variant">{m.user_email}</p>
                      ) : null}
                      <p className="mt-0.5 text-[11px] text-on-surface-variant/80">
                        Joined {formatDate(m.joined_at)}
                      </p>
                    </div>
                  </div>

                  <div className="flex flex-col items-stretch gap-2 sm:items-end">
                    <div className="flex justify-start sm:justify-end">
                      {canEditRole ? (
                        <MemberRoleSelect
                          userId={m.user_id}
                          currentRole={m.role as "admin" | "member"}
                          memberLabel={memberLabel}
                          action={updateRoleAction}
                        />
                      ) : (
                        <span
                          className={`inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-[11px] font-semibold uppercase tracking-wide ${roleBadgeClass(m.role)}`}
                        >
                          {m.role === "owner" ? <IconCrown size={12} /> : null}
                          {m.role}
                        </span>
                      )}
                    </div>

                    {hasActions ? (
                      <div className="flex flex-wrap items-center gap-2 sm:justify-end">
                        {canEditPermissions ? (
                          <Link
                            href={`/dashboard/homes/${homeId}/members/${m.user_id}/permissions`}
                            className="inline-flex items-center gap-1 rounded-lg border border-outline-variant bg-surface px-2.5 py-1.5 text-xs font-semibold text-on-surface transition hover:bg-surface-container"
                          >
                            <IconShieldLock size={12} />
                            Permissions
                          </Link>
                        ) : null}

                        {canTransfer ? (
                          <form action={transferOwnershipAction}>
                            <input type="hidden" name="user_id" value={m.user_id} />
                            <button
                              type="submit"
                              className="inline-flex items-center gap-1 rounded-lg border border-outline-variant bg-surface px-2.5 py-1.5 text-xs font-semibold text-on-surface transition hover:bg-surface-container"
                            >
                              <IconCrown size={12} />
                              Transfer ownership
                            </button>
                          </form>
                        ) : null}

                        {canRemoveOther ? (
                          <form action={removeMemberAction}>
                            <input type="hidden" name="user_id" value={m.user_id} />
                            <button
                              type="submit"
                              className="inline-flex items-center gap-1 rounded-lg border border-error/60 px-2.5 py-1.5 text-xs font-semibold text-error transition hover:bg-error-container"
                            >
                              <IconUserMinus size={12} />
                              Remove
                            </button>
                          </form>
                        ) : null}

                        {canLeave ? (
                          <form action={removeMemberAction}>
                            <input type="hidden" name="user_id" value={m.user_id} />
                            <input type="hidden" name="is_self" value="1" />
                            <button
                              type="submit"
                              className="inline-flex items-center gap-1 rounded-lg border border-error/60 px-2.5 py-1.5 text-xs font-semibold text-error transition hover:bg-error-container"
                            >
                              <IconUserMinus size={12} />
                              Leave home
                            </button>
                          </form>
                        ) : null}
                      </div>
                    ) : null}
                  </div>
                </li>
              )
            })}
            {membersPage.data.length === 0 ? (
              <li className="py-6 text-center text-sm text-on-surface-variant">
                No members yet.
              </li>
            ) : null}
          </ul>
        </section>

        {canManage && invitationsPage.data.length > 0 ? (
          <section className="rounded-3xl border border-white/40 bg-surface-container p-6 shadow-[8px_8px_16px_rgba(27,28,25,0.06),-8px_-8px_16px_rgba(255,255,255,0.9)]">
            <div className="mb-4 flex items-center gap-2 text-on-surface">
              <IconMail size={18} className="text-secondary" />
              <h3 className="font-display text-lg font-semibold">Pending invitations</h3>
            </div>
            <ul className="divide-y divide-outline-variant/45">
              {invitationsPage.data.map((inv) => (
                <li
                  key={inv.id}
                  className="flex flex-col gap-3 py-3 sm:flex-row sm:items-center sm:gap-4"
                >
                  <div className="flex-1 min-w-0">
                    <p className="truncate text-sm font-semibold text-on-surface">
                      {inv.invitee_email}
                    </p>
                    <p className="text-[11px] text-on-surface-variant">
                      Expires {formatDate(inv.expires_at)}
                      {inv.invitee_is_user ? " · existing user" : " · new user"}
                    </p>
                  </div>
                  <form action={cancelInvitationAction} className="sm:ml-auto">
                    <input type="hidden" name="invitation_id" value={inv.id} />
                    <button
                      type="submit"
                      className="rounded-lg border border-outline-variant px-2.5 py-1.5 text-xs font-semibold text-on-surface transition hover:bg-surface"
                    >
                      Cancel
                    </button>
                  </form>
                </li>
              ))}
            </ul>
          </section>
        ) : null}
      </div>
    </div>
  )
}
