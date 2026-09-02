"use client"

import { useCallback, useEffect, useRef, useState } from "react"
import { IconCpu, IconPencil, IconX } from "@tabler/icons-react"
import { type Appliance, type RoomDevice } from "@/lib/sol-core"
import AddAppliancePopover from "@/components/AddAppliancePopover"
import { useWS } from "@/providers/WSProvider"

function coerceBool(value: unknown): boolean {
  if (typeof value === "boolean") return value
  if (typeof value === "number") return value !== 0
  if (typeof value === "string") return value === "true" || value === "1" || value === "on"
  return false
}

interface Props {
  homeId: string
  roomId: string
  canManage: boolean
  initialDevices: RoomDevice[]
  initialAppliances: Appliance[]
  addApplianceAction: (formData: FormData) => Promise<void>
  deleteDeviceAction: (formData: FormData) => Promise<void>
  deleteApplianceAction: (formData: FormData) => Promise<void>
  updateApplianceAction?: (formData: FormData) => Promise<void>
}

interface DeviceStateEvent {
  device_id: string
  state: Record<string, unknown>
}

interface ApplianceStateEvent {
  appliance_id: string
  state: Record<string, unknown>
}

export default function RoomDeviceList({
  homeId,
  roomId,
  canManage,
  initialDevices,
  initialAppliances,
  addApplianceAction,
  deleteDeviceAction,
  deleteApplianceAction,
  updateApplianceAction,
}: Props) {
  const { subscribe, send } = useWS()
  const [devices, setDevices] = useState<RoomDevice[]>(initialDevices)
  const [appliances, setAppliances] = useState<Appliance[]>(initialAppliances)

  const pendingRef = useRef<Map<string, boolean>>(new Map())
  // Maps correlationId → {applianceId, previousIsOn} for rollback on failure
  const pendingCommandsRef = useRef<Map<string, { applianceId: string; previousIsOn: boolean }>>(new Map())
  const [editingApplianceId, setEditingApplianceId] = useState<string | null>(null)

  useEffect(() => {
    const unsubDevice = subscribe("device.state", (raw) => {
      const data = raw as DeviceStateEvent
      if (!data?.device_id) return
      setDevices((prev) =>
        prev.map((d) =>
          d.id === data.device_id
            ? { ...d, state: data.state, online: data.state?.online !== false }
            : d
        )
      )
    })

    const unsubAppliance = subscribe("appliance.state", (raw) => {
      const data = raw as ApplianceStateEvent
      if (!data?.appliance_id) return
      // Device confirmed its real state — clear any pending optimistic entry.
      pendingRef.current.delete(data.appliance_id)
      for (const [cid, entry] of pendingCommandsRef.current) {
        if (entry.applianceId === data.appliance_id) {
          pendingCommandsRef.current.delete(cid)
        }
      }
      setAppliances((prev) =>
        prev.map((a) =>
          a.id === data.appliance_id ? { ...a, state: data.state } : a
        )
      )
    })

    return () => {
      unsubDevice()
      unsubAppliance()
    }
  }, [subscribe])

  const toggle = useCallback(
    (deviceId: string, channel: number, currentlyOn: boolean, applianceId: string) => {
      const turnOn = !currentlyOn
      const correlationId = crypto.randomUUID()

      pendingRef.current.set(applianceId, turnOn)
      pendingCommandsRef.current.set(correlationId, { applianceId, previousIsOn: currentlyOn })

      setAppliances((prev) =>
        prev.map((a) =>
          a.id === applianceId ? { ...a, state: { ...a.state, isOn: turnOn } } : a
        )
      )

      send({
        type: "device.command",
        correlationId,
        data: {
          home_id: homeId,
          room_id: roomId,
          device_id: deviceId,
          action: "relay_set",
          params: { channel, power: turnOn },
        },
      })
    },
    [homeId, roomId, send]
  )

  useEffect(() => {
    const unsub = subscribe("command.ack", (raw) => {
      const data = raw as { correlationId?: string; success?: boolean }
      if (!data?.correlationId || data.success !== false) return
      const pending = pendingCommandsRef.current.get(data.correlationId)
      if (!pending) return
      pendingCommandsRef.current.delete(data.correlationId)
      pendingRef.current.delete(pending.applianceId)
      setAppliances((prev) =>
        prev.map((a) =>
          a.id === pending.applianceId
            ? { ...a, state: { ...a.state, isOn: pending.previousIsOn } }
            : a
        )
      )
    })
    return unsub
  }, [subscribe])

  if (devices.length === 0) {
    return (
      <div className="py-8 text-center">
        <p className="text-sm text-outline">No devices in this room</p>
      </div>
    )
  }

  return (
    <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
      {devices.map((device) => {
        const deviceAppliances = appliances.filter((a) => a.device_id === device.id)

        return (
          <div
            key={device.id}
            className="group relative flex flex-col overflow-hidden rounded-[2rem] border border-white/60 bg-surface-container-low p-1 shadow-[8px_8px_24px_rgba(87,66,62,0.07),-8px_-8px_24px_rgba(255,255,255,0.9)] transition-all hover:scale-[1.01] hover:shadow-[12px_12px_32px_rgba(87,66,62,0.1)]"
          >
            <div className="p-5 pb-4">
              <div className="flex items-start justify-between">
                <div className="flex items-center gap-3">
                  <div
                    className={`flex h-12 w-12 items-center justify-center rounded-2xl shadow-[inset_1px_1px_4px_rgba(255,255,255,0.6)] ${
                      device.online ? "bg-emerald-50 text-emerald-600" : "bg-stone-100 text-stone-400"
                    }`}
                  >
                    <IconCpu size={24} />
                  </div>
                  <div>
                    <h3 className="font-display text-lg font-bold text-on-surface leading-tight">
                      {device.name}
                    </h3>
                    <p className="mt-0.5 text-[10px] font-semibold uppercase tracking-wider text-outline">
                      {device.type} · {device.online ? "Connected" : "Offline"}
                    </p>
                  </div>
                </div>
                <div className="flex items-center gap-1.5">
                  {canManage && (
                    <>
                      <AddAppliancePopover
                        addApplianceAction={addApplianceAction}
                        device={{ id: device.id, name: device.name }}
                        existingAppliances={deviceAppliances}
                      />
                      <form action={deleteDeviceAction}>
                        <input type="hidden" name="device_id" value={device.id ?? ""} />
                        <button
                          type="submit"
                          className="flex h-8 w-8 items-center justify-center rounded-full bg-error-container/10 text-error opacity-0 transition-opacity group-hover:opacity-100 hover:bg-error-container/20"
                          title="Delete Device"
                        >
                          <IconX size={14} />
                        </button>
                      </form>
                    </>
                  )}
                </div>
              </div>

              <div className="mt-4 flex items-center gap-4">
                <div className="flex flex-col">
                  <span className="text-[9px] font-bold uppercase tracking-widest text-outline">
                    Uptime
                  </span>
                  <span className="text-xs font-medium text-on-surface-variant">
                    {formatUptime(device.state?.uptime as number)}
                  </span>
                </div>
                <div className="flex flex-col">
                  <span className="text-[9px] font-bold uppercase tracking-widest text-outline">
                    IP Address
                  </span>
                  <span className="text-xs font-medium text-on-surface-variant">
                    {String(device.state?.ip_address ?? "---")}
                  </span>
                </div>
              </div>
            </div>

            <div className="mt-auto space-y-1.5 bg-surface-container/40 px-3 pb-3 pt-2">
              {deviceAppliances.length === 0 ? (
                <div className="py-3 text-center">
                  <p className="text-[10px] italic text-outline">No appliances configured</p>
                </div>
              ) : (
                deviceAppliances.map((app) => {
                  const isOn = coerceBool(app.state?.isOn)
                  const isEditing = editingApplianceId === app.id
                  return (
                    <div key={app.id} className="space-y-1">
                      <div className="group/item flex w-full items-center gap-3 rounded-2xl border border-white/50 bg-white/30 px-4 py-3 shadow-[inset_1px_1px_3px_rgba(255,255,255,0.9),2px_2px_6px_rgba(87,66,62,0.04)] transition-colors hover:bg-white/50">
                        <div
                          className={`h-2 w-2 shrink-0 rounded-full transition-colors ${
                            isOn
                              ? "bg-emerald-500 shadow-[0_0_6px_rgba(16,185,129,0.55)]"
                              : "bg-stone-400"
                          }`}
                        />

                        <div className="min-w-0 flex-1">
                          <p className="truncate text-sm font-semibold text-on-surface leading-tight">
                            {app.name}
                          </p>
                          <p className="truncate text-[10px] text-on-surface-variant/60 leading-tight">
                            {app.type} · Ch{app.channel ?? "?"}
                            {app.gpio_pin != null ? ` · GPIO${app.gpio_pin}` : ""}
                          </p>
                        </div>

                        <div className="flex shrink-0 items-center gap-2">
                          {canManage && (
                            <div className="flex gap-1 opacity-0 transition-opacity group-hover/item:opacity-100">
                              {updateApplianceAction && (
                                <button
                                  type="button"
                                  onClick={() => setEditingApplianceId(isEditing ? null : app.id)}
                                  className="flex h-6 w-6 items-center justify-center rounded-lg border border-outline-variant/30 bg-surface-container text-on-surface-variant hover:bg-surface"
                                >
                                  <IconPencil size={11} />
                                </button>
                              )}
                              <form action={deleteApplianceAction}>
                                <input type="hidden" name="appliance_id" value={app.id} />
                                <button
                                  type="submit"
                                  className="flex h-6 w-6 items-center justify-center rounded-lg border border-error/20 bg-error-container/10 text-error hover:bg-error-container/30"
                                >
                                  <IconX size={11} />
                                </button>
                              </form>
                            </div>
                          )}

                          <button
                            type="button"
                            aria-label={isOn ? "Turn off" : "Turn on"}
                            onClick={() => toggle(device.id, app.channel ?? 0, isOn, app.id)}
                            className={`relative h-6 w-11 rounded-full border transition-all duration-200 focus:outline-none ${
                              isOn
                                ? "border-primary/30 bg-primary shadow-[inset_0_1px_3px_rgba(0,0,0,0.15)]"
                                : "border-outline-variant/40 bg-surface-container-high shadow-[inset_2px_2px_4px_rgba(87,66,62,0.12),inset_-2px_-2px_4px_rgba(255,255,255,0.8)]"
                            }`}
                          >
                            <span
                              className={`absolute left-0.5 top-0.5 h-5 w-5 rounded-full bg-white shadow-[2px_2px_4px_rgba(87,66,62,0.15),inset_1px_1px_2px_rgba(255,255,255,0.9)] transition-transform duration-200 ${
                                isOn ? "translate-x-5" : "translate-x-0"
                              }`}
                            />
                          </button>
                        </div>
                      </div>

                      {isEditing && updateApplianceAction && (
                        <form
                          action={updateApplianceAction}
                          onSubmit={() => setEditingApplianceId(null)}
                          className="ml-4 flex flex-wrap items-center gap-1.5 rounded-xl border border-outline-variant/30 bg-surface p-2 text-xs"
                        >
                          <input type="hidden" name="appliance_id" value={app.id} />
                          <input
                            type="text"
                            name="name"
                            defaultValue={app.name}
                            placeholder="Name"
                            className="clay-inset w-24 rounded-lg border border-white/55 px-2 py-1 text-xs text-on-surface"
                          />
                          <input
                            type="number"
                            name="gpio_pin"
                            defaultValue={app.gpio_pin ?? ""}
                            placeholder="GPIO"
                            min={0}
                            max={48}
                            className="clay-inset w-16 rounded-lg border border-white/55 px-2 py-1 text-xs text-on-surface"
                          />
                          <label className="flex items-center gap-1 text-[10px] text-on-surface-variant">
                            <input type="checkbox" name="active_low" value="true" defaultChecked={app.active_low} />
                            Active low
                          </label>
                          <button type="submit" className="rounded-lg border border-primary/30 bg-primary-container/30 px-2 py-1 text-[10px] font-semibold text-on-primary-container">
                            Save
                          </button>
                          <button type="button" onClick={() => setEditingApplianceId(null)} className="rounded-lg border border-outline-variant/30 px-2 py-1 text-[10px] text-on-surface-variant">
                            Cancel
                          </button>
                        </form>
                      )}
                    </div>
                  )
                })
              )}
            </div>
          </div>
        )
      })}
    </div>
  )
}

function formatUptime(ms?: number): string {
  if (!ms) return "--"
  const d = Math.floor(ms / (1000 * 60 * 60 * 24))
  const h = Math.floor((ms % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60))
    .toString()
    .padStart(2, "0")
  const m = Math.floor((ms % (1000 * 60 * 60)) / (1000 * 60))
    .toString()
    .padStart(2, "0")
  return `${d}d ${h}h ${m}m`
}
