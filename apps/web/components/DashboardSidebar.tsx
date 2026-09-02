import CreateRoomPopover from "@/components/CreateRoomPopover"
import DashboardSidebarNav from "@/components/DashboardSidebarNav"
import HomeSwitcher from "@/components/HomeSwitcher"
import { type CursorResponse, type Home, type Room, solCore } from "@/lib/sol-core"
import { redirect, unstable_rethrow } from "next/navigation"

function homeHref(homeID: string, params: Record<string, string | undefined>) {
  const query = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (!value) continue
    query.set(key, value)
  }
  const encoded = query.toString()
  return encoded ? `/dashboard/homes/${homeID}?${encoded}` : `/dashboard/homes/${homeID}`
}

interface DashboardSidebarProps {
  homeId: string
}

export default async function DashboardSidebar({ homeId }: DashboardSidebarProps) {
  let allHomes: CursorResponse<Home> = { data: [], next_cursor: null, has_more: false }
  try {
    allHomes = await solCore.homes.list({ limit: 50 })
  } catch {
    // non-fatal
  }

  const currentHome = allHomes.data.find((home) => home.id === homeId)
  const rooms = (await solCore.rooms.listAll(homeId).catch(() => [])) as Room[]
  const navRooms = rooms.slice(0, 5)

  async function createRoomAction(formData: FormData) {
    "use server"
    const name = String(formData.get("name") ?? "").trim()
    const floorRaw = String(formData.get("floor") ?? "").trim()
    const floor = floorRaw ? Number(floorRaw) : undefined

    if (!name) {
      redirect(homeHref(homeId, { error: "Room name is required" }))
    }
    if (floorRaw && Number.isNaN(floor)) {
      redirect(homeHref(homeId, { error: "Floor must be a number" }))
    }

    try {
      await solCore.rooms.create(homeId, { name, floor })
      redirect(homeHref(homeId, { notice: "Room created" }))
    } catch (error) {
      unstable_rethrow(error)
    }
  }

  return (
    <nav className="fixed left-4 top-[96px] bottom-4 z-40 hidden w-72 rounded-[40px] border border-white/20 bg-stone-100 px-6 py-7 shadow-[20px_0_40px_rgba(0,0,0,0.05)] md:flex md:flex-col">
      <div className="mb-8 px-1">
        <div className="mb-3 px-2">
          <p className="text-[11px] font-semibold uppercase tracking-[0.08em] text-stone-500">Home</p>
          <h2 className="mt-1 truncate text-xl font-bold tracking-tight text-stone-800">
            {currentHome?.name ?? "Select home"}
          </h2>
        </div>
        <HomeSwitcher homes={allHomes.data} activeHomeId={homeId} />
      </div>

      <DashboardSidebarNav
        homeId={homeId}
        navRooms={navRooms.map((room) => ({ id: room.id, name: room.name }))}
      />

      <div className="mt-auto">
        <CreateRoomPopover createRoomAction={createRoomAction} />
      </div>
    </nav>
  )
}
