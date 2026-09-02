import { solCore } from "@/lib/sol-core"
import ActivityFeed from "@/components/ActivityFeed"
import Link from "next/link"
import { redirect } from "next/navigation"
import { IconArrowLeft } from "@tabler/icons-react"

export const dynamic = "force-dynamic"

export default async function ActivityPage({
  params,
}: {
  params: Promise<{ homeId: string; roomId: string }>
}) {
  const { homeId, roomId } = await params

  const roomResult = await solCore.rooms.get(homeId, roomId).catch(() => null)
  if (!roomResult) {
    redirect(`/dashboard/homes/${homeId}?error=Room+not+found`)
  }
  const room = roomResult

  const firstPage = await solCore.rooms
    .activity(homeId, roomId, { limit: 20 })
    .catch(() => ({ data: [], has_more: false, next_cursor: null }))

  return (
    <div className="flex-1 overflow-y-auto px-6 pb-6 pt-0 md:px-8 md:pb-8 md:pt-0">
      <div className="-mt-0.5 mx-auto w-full max-w-7xl space-y-6">
        <header className="rounded-[2rem] border border-white/60 bg-surface-container-low p-6 shadow-[10px_10px_24px_rgba(87,66,62,0.12),-10px_-10px_24px_rgba(255,255,255,0.92)]">
          <div className="flex items-center gap-4">
            <Link
              href={`/dashboard/homes/${homeId}/rooms/${roomId}`}
              className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full border border-white/45 bg-surface text-on-surface shadow-[2px_2px_6px_rgba(87,66,62,0.07),-2px_-2px_6px_rgba(255,255,255,0.8)] hover:bg-surface-container transition-colors"
            >
              <IconArrowLeft size={16} />
            </Link>
            <div>
              <p className="text-xs font-semibold uppercase tracking-[0.18em] text-primary">{room.name}</p>
              <h1 className="font-display mt-1 text-3xl font-semibold tracking-tight text-on-surface">Activity Log</h1>
            </div>
          </div>
        </header>

        <section className="w-full min-w-0 overflow-hidden rounded-3xl border border-white/45 bg-surface-container p-5 shadow-[8px_8px_16px_rgba(87,66,62,0.05),-8px_-8px_16px_rgba(255,255,255,0.8)]">
          <ActivityFeed
            homeId={homeId}
            roomId={roomId}
            initialData={firstPage.data}
            initialNextCursor={firstPage.next_cursor}
            initialHasMore={firstPage.has_more}
          />
        </section>
      </div>
    </div>
  )
}
