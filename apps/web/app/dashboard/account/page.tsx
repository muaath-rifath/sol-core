import { auth } from "@/auth"
import { type CursorResponse, type Home, solCore } from "@/lib/sol-core"
import { zitadelAccount, type AccountProfile } from "@/lib/zitadel-account"
import HomeSwitcher from "@/components/HomeSwitcher"
import UserMenu from "@/components/UserMenu"
import Link from "next/link"
import { redirect, unstable_rethrow } from "next/navigation"
import { federatedLogout } from "@/app/actions"

type PageSearchParams = Promise<{ notice?: string; error?: string; section?: string }>

function errorMessage(e: unknown) {
  return e instanceof Error ? e.message : "Something went wrong"
}

// ── Server actions ────────────────────────────────────────────────────────────

async function updateProfileAction(formData: FormData) {
  "use server"
  const session = await auth()
  if (!session?.accessToken) redirect("/")
  try {
    await zitadelAccount.updateProfile(session.accessToken, {
      firstName: String(formData.get("firstName") ?? "").trim(),
      lastName: String(formData.get("lastName") ?? "").trim(),
    })
    redirect("/dashboard/account?notice=profile-updated&section=personal")
  } catch (e) {
    unstable_rethrow(e)
    redirect(`/dashboard/account?error=${encodeURIComponent(errorMessage(e))}&section=personal`)
  }
}

async function changePasswordAction(formData: FormData) {
  "use server"
  const session = await auth()
  if (!session?.accessToken) redirect("/")
  const currentPassword = String(formData.get("currentPassword") ?? "")
  const newPassword = String(formData.get("newPassword") ?? "")
  const confirmation = String(formData.get("confirmation") ?? "")
  if (!newPassword || newPassword !== confirmation) {
    redirect("/dashboard/account?error=Passwords+do+not+match&section=security")
  }
  try {
    await zitadelAccount.changePassword(session.accessToken, { currentPassword, newPassword })
    redirect("/dashboard/account?notice=password-updated&section=security")
  } catch (e) {
    unstable_rethrow(e)
    redirect(`/dashboard/account?error=${encodeURIComponent(errorMessage(e))}&section=security`)
  }
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function decodeNotice(code: string | undefined): string | null {
  switch (code) {
    case "profile-updated": return "Profile updated."
    case "password-updated": return "Password updated successfully."
    default: return null
  }
}

// ── Page ──────────────────────────────────────────────────────────────────────

export default async function AccountPage({ searchParams }: { searchParams: PageSearchParams }) {
  const session = await auth()
  if (!session?.accessToken) redirect("/")

  const { notice: noticeCode, error, section } = await searchParams
  const notice = decodeNotice(noticeCode)

  const [homesResult, profileResult] = await Promise.allSettled([
    solCore.homes.list({ limit: 50 }),
    zitadelAccount.getProfile(session.accessToken),
  ])

  const allHomes: CursorResponse<Home> =
    homesResult.status === "fulfilled" ? homesResult.value : { data: [], next_cursor: null, has_more: false }
  const profile: AccountProfile | null =
    profileResult.status === "fulfilled" ? profileResult.value : null

  return (
    <div className="bg-clay-canvas relative min-h-screen text-on-surface">
      <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_20%_0%,rgba(255,255,255,0.65),transparent_40%)]" />

      <header className="clay-glass sticky top-0 z-30 border-b border-white/45">
        <div className="mx-auto flex w-full max-w-7xl items-center justify-between gap-4 px-4 py-3 sm:px-6">
          <HomeSwitcher homes={allHomes.data} activeHomeId={undefined} />
          <UserMenu
            name={session.user?.name}
            email={session.user?.email}
            image={session.user?.image}
            accountSettingsUrl="/dashboard/account"
            signOutAction={federatedLogout}
          />
        </div>
      </header>

      <div className="relative mx-auto w-full max-w-3xl px-4 py-8 sm:px-6">
        <div className="mb-6 flex items-center gap-3">
          <Link
            href="/dashboard"
            className="rounded-lg p-1.5 text-on-surface-variant transition hover:bg-surface-container hover:text-on-surface"
          >
            <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M15 19l-7-7 7-7" />
            </svg>
          </Link>
          <div>
            <h1 className="font-display text-xl font-semibold tracking-tight text-on-surface">Account</h1>
            <p className="text-xs text-on-surface-variant">Manage your personal info and security</p>
          </div>
        </div>

        {/* Feedback banners */}
        {(section === "personal" || !section) && notice ? (
          <p className="mb-4 rounded-2xl border border-tertiary-fixed-dim bg-tertiary-fixed px-4 py-3 text-sm text-on-tertiary-fixed">{notice}</p>
        ) : null}
        {(section === "personal" || !section) && error ? (
          <p className="mb-4 rounded-2xl border border-error bg-error-container px-4 py-3 text-sm text-on-error-container">{decodeURIComponent(error)}</p>
        ) : null}

        <div className="space-y-4">

          {/* ── Personal Info ──────────────────────────────────────────── */}
          <section className="clay-raised rounded-3xl p-6">
            <h2 className="font-display text-base font-semibold text-on-surface">Personal info</h2>
            <p className="mt-0.5 text-xs text-on-surface-variant">Manage your basic information</p>

            {profile ? (
              <form action={updateProfileAction} className="mt-5 space-y-4">
                <div className="grid gap-4 sm:grid-cols-2">
                  <div>
                    <label className="mb-1 block text-xs font-medium text-on-surface-variant">Username</label>
                    <p className="clay-inset rounded-xl px-3 py-2.5 text-sm text-on-surface-variant">{profile.username}</p>
                  </div>
                  <div>
                    <label className="mb-1 block text-xs font-medium text-on-surface-variant">Email</label>
                    <p className="clay-inset rounded-xl px-3 py-2.5 text-sm text-on-surface-variant">{profile.email}</p>
                  </div>
                  <div>
                    <label htmlFor="firstName" className="mb-1 block text-xs font-medium text-on-surface-variant">
                      First name <span className="text-error">*</span>
                    </label>
                    <input
                      id="firstName"
                      name="firstName"
                      type="text"
                      required
                      defaultValue={profile.firstName}
                      className="clay-inset w-full rounded-xl border border-white/50 px-3 py-2.5 text-sm text-on-surface outline-none ring-2 ring-transparent transition focus:ring-primary/35"
                    />
                  </div>
                  <div>
                    <label htmlFor="lastName" className="mb-1 block text-xs font-medium text-on-surface-variant">
                      Last name <span className="text-error">*</span>
                    </label>
                    <input
                      id="lastName"
                      name="lastName"
                      type="text"
                      required
                      defaultValue={profile.lastName}
                      className="clay-inset w-full rounded-xl border border-white/50 px-3 py-2.5 text-sm text-on-surface outline-none ring-2 ring-transparent transition focus:ring-primary/35"
                    />
                  </div>
                </div>
                <div className="flex justify-end">
                  <button
                    type="submit"
                    className="btn-primary px-5 py-2 text-sm font-semibold"
                  >
                    Save
                  </button>
                </div>
              </form>
            ) : (
              <p className="mt-4 text-sm text-on-surface-variant">Could not load profile.</p>
            )}
          </section>

          {/* ── Account Security ───────────────────────────────────────── */}
          <section className="clay-raised rounded-3xl p-6">
            <h2 className="font-display text-base font-semibold text-on-surface">Account security</h2>
            <p className="mt-0.5 text-xs text-on-surface-variant">Configure ways to sign in</p>

            {section === "security" && notice ? (
              <p className="mt-3 rounded-2xl border border-tertiary-fixed-dim bg-tertiary-fixed px-4 py-3 text-sm text-on-tertiary-fixed">{notice}</p>
            ) : null}
            {section === "security" && error ? (
              <p className="mt-3 rounded-2xl border border-error bg-error-container px-4 py-3 text-sm text-on-error-container">{decodeURIComponent(error)}</p>
            ) : null}

            <div className="mt-5 space-y-4">
              {/* Password */}
              <div className="clay-inset rounded-2xl p-4">
                <div>
                  <p className="text-sm font-medium text-on-surface">Password</p>
                  <p className="mt-0.5 text-xs text-on-surface-variant">Change your sign-in password</p>
                </div>

                <form action={changePasswordAction} className="mt-4 space-y-3">
                  <div>
                    <label className="mb-1 block text-xs font-medium text-on-surface-variant">Current password</label>
                    <input
                      name="currentPassword"
                      type="password"
                      autoComplete="current-password"
                      className="w-full rounded-xl border border-white/50 bg-surface-container-low px-3 py-2.5 text-sm text-on-surface outline-none ring-2 ring-transparent transition focus:ring-primary/35"
                    />
                  </div>
                  <div className="grid gap-3 sm:grid-cols-2">
                    <div>
                      <label className="mb-1 block text-xs font-medium text-on-surface-variant">New password</label>
                      <input
                        name="newPassword"
                        type="password"
                        autoComplete="new-password"
                        required
                        className="w-full rounded-xl border border-white/50 bg-surface-container-low px-3 py-2.5 text-sm text-on-surface outline-none ring-2 ring-transparent transition focus:ring-primary/35"
                      />
                    </div>
                    <div>
                      <label className="mb-1 block text-xs font-medium text-on-surface-variant">Confirm password</label>
                      <input
                        name="confirmation"
                        type="password"
                        autoComplete="new-password"
                        required
                        className="w-full rounded-xl border border-white/50 bg-surface-container-low px-3 py-2.5 text-sm text-on-surface outline-none ring-2 ring-transparent transition focus:ring-primary/35"
                      />
                    </div>
                  </div>
                  <div className="flex justify-end">
                    <button
                      type="submit"
                      className="btn-primary px-4 py-2 text-sm font-semibold"
                    >
                      Update password
                    </button>
                  </div>
                </form>
              </div>

              {/* MFA / advanced security → Zitadel */}
              <div className="clay-inset rounded-2xl p-4">
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <p className="text-sm font-medium text-on-surface">Two-factor authentication &amp; more</p>
                    <p className="mt-0.5 text-xs text-on-surface-variant">
                      Manage passkeys, authenticator apps, and active sessions in your Zitadel account.
                    </p>
                  </div>
                  <a
                    href={`${process.env.AUTH_ZITADEL_ISSUER}/ui/account`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="btn-outline shrink-0 rounded-full px-3 py-1.5 text-xs font-semibold"
                  >
                    Open →
                  </a>
                </div>
              </div>
            </div>
          </section>

        </div>
      </div>
    </div>
  )
}
