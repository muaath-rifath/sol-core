import { auth } from "@/auth"
import { type Room, type RoomDevice, solCore } from "@/lib/sol-core"
import {
  IconArmchair,
  IconBed,
  IconChefHat,
  IconDeviceDesktop,
} from "@tabler/icons-react"
import Link from "next/link"
import { redirect, unstable_rethrow } from "next/navigation"
import type { ForwardRefExoticComponent, RefAttributes } from "react"
import type { IconProps } from "@tabler/icons-react"
import { SolChatWidget } from "./SolChatWidget"

export const dynamic = "force-dynamic"

type HomePageSearchParams = Promise<{
  notice?: string
  error?: string
}>

function homeHref(homeID: string, params: Record<string, string | undefined>) {
  const query = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (!value) continue
    query.set(key, value)
  }
  const encoded = query.toString()
  return encoded ? `/dashboard/homes/${homeID}?${encoded}` : `/dashboard/homes/${homeID}`
}

function roomHref(homeID: string, roomID: string) {
  return `/dashboard/homes/${homeID}/rooms/${roomID}`
}

function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message
  return "Something went wrong"
}

function coerceBool(value: unknown): boolean {
  if (typeof value === "boolean") return value
  if (typeof value === "number") return value !== 0
  if (typeof value === "string") return value === "true" || value === "1" || value === "on"
  return false
}

type TablerIcon = ForwardRefExoticComponent<IconProps & RefAttributes<SVGSVGElement>>

function roomIcon(name: string): TablerIcon {
  const lower = name.toLowerCase()
  if (lower.includes("living")) return IconArmchair
  if (lower.includes("kitchen")) return IconChefHat
  if (lower.includes("bed")) return IconBed
  if (lower.includes("bath")) return IconBed
  if (lower.includes("garage")) return IconDeviceDesktop
  if (lower.includes("office")) return IconDeviceDesktop
  return IconArmchair
}

export default async function HomeDetailPage({
  params,
  searchParams,
}: {
  params: Promise<{ homeId: string }>
  searchParams: HomePageSearchParams
}) {
  const session = await auth()
  if (!session) redirect("/")

  const { homeId } = await params
  const query = await searchParams

  const rooms = (await solCore.rooms.listAll(homeId).catch(() => [])) as Room[]
  const roomDevices = new Map<string, RoomDevice[]>()

  await Promise.all(
    rooms.map(async (room) => {
      try {
        const devices = await solCore.rooms.devices.listAll(homeId, room.id)
        roomDevices.set(room.id, devices)
      } catch {
        roomDevices.set(room.id, [])
      }
    }),
  )

  const primaryRoom = rooms[0]
  const devices = primaryRoom ? (roomDevices.get(primaryRoom.id) ?? []) : []

  const secondaryRooms = rooms.filter((room) => room.id !== primaryRoom?.id).slice(0, 2)

  const primaryRoomOnCount = devices.filter((device) => coerceBool(device.state?.isOn)).length
  const primaryRoomTurnOn = primaryRoomOnCount === 0

  async function togglePrimaryRoomAction(formData: FormData) {
    "use server"
    const roomID = String(formData.get("room_id") ?? "").trim()
    const turnOn = String(formData.get("turn_on") ?? "") === "1"

    if (!roomID) redirect(homeHref(homeId, { error: "Missing room" }))

    const targetRoom = rooms.find((room) => room.id === roomID)
    if (!targetRoom) redirect(homeHref(homeId, { error: "Room not found" }))

    const targetDevices = roomDevices.get(targetRoom!.id) ?? []
    const controllable = targetDevices.filter((device) =>
      ["light", "switch", "fan"].includes(device.type.toLowerCase()),
    )

    if (controllable.length === 0) {
      redirect(homeHref(homeId, { error: "No controllable devices in this room" }))
    }

    try {
      await Promise.all(
        controllable.map((device) =>
          solCore.rooms.devices.command(homeId, targetRoom!.id, device.id, {
            action: "set_relay",
            params: { power: turnOn, channel: 0 },
          }),
        ),
      )
      redirect(homeHref(homeId, { notice: turnOn ? "Room turned on" : "Room turned off" }))
    } catch (error) {
      unstable_rethrow(error)
      redirect(homeHref(homeId, { error: errorMessage(error) }))
    }
  }

  const firstName = session.user?.name?.split(" ")[0]

  return (
    <div className="flex-1 overflow-y-auto p-6 md:p-8">
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

      <div className="mb-8 px-2">
        <h2 className="font-display text-[40px] font-bold tracking-[-0.02em] text-on-surface">
          {firstName ? `Good morning, ${firstName}.` : "Good morning."}
        </h2>
      </div>

      <div className="grid grid-cols-1 gap-6 xl:grid-cols-2">
        {/* Left: room cards */}
        <div className="flex flex-col gap-6">
          {/* Primary room card */}
          <article className="group relative overflow-hidden rounded-xl border border-white/40 bg-surface-container p-6 shadow-[12px_12px_24px_rgba(27,28,25,0.06),-12px_-12px_24px_rgba(255,255,255,0.9),inset_2px_2px_4px_rgba(255,255,255,0.8),inset_-2px_-2px_4px_rgba(27,28,25,0.02)]">
            <div className="pointer-events-none absolute -right-20 -top-20 h-64 w-64 rounded-full bg-primary-container/30 blur-3xl transition-transform duration-700 ease-out group-hover:scale-110" />
            <div className="relative z-10 flex items-start justify-between">
              <div>
                {primaryRoom ? (
                  <>
                    <div className="mb-1 flex items-center gap-2 text-primary">
                      {(() => {
                        const Icon = roomIcon(primaryRoom.name)
                        return <Icon size={18} />
                      })()}
                      <span className="text-[13px] font-semibold uppercase tracking-[0.05em]">
                        {primaryRoom.name}
                      </span>
                    </div>
                    <p className="text-sm text-on-surface-variant">
                      {devices.length} device{devices.length !== 1 ? "s" : ""}
                      {primaryRoomOnCount > 0 ? ` · ${primaryRoomOnCount} on` : ""}
                    </p>
                  </>
                ) : (
                  <p className="text-sm text-on-surface-variant">No rooms yet</p>
                )}
              </div>
              {primaryRoom ? (
                <form action={togglePrimaryRoomAction}>
                  <input type="hidden" name="room_id" value={primaryRoom.id} />
                  <input type="hidden" name="turn_on" value={primaryRoomTurnOn ? "1" : "0"} />
                  <button
                    className="flex h-8 w-14 items-center rounded-full bg-surface-variant p-1 shadow-[inset_4px_4px_8px_rgba(27,28,25,0.1),inset_-4px_-4px_8px_rgba(255,255,255,0.7)]"
                    type="submit"
                  >
                    <span
                      className={`h-6 w-6 rounded-full bg-primary shadow-[2px_2px_4px_rgba(0,0,0,0.2),-2px_-2px_4px_rgba(255,255,255,0.4)] ${
                        primaryRoomTurnOn ? "" : "ml-auto"
                      }`}
                    />
                  </button>
                </form>
              ) : null}
            </div>

            {devices.length > 0 ? (
              <div className="relative z-10 mt-4 flex flex-col gap-2">
                {devices.slice(0, 5).map((device) => (
                  <Link
                    key={device.id}
                    href={roomHref(homeId, primaryRoom!.id)}
                    className="flex items-center justify-between rounded-lg bg-surface-container-high px-3 py-2 shadow-[inset_1px_1px_2px_rgba(255,255,255,0.6),inset_-1px_-1px_2px_rgba(27,28,25,0.04)]"
                  >
                    <span className="text-[13px] font-semibold text-on-surface">{device.name}</span>
                    <span
                      className={`text-xs ${
                        coerceBool(device.state?.isOn) ? "text-primary" : "text-on-surface-variant"
                      }`}
                    >
                      {coerceBool(device.state?.isOn) ? "On" : "Off"}
                    </span>
                  </Link>
                ))}
              </div>
            ) : null}
          </article>

          {/* Room mini-cards */}
          {secondaryRooms.length > 0 ? (
            <div className="grid grid-cols-2 gap-6">
              {secondaryRooms.map((room) => {
                const miniDevices = roomDevices.get(room.id) ?? []
                const isOn = miniDevices.some((device) => coerceBool(device.state?.isOn))
                const MiniRoomIcon = roomIcon(room.name)
                return (
                  <Link
                    key={room.id}
                    href={roomHref(homeId, room.id)}
                    className="flex flex-col justify-between rounded-xl border border-white/40 bg-surface-container p-5 shadow-[8px_8px_16px_rgba(27,28,25,0.06),-8px_-8px_16px_rgba(255,255,255,0.9),inset_1px_1px_2px_rgba(255,255,255,0.8),inset_-1px_-1px_2px_rgba(27,28,25,0.03)]"
                  >
                    <div className="mb-4 flex items-center justify-between">
                      <div className="flex h-10 w-10 items-center justify-center rounded-full bg-surface shadow-[4px_4px_8px_rgba(27,28,25,0.05),-4px_-4px_8px_rgba(255,255,255,0.8)]">
                        <MiniRoomIcon size={16} className="text-on-surface-variant" />
                      </div>
                      <div
                        className={`flex h-6 w-10 items-center rounded-full p-1 shadow-[inset_2px_2px_4px_rgba(27,28,25,0.1),inset_-2px_-2px_4px_rgba(255,255,255,0.6)] ${
                          isOn ? "justify-end bg-primary-container" : "bg-surface-variant"
                        }`}
                      >
                        <div
                          className={`h-4 w-4 rounded-full shadow-sm ${
                            isOn ? "bg-primary" : "bg-surface-container-highest"
                          }`}
                        />
                      </div>
                    </div>
                    <div>
                      <h4 className="text-base font-semibold text-on-surface">{room.name}</h4>
                      <p className={`text-sm ${isOn ? "text-primary" : "text-on-surface-variant"}`}>
                        {isOn ? "Lights On" : "Off"}
                      </p>
                    </div>
                  </Link>
                )
              })}
            </div>
          ) : null}
        </div>

        {/* Right: Sol chat */}
        <SolChatWidget homeId={homeId} />
      </div>
    </div>
  )
}
