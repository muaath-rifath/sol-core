"use client"

import { type OTAAttempt, type OTAAttemptStatus } from "@/lib/sol-core"
import { cancelOTAAction, listOTAAttemptsAction, retryOTAAction } from "@/lib/actions"
import { IconLoader2, IconRefresh, IconX } from "@tabler/icons-react"
import clsx from "clsx"
import { useEffect, useMemo, useRef, useState } from "react"

type DeviceInfo = {
  id: string
  name: string
}

type Props = {
  homeId: string
  roomId: string
  devices: DeviceInfo[]
  initialAttempts: OTAAttempt[]
}

const ACTIVE_STATUSES: OTAAttemptStatus[] = ["initiated", "acknowledged", "downloading", "verifying", "updating", "cancelling"]

const CANCELLABLE_STATUSES: OTAAttemptStatus[] = ["initiated", "acknowledged", "downloading", "verifying", "updating"]

function statusTone(status: OTAAttemptStatus): "error" | "success" | "info" {
  if (status === "failed" || status === "timed_out") return "error"
  if (status === "updated" || status === "cancelled") return "success"
  return "info"
}

function statusLabel(status: OTAAttemptStatus): string {
  switch (status) {
    case "initiated":
      return "Initiated"
    case "acknowledged":
      return "Acknowledged"
    case "downloading":
      return "Downloading"
    case "verifying":
      return "Verifying"
    case "updating":
      return "Flashing"
    case "cancelling":
      return "Cancelling"
    case "cancelled":
      return "Cancelled"
    case "timed_out":
      return "Timed Out"
    case "updated":
      return "Completed"
    case "failed":
      return "Failed"
    default:
      return status
  }
}

function wsUrl(): string {
  const configured = process.env.NEXT_PUBLIC_SOL_CORE_WS_URL?.trim()
  if (configured) {
    return configured
  }
  if (typeof window === "undefined") {
    return ""
  }
  const proto = window.location.protocol === "https:" ? "wss:" : "ws:"
  return `${proto}//${window.location.host}/ws`
}

export default function OTALogPanel({ homeId, roomId, devices, initialAttempts }: Props) {
  const [attempts, setAttempts] = useState<OTAAttempt[]>(initialAttempts)
  const [activeID, setActiveID] = useState<string>(initialAttempts[0]?.id ?? "")
  const [showLogs, setShowLogs] = useState(false)
  const [isRetrying, setIsRetrying] = useState(false)
  const [isCancelling, setIsCancelling] = useState(false)
  const logEndRef = useRef<HTMLDivElement>(null)

  const deviceByID = useMemo(() => {
    return new Map(devices.map((d) => [d.id, d.name]))
  }, [devices])

  const activeAttempt = useMemo(() => {
    return attempts.find((a) => a.id === activeID) ?? attempts[0]
  }, [attempts, activeID])

  useEffect(() => {
    if (activeAttempt) {
      setActiveID(activeAttempt.id)
    }
  }, [activeAttempt?.id])

  useEffect(() => {
    if (showLogs && logEndRef.current) {
      logEndRef.current.scrollIntoView({ behavior: "smooth" })
    }
  }, [showLogs, activeAttempt?.logs])

  useEffect(() => {
    let closed = false
    let socket: WebSocket | null = null

    async function connect() {
      try {
        const tokenRes = await fetch("/api/ws-token", { cache: "no-store" })
        if (!tokenRes.ok) return
        const json = await tokenRes.json() as { token?: string }
        if (!json.token) return
        const base = wsUrl()
        if (!base) return
        const sep = base.includes("?") ? "&" : "?"
        socket = new WebSocket(`${base}${sep}token=${encodeURIComponent(json.token)}`)

        socket.onmessage = (ev) => {
          try {
            const payload = JSON.parse(ev.data) as { type?: string; data?: OTAAttempt }
            if (payload.type !== "ota.attempt.updated" || !payload.data) {
              return
            }
            const next = payload.data
            if (next.home_id !== homeId || next.room_id !== roomId) {
              return
            }
            setAttempts((prev) => {
              const idx = prev.findIndex((x) => x.id === next.id)
              if (idx === -1) {
                return [next, ...prev]
              }
              const copy = [...prev]
              copy[idx] = next
              copy.sort((a, b) => +new Date(b.created_at) - +new Date(a.created_at))
              return copy
            })
          } catch {
            // ignore malformed socket payload
          }
        }
      } catch {
        // ignore ws token failures
      }
    }

    connect()

    const poll = window.setInterval(async () => {
      if (closed) return
      try {
        const res = await listOTAAttemptsAction(homeId, roomId, 50)
        if (res.data) {
          setAttempts(res.data)
        }
      } catch {
        // ignore periodic refresh failures
      }
    }, 5000)

    return () => {
      closed = true
      window.clearInterval(poll)
      if (socket) socket.close()
    }
  }, [homeId, roomId])

  async function retryActive() {
    if (!activeAttempt || ACTIVE_STATUSES.includes(activeAttempt.status)) {
      return
    }
    setIsRetrying(true)
    try {
      await retryOTAAction(homeId, roomId, activeAttempt.id)
      const refreshed = await listOTAAttemptsAction(homeId, roomId, 50)
      if (refreshed.data) {
        setAttempts(refreshed.data)
      }
    } finally {
      setIsRetrying(false)
    }
  }

  async function cancelActive() {
    if (!activeAttempt || !CANCELLABLE_STATUSES.includes(activeAttempt.status)) {
      return
    }
    setIsCancelling(true)
    try {
      await cancelOTAAction(homeId, roomId, activeAttempt.id)
      const refreshed = await listOTAAttemptsAction(homeId, roomId, 50)
      if (refreshed.data) {
        setAttempts(refreshed.data)
      }
    } finally {
      setIsCancelling(false)
    }
  }

  return (
    <section className="rounded-3xl border border-white/45 bg-surface-container p-5 shadow-[8px_8px_16px_rgba(87,66,62,0.05),-8px_-8px_16px_rgba(255,255,255,0.8)]">
      <div className="mb-4 flex items-center justify-between">
        <h3 className="font-display text-lg sm:text-xl font-semibold text-on-surface">OTA Flashing Logs</h3>
        <div className="flex items-center gap-2">
          {activeAttempt && (
            <button
              type="button"
              onClick={cancelActive}
              disabled={isCancelling || !CANCELLABLE_STATUSES.includes(activeAttempt.status)}
              className="inline-flex items-center gap-1 rounded-lg border border-outline-variant/50 px-3 py-1.5 text-xs font-semibold text-on-surface-variant disabled:opacity-50"
            >
              {isCancelling ? <IconLoader2 size={14} className="animate-spin" /> : <IconX size={14} />}
              Cancel
            </button>
          )}
          {activeAttempt && (
            <button
              type="button"
              onClick={retryActive}
              disabled={isRetrying || ACTIVE_STATUSES.includes(activeAttempt.status)}
              className="inline-flex items-center gap-1 rounded-lg border border-outline-variant/50 px-3 py-1.5 text-xs font-semibold text-on-surface-variant disabled:opacity-50"
            >
              {isRetrying ? <IconLoader2 size={14} className="animate-spin" /> : <IconRefresh size={14} />}
              Retry
            </button>
          )}
        </div>
      </div>

      {attempts.length === 0 ? (
        <p className="text-sm text-outline">No OTA attempts yet.</p>
      ) : (
        <div className="space-y-2">
          {attempts.slice(0, 8).map((attempt) => {
            const tone = statusTone(attempt.status)
            return (
              <button
                type="button"
                key={attempt.id}
                onClick={() => {
                  setActiveID(attempt.id)
                  setShowLogs(true)
                }}
                className={clsx(
                  "w-full rounded-xl border px-3 py-2 text-left transition",
                  activeID === attempt.id ? "border-primary/50 bg-primary/5" : "border-white/45 bg-surface",
                )}
              >
                <div className="flex items-center justify-between gap-2">
                  <span className="truncate text-sm font-semibold text-on-surface">
                    {deviceByID.get(attempt.device_id) ?? attempt.device_id}
                  </span>
                  <span className={clsx("rounded-full px-2 py-0.5 text-[10px] font-bold uppercase tracking-wide", {
                    "bg-error-container text-on-error-container": tone === "error",
                    "bg-tertiary-fixed text-on-tertiary-fixed": tone === "success",
                    "bg-secondary-container text-on-secondary-container": tone === "info",
                  })}>
                    {statusLabel(attempt.status)}
                  </span>
                </div>
                <div className="mt-2 h-2 overflow-hidden rounded-full bg-surface-container-high">
                  <div className={clsx("h-full rounded-full", {
                    "bg-error": tone === "error",
                    "bg-tertiary": tone === "success",
                    "bg-primary": tone === "info",
                  })} style={{ width: `${attempt.progress_pct}%` }} />
                </div>
                <p className="mt-1 text-[11px] text-outline">
                  {new Date(attempt.updated_at).toLocaleString()} - {attempt.progress_pct}%
                </p>
              </button>
            )
          })}
        </div>
      )}

      {showLogs && activeAttempt && (
        <div className="fixed inset-0 z-[100] flex items-center justify-center bg-stone-900/45 p-4 backdrop-blur-sm">
          <div className="bg-clay-canvas w-full max-w-3xl overflow-hidden rounded-[2.5rem] border border-white/60 shadow-2xl">
            <div className="flex items-center justify-between border-b border-white/40 p-6">
              <div>
                <h4 className="font-display text-xl font-bold text-on-surface">OTA Flash Logs</h4>
                <p className="mt-0.5 text-xs text-on-surface-variant">
                  {deviceByID.get(activeAttempt.device_id) ?? activeAttempt.device_id} - {statusLabel(activeAttempt.status)} - {activeAttempt.progress_pct}%
                </p>
              </div>
              <button onClick={() => setShowLogs(false)} className="text-on-surface-variant hover:text-on-surface">
                <IconX size={22} />
              </button>
            </div>
            <div className="p-6">
              <div className="clay-inset relative h-96 overflow-hidden rounded-2xl border border-white/55 bg-stone-950 font-mono text-xs text-stone-100">
                <div className="absolute inset-0 overflow-y-auto p-4">
                  {(activeAttempt.logs || "No logs yet.").split("\n").map((line, idx) => (
                    <div key={idx} className={clsx("mb-1", {
                      "text-error": line.toLowerCase().includes("fail") || line.toLowerCase().includes("error"),
                      "text-emerald-400": line.toLowerCase().includes("success") || line.toLowerCase().includes("updated"),
                      "text-stone-300": !line.toLowerCase().includes("fail") && !line.toLowerCase().includes("error") && !line.toLowerCase().includes("success") && !line.toLowerCase().includes("updated"),
                    })}>
                      {line}
                    </div>
                  ))}
                  {activeAttempt.error_text ? (
                    <div className="mt-3 rounded-md bg-error-container/30 px-2 py-1 text-error">{activeAttempt.error_text}</div>
                  ) : null}
                  <div ref={logEndRef} />
                </div>
              </div>
            </div>
          </div>
        </div>
      )}
    </section>
  )
}
