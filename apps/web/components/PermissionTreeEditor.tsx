"use client"

import type { PermissionScopeRef, PermissionTree, PermissionTreeRoom } from "@/lib/sol-core"
import {
  IconBolt,
  IconChevronDown,
  IconChevronRight,
  IconHome2,
  IconPencil,
  IconPower,
  IconTool,
  IconX,
} from "@tabler/icons-react"
import { useMemo, useState, useTransition } from "react"

type GrantSets = {
  rooms: Set<string>
  devices: Set<string>
  appliances: Set<string>
}

interface Props {
  tree: PermissionTree
  action: (formData: FormData) => Promise<void>
}

function seedGrants(tree: PermissionTree): GrantSets {
  const rooms = new Set<string>()
  const devices = new Set<string>()
  const appliances = new Set<string>()
  for (const room of tree.rooms) {
    if (room.granted_directly) rooms.add(room.id)
    for (const device of room.devices) {
      if (device.granted_directly) devices.add(device.id)
      for (const app of device.appliances) {
        if (app.granted_directly) appliances.add(app.id)
      }
    }
  }
  return { rooms, devices, appliances }
}

function cloneSets(g: GrantSets): GrantSets {
  return {
    rooms: new Set(g.rooms),
    devices: new Set(g.devices),
    appliances: new Set(g.appliances),
  }
}

type CheckState = "none" | "some" | "all"

function applianceEffective(g: GrantSets, applianceId: string, deviceId: string, roomId: string): boolean {
  return g.appliances.has(applianceId) || g.devices.has(deviceId) || g.rooms.has(roomId)
}

function deviceState(g: GrantSets, room: PermissionTreeRoom, deviceIdx: number): CheckState {
  const device = room.devices[deviceIdx]
  if (g.rooms.has(room.id) || g.devices.has(device.id)) return "all"
  if (device.appliances.length === 0) return "none"
  let granted = 0
  for (const a of device.appliances) {
    if (g.appliances.has(a.id)) granted++
  }
  if (granted === 0) return "none"
  if (granted === device.appliances.length) return "all"
  return "some"
}

function roomState(g: GrantSets, room: PermissionTreeRoom): CheckState {
  if (g.rooms.has(room.id)) return "all"
  if (room.devices.length === 0) return "none"
  let allCount = 0
  let someCount = 0
  for (let i = 0; i < room.devices.length; i++) {
    const s = deviceState(g, room, i)
    if (s === "all") allCount++
    else if (s === "some") someCount++
  }
  if (allCount === room.devices.length) return "all"
  if (allCount === 0 && someCount === 0) return "none"
  return "some"
}

// Promote upwards: if every appliance under a device is granted, replace the
// per-appliance grants with a device grant. If every device under a room is
// granted (directly), replace with a room grant. Keeps the saved set minimal.
function promote(g: GrantSets, tree: PermissionTree): GrantSets {
  const next = cloneSets(g)
  for (const room of tree.rooms) {
    let allDevicesGranted = room.devices.length > 0
    for (const device of room.devices) {
      if (next.devices.has(device.id)) continue
      if (device.appliances.length === 0) {
        allDevicesGranted = false
        continue
      }
      let allAppGranted = true
      for (const a of device.appliances) {
        if (!next.appliances.has(a.id)) {
          allAppGranted = false
          break
        }
      }
      if (allAppGranted) {
        next.devices.add(device.id)
        for (const a of device.appliances) next.appliances.delete(a.id)
      } else {
        allDevicesGranted = false
      }
    }
    if (allDevicesGranted && !next.rooms.has(room.id)) {
      next.rooms.add(room.id)
      for (const device of room.devices) next.devices.delete(device.id)
    }
  }
  return next
}

// Demote a room grant into per-device grants for the rest of its devices.
function demoteRoom(g: GrantSets, room: PermissionTreeRoom, exceptDeviceId?: string) {
  if (!g.rooms.has(room.id)) return
  g.rooms.delete(room.id)
  for (const device of room.devices) {
    if (device.id === exceptDeviceId) continue
    g.devices.add(device.id)
  }
}

// Demote a device grant into per-appliance grants for the rest of its appliances.
function demoteDevice(
  g: GrantSets,
  room: PermissionTreeRoom,
  deviceIdx: number,
  exceptApplianceId?: string,
) {
  const device = room.devices[deviceIdx]
  if (!g.devices.has(device.id)) return
  g.devices.delete(device.id)
  for (const a of device.appliances) {
    if (a.id === exceptApplianceId) continue
    g.appliances.add(a.id)
  }
}

function toggleRoom(g: GrantSets, room: PermissionTreeRoom): GrantSets {
  const next = cloneSets(g)
  const state = roomState(g, room)
  if (state === "all") {
    next.rooms.delete(room.id)
    for (const device of room.devices) {
      next.devices.delete(device.id)
      for (const a of device.appliances) next.appliances.delete(a.id)
    }
  } else {
    next.rooms.add(room.id)
    for (const device of room.devices) {
      next.devices.delete(device.id)
      for (const a of device.appliances) next.appliances.delete(a.id)
    }
  }
  return next
}

function toggleDevice(g: GrantSets, room: PermissionTreeRoom, deviceIdx: number): GrantSets {
  const next = cloneSets(g)
  const device = room.devices[deviceIdx]
  const state = deviceState(g, room, deviceIdx)

  if (g.rooms.has(room.id)) {
    demoteRoom(next, room, device.id)
    if (state === "all") {
      // device was inheriting room grant; turn this device off, leave others on
    } else {
      next.devices.add(device.id)
    }
    return next
  }

  if (state === "all") {
    next.devices.delete(device.id)
    for (const a of device.appliances) next.appliances.delete(a.id)
  } else {
    next.devices.add(device.id)
    for (const a of device.appliances) next.appliances.delete(a.id)
  }
  return next
}

function toggleAppliance(
  g: GrantSets,
  room: PermissionTreeRoom,
  deviceIdx: number,
  applianceId: string,
): GrantSets {
  const next = cloneSets(g)
  const device = room.devices[deviceIdx]
  const wasEffective = applianceEffective(g, applianceId, device.id, room.id)

  if (g.rooms.has(room.id)) {
    demoteRoom(next, room, device.id)
    next.devices.add(device.id)
  }
  if (next.devices.has(device.id)) demoteDevice(next, room, deviceIdx, applianceId)

  if (wasEffective) {
    next.appliances.delete(applianceId)
  } else {
    next.appliances.add(applianceId)
  }
  return next
}

function indeterminateRef(state: CheckState) {
  return (el: HTMLInputElement | null) => {
    if (el) el.indeterminate = state === "some"
  }
}

function buildGrantsList(g: GrantSets): PermissionScopeRef[] {
  const out: PermissionScopeRef[] = []
  for (const id of g.rooms) out.push({ type: "room", id })
  for (const id of g.devices) out.push({ type: "device", id })
  for (const id of g.appliances) out.push({ type: "appliance", id })
  return out
}

function seedManagedRooms(tree: PermissionTree): Set<string> {
  const s = new Set<string>()
  for (const room of tree.rooms) {
    if (room.can_manage_devices) s.add(room.id)
  }
  return s
}

export default function PermissionTreeEditor({ tree, action }: Props) {
  const initial = useMemo(() => seedGrants(tree), [tree])
  const initialManaged = useMemo(() => seedManagedRooms(tree), [tree])
  const [grants, setGrants] = useState<GrantSets>(initial)
  const [managedRooms, setManagedRooms] = useState<Set<string>>(initialManaged)
  const [editing, setEditing] = useState(false)
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set())
  const [pending, startTransition] = useTransition()

  const summary = useMemo(() => {
    let appCount = 0
    let totalApps = 0
    for (const room of tree.rooms) {
      for (const device of room.devices) {
        for (const a of device.appliances) {
          totalApps++
          if (applianceEffective(grants, a.id, device.id, room.id)) appCount++
        }
      }
    }
    return { granted: appCount, total: totalApps }
  }, [grants, tree])

  function applyChange(next: GrantSets) {
    setGrants(promote(next, tree))
  }

  function handleSave() {
    const list = buildGrantsList(grants)
    const fd = new FormData()
    fd.set("grants", JSON.stringify(list))
    fd.set("manage_rooms", JSON.stringify([...managedRooms]))
    startTransition(async () => {
      await action(fd)
    })
  }

  function handleCancel() {
    setGrants(initial)
    setManagedRooms(initialManaged)
    setEditing(false)
  }

  function toggleExpand(roomId: string) {
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(roomId)) next.delete(roomId)
      else next.add(roomId)
      return next
    })
  }

  return (
    <section className="rounded-3xl border border-white/40 bg-surface-container p-6 shadow-[8px_8px_16px_rgba(27,28,25,0.06),-8px_-8px_16px_rgba(255,255,255,0.9)]">
      <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
        <div>
          <h3 className="font-display text-lg font-semibold text-on-surface">
            Access scope
          </h3>
          <p className="mt-0.5 text-xs text-on-surface-variant">
            {summary.granted} of {summary.total} appliance{summary.total === 1 ? "" : "s"} accessible
          </p>
        </div>
        <div className="flex items-center gap-2">
          {!editing ? (
            <button
              type="button"
              onClick={() => setEditing(true)}
              className="btn-primary inline-flex items-center gap-1 rounded-xl px-3 py-2 text-sm font-semibold"
            >
              <IconPencil size={16} />
              Edit
            </button>
          ) : (
            <>
              <button
                type="button"
                onClick={handleCancel}
                disabled={pending}
                className="inline-flex items-center gap-1 rounded-xl border border-outline-variant px-3 py-2 text-sm font-medium text-on-surface-variant transition hover:bg-surface disabled:opacity-60"
              >
                <IconX size={16} />
                Cancel
              </button>
              <button
                type="button"
                onClick={handleSave}
                disabled={pending}
                className="btn-primary inline-flex items-center gap-1 rounded-xl px-3 py-2 text-sm font-semibold disabled:opacity-60"
              >
                {pending ? "Saving…" : "Save"}
              </button>
            </>
          )}
        </div>
      </div>

      <ul className="flex flex-col gap-3">
        {tree.rooms.length === 0 ? (
          <li className="rounded-2xl border border-dashed border-outline-variant px-4 py-6 text-center text-sm text-on-surface-variant">
            This home has no rooms yet.
          </li>
        ) : null}
        {tree.rooms.map((room) => {
          const rState = roomState(grants, room)
          const isOpen = expanded.has(room.id)
          return (
            <li
              key={room.id}
              className="rounded-2xl border border-white/55 bg-surface px-4 py-3"
            >
              <div className="flex items-center gap-3">
                <button
                  type="button"
                  onClick={() => toggleExpand(room.id)}
                  className="text-on-surface-variant transition hover:text-on-surface"
                  aria-label={isOpen ? "Collapse room" : "Expand room"}
                >
                  {isOpen ? <IconChevronDown size={18} /> : <IconChevronRight size={18} />}
                </button>
                <input
                  type="checkbox"
                  className="size-4 accent-primary disabled:opacity-50"
                  checked={rState === "all"}
                  ref={indeterminateRef(rState)}
                  disabled={!editing || room.devices.length === 0}
                  onChange={() => applyChange(toggleRoom(grants, room))}
                />
                <IconHome2 size={16} className="text-primary" />
                <span className="flex-1 font-semibold text-on-surface">{room.name}</span>
                {editing ? (
                  <button
                    type="button"
                    title={managedRooms.has(room.id) ? "Revoke device management" : "Grant device management"}
                    aria-label={managedRooms.has(room.id) ? "Revoke device management" : "Grant device management"}
                    onClick={() =>
                      setManagedRooms((prev) => {
                        const next = new Set(prev)
                        if (next.has(room.id)) next.delete(room.id)
                        else next.add(room.id)
                        return next
                      })
                    }
                    className={`rounded-lg p-1 transition ${
                      managedRooms.has(room.id)
                        ? "text-primary"
                        : "text-on-surface-variant/40 hover:text-on-surface-variant"
                    }`}
                  >
                    <IconTool size={15} />
                  </button>
                ) : managedRooms.has(room.id) ? (
                  <IconTool size={15} className="text-primary" title="Can manage devices" />
                ) : null}
                <span className="text-xs text-on-surface-variant">
                  {room.devices.length} switchboard{room.devices.length === 1 ? "" : "s"}
                </span>
              </div>

              {isOpen ? (
                <ul className="mt-3 ml-7 flex flex-col gap-2 border-l border-outline-variant pl-4">
                  {room.devices.length === 0 ? (
                    <li className="text-xs text-on-surface-variant">No switchboards.</li>
                  ) : null}
                  {room.devices.map((device, deviceIdx) => {
                    const dState = deviceState(grants, room, deviceIdx)
                    return (
                      <li key={device.id} className="rounded-xl bg-surface-container-low px-3 py-2">
                        <div className="flex items-center gap-3">
                          <input
                            type="checkbox"
                            className="size-4 accent-primary disabled:opacity-50"
                            checked={dState === "all"}
                            ref={indeterminateRef(dState)}
                            disabled={!editing || device.appliances.length === 0}
                            onChange={() => applyChange(toggleDevice(grants, room, deviceIdx))}
                          />
                          <IconBolt size={14} className="text-secondary" />
                          <span className="flex-1 text-sm font-medium text-on-surface">
                            {device.name}
                          </span>
                          <span className="text-[11px] text-on-surface-variant">
                            {device.appliances.length} appliance{device.appliances.length === 1 ? "" : "s"}
                          </span>
                        </div>
                        <ul className="mt-2 ml-6 flex flex-col gap-1.5">
                          {device.appliances.map((a) => {
                            const effective = applianceEffective(grants, a.id, device.id, room.id)
                            return (
                              <li
                                key={a.id}
                                className="flex items-center gap-3 rounded-lg px-2 py-1 text-sm text-on-surface"
                              >
                                <input
                                  type="checkbox"
                                  className="size-4 accent-primary disabled:opacity-50"
                                  checked={effective}
                                  disabled={!editing}
                                  onChange={() =>
                                    applyChange(toggleAppliance(grants, room, deviceIdx, a.id))
                                  }
                                />
                                <IconPower size={12} className="text-on-surface-variant" />
                                <span className="flex-1">{a.name}</span>
                                {typeof a.channel === "number" ? (
                                  <span className="text-[11px] text-on-surface-variant">
                                    ch {a.channel}
                                  </span>
                                ) : null}
                              </li>
                            )
                          })}
                          {device.appliances.length === 0 ? (
                            <li className="text-xs text-on-surface-variant">No appliances.</li>
                          ) : null}
                        </ul>
                      </li>
                    )
                  })}
                </ul>
              ) : null}
            </li>
          )
        })}
      </ul>
    </section>
  )
}
