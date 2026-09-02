"use client"

import { useEffect, useState } from "react"
import { IconX } from "@tabler/icons-react"

import Portal from "./ui/portal"

interface Props {
  createRoomAction: (formData: FormData) => Promise<void>
}

export default function CreateRoomPopover({ createRoomAction }: Props) {
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
        className="flex w-full items-center justify-center rounded-full bg-primary px-6 py-3.5 text-sm font-semibold text-on-primary shadow-[8px_8px_14px_rgba(165,59,41,0.24),-4px_-4px_10px_rgba(255,255,255,0.75)] transition-all hover:brightness-105"
        type="button"
        onClick={() => setOpen(true)}
      >
        Create New Room
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
              <h3 className="font-display text-lg font-semibold text-on-surface">Create a New Room</h3>
              <button
                type="button"
                className="rounded-full p-2 text-on-surface-variant transition hover:bg-surface"
                onClick={() => setOpen(false)}
              >
                <IconX size={16} />
              </button>
            </div>

            <form action={createRoomAction} className="space-y-4">
              <input
                type="text"
                name="name"
                required
                maxLength={80}
                placeholder="Room name"
                className="clay-inset w-full rounded-xl border border-white/55 px-3 py-2.5 text-sm text-on-surface placeholder:text-on-surface-variant/70"
              />
              <input
                type="number"
                name="floor"
                placeholder="Floor (optional)"
                className="clay-inset w-full rounded-xl border border-white/55 px-3 py-2.5 text-sm text-on-surface placeholder:text-on-surface-variant/70"
              />

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
