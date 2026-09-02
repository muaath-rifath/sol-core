"use client"

import type { Home } from "@/lib/sol-core"
import { createHomeAction } from "@/app/actions"
import { useRouter } from "next/navigation"
import { useEffect, useRef, useState, type FormEvent } from "react"

interface Props {
  homes: Home[]
  activeHomeId: string | undefined
}

function persistLastHome(id: string) {
  document.cookie = `last_home=${id}; path=/; max-age=31536000; SameSite=Lax`
}

export default function HomeSwitcher({ homes, activeHomeId }: Props) {
  const [open, setOpen] = useState(false)
  const [creating, setCreating] = useState(false)
  const [newHomeName, setNewHomeName] = useState("")
  const [createError, setCreateError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const router = useRouter()

  const activeHome = homes.find((h) => h.id === activeHomeId)

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false)
        setCreating(false)
        setNewHomeName("")
        setCreateError(null)
      }
    }
    if (open) document.addEventListener("mousedown", handleClick)
    return () => document.removeEventListener("mousedown", handleClick)
  }, [open])

  useEffect(() => {
    if (creating) inputRef.current?.focus()
  }, [creating])

  function switchHome(id: string) {
    setOpen(false)
    persistLastHome(id)
    router.push(`/dashboard/homes/${id}`)
  }

  function openCreate() {
    setCreating(true)
    setNewHomeName("")
    setCreateError(null)
  }

  function cancelCreate() {
    setCreating(false)
    setNewHomeName("")
    setCreateError(null)
  }

  async function handleCreate(e: FormEvent) {
    e.preventDefault()
    const name = newHomeName.trim()
    if (!name) return
    setSubmitting(true)
    setCreateError(null)
    try {
      const home = await createHomeAction(name)
      setOpen(false)
      setCreating(false)
      setNewHomeName("")
      persistLastHome(home.id)
      router.push(`/dashboard/homes/${home.id}`)
    } catch (err) {
      setCreateError(err instanceof Error ? err.message : "Failed to create home")
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div ref={ref} className="relative">
      <button
        onClick={() => setOpen((v) => !v)}
        className="group flex items-center gap-2 rounded-full bg-surface-container px-3 py-2 text-on-surface shadow-[4px_4px_8px_rgba(87,66,62,0.08),-4px_-4px_8px_rgba(255,255,255,0.92)] transition hover:scale-[1.02]"
      >
        <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-primary-container text-[11px] font-bold text-on-primary-container shadow-[inset_1px_1px_2px_rgba(255,255,255,0.65),2px_2px_5px_rgba(87,66,62,0.13)]">
          {activeHome?.name.charAt(0).toUpperCase() ?? "?"}
        </span>
        <span className="max-w-[160px] truncate text-sm font-semibold text-on-surface">
          {activeHome?.name ?? "Select home"}
        </span>
        <svg
          className="h-3 w-3 shrink-0 text-on-surface-variant"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          strokeWidth={2.5}
        >
          <path strokeLinecap="round" strokeLinejoin="round" d="M8 9l4-4 4 4M8 15l4 4 4-4" />
        </svg>
      </button>

      {open ? (
        <div className="absolute left-0 top-full z-50 mt-2 w-64 overflow-hidden rounded-2xl border border-white/60 bg-surface-container-low shadow-[8px_8px_18px_rgba(87,66,62,0.12),-8px_-8px_18px_rgba(255,255,255,0.92)]">
          {/* Home list */}
          {!creating ? (
            <>
              <div className="px-3 pt-3 pb-1.5">
                <p className="text-[11px] font-medium uppercase tracking-[0.14em] text-on-surface-variant">Your homes</p>
              </div>
              <div className="max-h-72 overflow-y-auto px-1.5 pb-1.5">
                {homes.length === 0 ? (
                  <p className="px-3 py-2 text-sm text-on-surface-variant">No homes yet.</p>
                ) : (
                  homes.map((home) => {
                    const active = home.id === activeHomeId
                    return (
                      <button
                        key={home.id}
                        onClick={() => switchHome(home.id)}
                        className={`flex w-full items-center gap-2.5 rounded-xl px-2 py-2 text-left transition ${
                          active
                            ? "bg-surface text-on-surface shadow-[inset_1px_1px_2px_rgba(255,255,255,0.7),2px_2px_6px_rgba(87,66,62,0.1)]"
                            : "text-on-surface-variant hover:bg-surface"
                        }`}
                      >
                        <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-primary-container text-xs font-bold text-on-primary-container">
                          {home.name.charAt(0).toUpperCase()}
                        </span>
                        <div className="min-w-0 flex-1">
                          <p className="truncate text-sm font-medium text-on-surface">{home.name}</p>
                          <p className="text-[11px] text-on-surface-variant capitalize">{home.my_role ?? "member"}</p>
                        </div>
                        {active ? (
                          <svg className="h-3.5 w-3.5 shrink-0 text-primary" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                            <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
                          </svg>
                        ) : null}
                      </button>
                    )
                  })
                )}
              </div>

              {/* Create home button */}
              <div className="border-t border-white/50 px-1.5 py-1.5">
                <button
                  onClick={openCreate}
                  className="flex w-full items-center gap-2 rounded-xl px-2 py-2 text-left text-on-surface-variant transition hover:bg-surface hover:text-on-surface"
                >
                  <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-surface-container text-on-surface-variant">
                    <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M12 4v16m8-8H4" />
                    </svg>
                  </span>
                  <span className="text-sm font-medium">Create home</span>
                </button>
              </div>
            </>
          ) : (
            /* Inline create form */
            <div className="p-3">
              <p className="mb-3 text-[11px] font-medium uppercase tracking-[0.14em] text-on-surface-variant">New home</p>
              <form onSubmit={handleCreate} className="space-y-2">
                <input
                  ref={inputRef}
                  type="text"
                  value={newHomeName}
                  onChange={(e) => setNewHomeName(e.target.value)}
                  placeholder="Home name"
                  maxLength={80}
                  disabled={submitting}
                  className="clay-inset w-full rounded-xl border border-white/50 px-3 py-2.5 text-sm text-on-surface placeholder:text-on-surface-variant/60 outline-none ring-2 ring-transparent transition focus:ring-primary/35 disabled:opacity-50"
                />
                {createError ? (
                  <p className="text-xs text-error">{createError}</p>
                ) : null}
                <div className="flex gap-2">
                  <button
                    type="button"
                    onClick={cancelCreate}
                    disabled={submitting}
                    className="flex-1 rounded-xl border border-outline-variant py-2 text-sm font-medium text-on-surface-variant transition hover:bg-surface disabled:opacity-50"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    disabled={submitting || !newHomeName.trim()}
                    className="btn-primary flex-1 rounded-xl py-2 text-sm font-semibold disabled:opacity-50"
                  >
                    {submitting ? "Creating…" : "Create"}
                  </button>
                </div>
              </form>
            </div>
          )}
        </div>
      ) : null}
    </div>
  )
}
