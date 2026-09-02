import { auth, authProvider, signIn } from "@/auth"
import { redirect } from "next/navigation"

export default async function HomePage({
  searchParams,
}: {
  searchParams: Promise<{ callbackUrl?: string }>
}) {
  const session = await auth()
  const { callbackUrl } = await searchParams

  if (session && session.error !== "RefreshAccessTokenError") {
    redirect(callbackUrl ?? "/dashboard")
  }

  return (
    <div className="bg-clay-canvas flex min-h-screen items-center justify-center px-4 py-8">
      <main className="w-full max-w-5xl overflow-hidden rounded-[2.2rem] border border-white/55 bg-surface-container-low shadow-[12px_12px_28px_rgba(87,66,62,0.14),-12px_-12px_28px_rgba(255,255,255,0.92)]">
        <div className="grid gap-0 md:grid-cols-[1.2fr_1fr]">
          <section className="space-y-6 bg-[linear-gradient(150deg,#b34a37_0%,#a53b29_45%,#7f281b_100%)] px-8 py-10 text-on-primary sm:px-10 sm:py-12">
            <p className="text-xs font-semibold uppercase tracking-[0.2em] text-primary-fixed">SOL Next</p>
            <h1 className="font-display text-4xl font-bold leading-tight tracking-tight">Smart homes, shaped for comfort.</h1>
            <p className="max-w-md text-sm text-primary-fixed sm:text-base">
              Create homes when you need them, invite the right people by email, and keep membership
              organized with reliable cursor-based lists.
            </p>
            <div className="grid max-w-md grid-cols-2 gap-3 text-xs">
              <div className="rounded-2xl border border-white/25 bg-white/10 px-3 py-2">
                <p className="text-primary-fixed">Homes</p>
                <p className="mt-1 text-base font-semibold text-on-primary">Managed</p>
              </div>
              <div className="rounded-2xl border border-white/25 bg-white/10 px-3 py-2">
                <p className="text-primary-fixed">Access</p>
                <p className="mt-1 text-base font-semibold text-on-primary">Role-based</p>
              </div>
            </div>
          </section>

          <section className="flex flex-col justify-between bg-surface-container-low px-8 py-10 sm:px-10 sm:py-12">
            <div>
              <h2 className="font-display text-2xl font-semibold tracking-tight text-on-surface">Sign in to continue</h2>
              <p className="mt-2 text-sm text-on-surface-variant">
                Use your organization account to access homes, invitations, and devices.
              </p>
            </div>

            <form
              action={async () => {
                "use server"
                await signIn(authProvider, { redirectTo: callbackUrl ?? "/dashboard" })
              }}
              className="mt-8"
            >
              <button
                type="submit"
                className="btn-primary w-full px-5 py-3 text-sm font-semibold"
              >
                Log in / Sign up
              </button>
            </form>
          </section>
        </div>
      </main>
    </div>
  )
}
