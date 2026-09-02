"use client"

import { useEffect, useState } from "react"
import { IconBolt, IconX } from "@tabler/icons-react"

import Portal from "./ui/portal"

interface Props {
  createDeviceAction: (formData: FormData) => Promise<void>
}

export default function CreateDevicePopover({ createDeviceAction }: Props) {
  const [open, setOpen] = useState(false)

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        setOpen(false)
      }
    }
    if (open) {
      document.addEventListener("keydown", onKeyDown)
    }
    return () => document.removeEventListener("keydown", onKeyDown)
  }, [open])

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="btn-primary inline-flex items-center gap-2 rounded-xl px-4 py-2 text-sm font-semibold"
      >
        <IconBolt size={16} />
        Create Node
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
              <h3 className="font-display text-lg font-semibold text-on-surface">Create Hub Node</h3>
              <button
                type="button"
                className="rounded-full p-2 text-on-surface-variant transition hover:bg-surface"
                onClick={() => setOpen(false)}
              >
                <IconX size={16} />
              </button>
            </div>

            <form action={createDeviceAction} className="space-y-4">
              <input
                type="text"
                name="name"
                required
                placeholder="Node identifier (e.g. ESP32-01)"
                className="clay-inset w-full rounded-xl border border-white/55 px-3 py-2.5 text-sm text-on-surface placeholder:text-on-surface-variant/70"
              />

              <input type="hidden" name="type" value="switch" />
              <div className="rounded-xl border border-white/50 bg-surface px-3 py-2 text-sm text-on-surface-variant">
                Node Type: <span className="font-semibold text-on-surface">ESP32 Universal</span>
              </div>

              <div className="flex gap-3 pt-2">
                <button
                  type="button"
                  className="flex-1 rounded-xl border border-outline-variant px-3 py-2 text-sm font-medium text-on-surface-variant transition hover:bg-surface"
                  onClick={() => setOpen(false)}
                >
                  Cancel
                </button>
                <button type="submit" className="btn-primary flex-1 rounded-xl px-3 py-2 text-sm font-semibold">
                  Create
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
