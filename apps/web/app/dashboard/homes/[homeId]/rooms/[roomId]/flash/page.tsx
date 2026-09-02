import Flasher from "@/components/Flasher"
import { solCore } from "@/lib/sol-core"
import Link from "next/link"
import { redirect } from "next/navigation"
import FirmwareBuildModal from "@/components/FirmwareBuildModal"

export const dynamic = "force-dynamic"

export default async function FlashPage({
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

  const [devices, firmwareVersions, appliancesRes] = await Promise.all([
    solCore.rooms.devices.listAll(homeId, roomId).catch(() => []),
    solCore.firmware.list().catch(() => []),
    solCore.appliances.listByRoom(homeId, roomId).catch(() => ({ data: [] })),
  ])
  const appliances = appliancesRes?.data ?? []

  const flashDevices = devices.map((d) => ({ id: d.id, name: d.name, room_id: d.room_id, metadata: d.metadata }))
  const flashFirmware = firmwareVersions.map((f) => ({
    id: f.id,
    template_id: f.template_id,
    version: f.version,
  }))

  return (
    <div className="flex-1 overflow-y-auto p-6 md:p-8">
      <div className="mx-auto w-full max-w-5xl space-y-5">
        <header className="rounded-[2rem] border border-white/60 bg-surface-container-low p-6 shadow-[10px_10px_24px_rgba(87,66,62,0.12),-10px_-10px_24px_rgba(255,255,255,0.92)]">
          <div className="flex flex-wrap items-center justify-between gap-4">
            <div>
              <p className="text-xs font-semibold uppercase tracking-[0.2em] text-primary">Flasher</p>
              <h1 className="font-display mt-2 text-3xl font-semibold tracking-tight text-on-surface">{room.name}</h1>
            </div>
            <div className="flex items-center gap-2">
               <FirmwareBuildModal templateId="switch" homeId={homeId} />
            </div>
          </div>
          <div className="mt-3 flex flex-wrap gap-2">
            <Link
              href={`/dashboard/homes/${homeId}/rooms/${roomId}`}
              className="btn-outline px-3 py-1.5 text-sm"
            >
              Back to room
            </Link>
            <Link
              href={`/dashboard/homes/${homeId}/firmware`}
              className="btn-outline px-3 py-1.5 text-sm"
            >
              Manage firmware
            </Link>
          </div>
        </header>

        <Flasher
          firmwareVersions={flashFirmware}
          devices={flashDevices}
          appliances={appliances}
          mqttBrokerUrl={process.env.NEXT_PUBLIC_MQTT_BROKER_URL ?? "mqtts://mqtt.sol.muaathrifath.me:8883"}
          caCert={(process.env.NEXT_PUBLIC_MQTT_CA_CERT ?? "").replace(/\\n/g, "\n")}
        />
      </div>
    </div>
  )
}
