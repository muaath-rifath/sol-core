"use client"

import { useEffect, useState } from "react"
import { IconPlus, IconTool, IconX } from "@tabler/icons-react"

import Portal from "./ui/portal"
import { RESERVED_GPIO_PINS } from "@/lib/firmware-patcher"

interface DeviceOption {
  id: string
  name: string
}

interface ExistingAppliance {
  channel?: number
}

interface Props {
  addApplianceAction: (formData: FormData) => Promise<void>
  device: DeviceOption
  existingAppliances?: ExistingAppliance[]
}

export default function AddAppliancePopover({ addApplianceAction, device, existingAppliances = [] }: Props) {
  const [open, setOpen] = useState(false)
  const [gpioPin, setGpioPin] = useState("")
  const usedChannels = new Set(existingAppliances.map((a) => a.channel).filter((c): c is number => c != null))
  const gpioPinNum = gpioPin === "" ? NaN : Number(gpioPin)
  const isGpioReserved = !isNaN(gpioPinNum) && RESERVED_GPIO_PINS.has(gpioPinNum)

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        setOpen(false)
      }
    }
    if (open) {
      document.addEventListener("keydown", onKeyDown)
    } else {
      setGpioPin("")
    }
    return () => document.removeEventListener("keydown", onKeyDown)
  }, [open])

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="inline-flex items-center gap-1.5 rounded-md border border-outline-variant bg-surface-container-low px-2 py-1 text-[10px] font-bold text-on-surface-variant transition-colors hover:border-outline hover:bg-surface"
      >
        <IconPlus size={12} />
        Add Device
      </button>

      {open ? (
        <Portal>
          <div
            className="fixed inset-0 z-[80] flex items-center justify-center bg-black/35 p-4"
            onClick={() => setOpen(false)}
          >
          <div
            className="w-full max-w-md rounded-3xl border border-white/65 bg-surface-container p-6 shadow-[16px_16px_34px_rgba(27,28,25,0.18),-16px_-16px_34px_rgba(255,255,255,0.88)]"
            onClick={(event) => event.stopPropagation()}
          >
            <div className="mb-5 flex items-center justify-between">
              <h3 className="font-display text-lg font-semibold text-on-surface">Add Appliance</h3>
              <button
                type="button"
                className="rounded-full p-2 text-on-surface-variant transition hover:bg-surface"
                onClick={() => setOpen(false)}
              >
                <IconX size={16} />
              </button>
            </div>

            <form action={addApplianceAction} className="space-y-4">
              <input type="hidden" name="device_id" value={device.id} />

              <div className="rounded-xl border border-white/50 bg-surface px-3 py-2 text-sm text-on-surface-variant">
                Node: <span className="font-semibold text-on-surface">{device.name}</span>
              </div>

              <input
                type="text"
                name="name"
                required
                placeholder="Appliance name"
                className="clay-inset w-full rounded-xl border border-white/55 px-3 py-2.5 text-sm text-on-surface placeholder:text-on-surface-variant/70"
              />

              <select
                name="type"
                className="clay-inset w-full rounded-xl border border-white/55 px-3 py-2.5 text-sm text-on-surface"
              >
                <option value="switch">Switchboard Relay</option>
              </select>

              <div className="grid grid-cols-2 gap-2">
                <select
                  name="channel"
                  required
                  className="clay-inset w-full rounded-xl border border-white/55 px-3 py-2.5 text-sm text-on-surface"
                >
                  <option value="">Channel...</option>
                  {[0, 1, 2, 3].map((ch) => (
                    <option key={ch} value={ch} disabled={usedChannels.has(ch)}>
                      Ch {ch}{usedChannels.has(ch) ? " (in use)" : ""}
                    </option>
                  ))}
                </select>
                <div className="space-y-0.5">
                  <input
                    type="number"
                    name="gpio_pin"
                    placeholder="GPIO Pin (opt)"
                    value={gpioPin}
                    onChange={(e) => setGpioPin(e.target.value)}
                    className={`clay-inset w-full rounded-xl border px-3 py-2.5 text-sm text-on-surface placeholder:text-on-surface-variant/70 ${
                      isGpioReserved ? "border-error bg-error-container/10" : "border-white/55"
                    }`}
                  />
                  {isGpioReserved && (
                    <p className="text-[9px] font-semibold text-error">GPIO {gpioPin} is an I2S audio pin — don&apos;t use</p>
                  )}
                </div>
              </div>

              <label className="flex items-center gap-2 px-2 pb-1 text-sm text-on-surface-variant">
                <input type="checkbox" name="active_low" value="true" />
                Active Low Mapping
              </label>

              <div className="flex gap-3 pt-2">
                <button
                  type="button"
                  className="flex-1 rounded-xl border border-outline-variant px-3 py-2 text-sm font-medium text-on-surface-variant transition hover:bg-surface"
                  onClick={() => setOpen(false)}
                >
                  Cancel
                </button>
                <button type="submit" className="btn-outline flex-1 rounded-xl px-3 py-2 text-sm font-semibold">
                  <span className="inline-flex items-center gap-1.5">
                    <IconTool size={14} />
                    Create
                  </span>
                </button>
              </div>
            </form>
          </div>
        </div>
      </Portal>
      ) : null}
    </>
  )
}
