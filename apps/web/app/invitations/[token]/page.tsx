import { auth, authProvider, signIn } from "@/auth"
import { solCore } from "@/lib/sol-core"
import Link from "next/link"
import { redirect, unstable_rethrow } from "next/navigation"

type InvitationPageSearchParams = Promise<{
  notice?: string
  error?: string
  autoAccept?: string
}>

function invitationHref(token: string, params: Record<string, string | undefined>) {
  const query = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (!value) {
      continue
    }
    query.set(key, value)
  }
  const encoded = query.toString()
  return encoded ? `/invitations/${token}?${encoded}` : `/invitations/${token}`
}

function errorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message
  }
  return "Something went wrong"
}

export default async function InvitationPage({
  params,
  searchParams,
}: {
  params: Promise<{ token: string }>
  searchParams: InvitationPageSearchParams
}) {
  const { token } = await params
  const query = await searchParams
  const session = await auth()

  let invitationError: string | null = null
  let invitation = null as Awaited<ReturnType<typeof solCore.invitations.getPublic>> | null

  try {
    invitation = await solCore.invitations.getPublic(token)
  } catch (error) {
    invitationError = errorMessage(error)
  }

  if (session && invitation && invitation.status === "pending" && query.autoAccept === "1") {
    try {
      await solCore.invitations.accept(token)
    } catch (error) {
      redirect(invitationHref(token, { error: errorMessage(error) }))
    }
    redirect("/dashboard?notice=invite-accepted")
  }

  async function acceptAction() {
    "use server"
    if (!(await auth())) {
      await signIn(authProvider, { redirectTo: `/invitations/${token}` })
      return
    }

    try {
      await solCore.invitations.accept(token)
      redirect("/dashboard?notice=invite-accepted")
    } catch (error) {
      unstable_rethrow(error)
      redirect(invitationHref(token, { error: errorMessage(error) }))
    }
  }

  async function declineAction() {
    "use server"
    try {
      await solCore.invitations.declinePublic(token)
      redirect(invitationHref(token, { notice: "Invitation declined" }))
    } catch (error) {
      unstable_rethrow(error)
      redirect(invitationHref(token, { error: errorMessage(error) }))
    }
  }

  async function signInAction() {
    "use server"
    await signIn(authProvider, { redirectTo: `/invitations/${token}` })
  }

  async function signUpAction() {
    "use server"
    await signIn(authProvider, { redirectTo: invitationHref(token, { autoAccept: "1" }) })
  }

  return (
    <div className="bg-clay-canvas min-h-screen px-4 py-10">
      <div className="mx-auto w-full max-w-2xl rounded-[2rem] border border-white/55 bg-surface-container-low p-8 shadow-[12px_12px_28px_rgba(87,66,62,0.14),-12px_-12px_28px_rgba(255,255,255,0.92)]">
        <p className="text-xs font-semibold uppercase tracking-[0.18em] text-primary">SOL Next</p>

        {query.notice ? (
            <p className="mt-4 rounded-xl border border-tertiary-fixed-dim bg-tertiary-fixed px-4 py-3 text-sm text-on-tertiary-fixed">
              {query.notice}
            </p>
          ) : null}

        {query.error ? (
            <p className="mt-4 rounded-xl border border-error bg-error-container px-4 py-3 text-sm text-on-error-container">
              {query.error}
            </p>
          ) : null}

        {invitationError || !invitation ? (
          <div className="mt-4 rounded-2xl border border-error bg-error-container p-6">
            <h1 className="font-display text-2xl font-semibold tracking-tight text-on-error-container">Invitation unavailable</h1>
            <p className="mt-2 text-sm text-on-error-container">
              {invitationError ?? "This invitation was not found or has already expired."}
            </p>
            <div className="mt-4 flex flex-wrap gap-2">
              <Link
                href="/"
                className="btn-outline px-4 py-2 text-sm font-semibold"
              >
                Go to sign-in
              </Link>
              {session ? (
                <Link
                  href="/dashboard"
                  className="btn-outline px-4 py-2 text-sm font-semibold"
                >
                  Open dashboard
                </Link>
              ) : null}
            </div>
          </div>
        ) : (
          <>
            <h1 className="font-display mt-4 text-3xl font-semibold tracking-tight text-on-surface">
              You were invited to join <span className="text-primary">{invitation.home_name}</span>
            </h1>
            <p className="mt-3 text-sm text-on-surface-variant">
              <strong>{invitation.inviter_name}</strong> invited <strong>{invitation.invitee_email}</strong>.
              This invitation expires on {new Date(invitation.expires_at).toLocaleString()}.
            </p>

            {invitation.status === "pending" ? (
              session ? (
                <div className="mt-6 flex flex-wrap gap-3">
                  <form action={acceptAction}>
                    <button
                      type="submit"
                      className="btn-primary rounded-full px-5 py-2.5 text-sm font-semibold"
                    >
                      Accept invitation
                    </button>
                  </form>
                  <form action={declineAction}>
                    <button
                      type="submit"
                      className="rounded-full border border-error px-5 py-2.5 text-sm font-semibold text-error transition hover:bg-error-container"
                    >
                      Decline invitation
                    </button>
                  </form>
                </div>
              ) : (
                <div className="mt-6 rounded-2xl border border-secondary-fixed-dim bg-secondary-fixed p-4">
                  {invitation.invitee_is_user ? (
                    <>
                      <p className="text-sm text-on-secondary-fixed">
                        Sign in with the invited account email to accept this invitation.
                      </p>
                      <form action={signInAction} className="mt-3">
                        <button
                          type="submit"
                          className="rounded-full bg-secondary px-5 py-2.5 text-sm font-semibold text-on-secondary transition hover:brightness-105"
                        >
                          Sign in to continue
                        </button>
                      </form>
                    </>
                  ) : (
                    <>
                      <p className="text-sm text-on-secondary-fixed">
                        Create an account with the invited email. After sign up, we will
                        automatically finish accepting this invitation.
                      </p>
                      <form action={signUpAction} className="mt-3">
                        <button
                          type="submit"
                          className="rounded-full bg-secondary px-5 py-2.5 text-sm font-semibold text-on-secondary transition hover:brightness-105"
                        >
                          Sign up to continue
                        </button>
                      </form>
                      <form action={signInAction} className="mt-2">
                        <button
                          type="submit"
                          className="btn-outline px-5 py-2.5 text-sm font-semibold"
                        >
                          I already have an account
                        </button>
                      </form>
                    </>
                  )}
                  <form action={declineAction} className="mt-2">
                    <button
                      type="submit"
                      className="btn-outline px-5 py-2.5 text-sm font-semibold"
                    >
                      Decline without account
                    </button>
                  </form>
                </div>
              )
            ) : (
              <div className="mt-6 rounded-2xl border border-outline-variant bg-surface-container p-4">
                <p className="text-sm text-on-surface">
                  {invitation.status === "accepted"
                    ? "This invitation was already accepted."
                    : invitation.status === "declined"
                      ? "This invitation was declined."
                      : "This invitation has expired."}
                </p>
                <div className="mt-3 flex flex-wrap gap-2">
                  <Link
                    href="/dashboard"
                    className="btn-primary rounded-full px-4 py-2 text-sm font-semibold"
                  >
                    Open dashboard
                  </Link>
                  <Link
                    href="/"
                    className="btn-outline px-4 py-2 text-sm font-semibold"
                  >
                    Go to home
                  </Link>
                </div>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  )
}
