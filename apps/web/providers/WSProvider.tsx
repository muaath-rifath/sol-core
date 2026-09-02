"use client"

import { createContext, useCallback, useContext, useEffect, useRef } from "react"

type EventHandler = (data: unknown) => void

interface WSContextValue {
  subscribe: (type: string, handler: EventHandler) => () => void
  send: (msg: object) => void
}

const WSContext = createContext<WSContextValue | null>(null)

const WS_URL = process.env.NEXT_PUBLIC_SOL_CORE_WS_URL

export function WSProvider({ children }: { children: React.ReactNode }) {
  const socketRef = useRef<WebSocket | null>(null)
  const listenersRef = useRef<Map<string, Set<EventHandler>>>(new Map())
  const reconnectDelay = useRef(250)
  const unmounted = useRef(false)

  const dispatch = useCallback((type: string, data: unknown) => {
    const handlers = listenersRef.current.get(type)
    if (handlers) {
      for (const h of handlers) h(data)
    }
  }, [])

  useEffect(() => {
    unmounted.current = false

    async function connect() {
      if (unmounted.current || !WS_URL) return

      try {
        const res = await fetch("/api/ws-token", { cache: "no-store" })
        if (!res.ok) throw new Error("token fetch failed")
        const { token } = (await res.json()) as { token?: string }
        if (!token) throw new Error("no token")

        const sep = WS_URL.includes("?") ? "&" : "?"
        const ws = new WebSocket(`${WS_URL}${sep}token=${encodeURIComponent(token)}`)
        socketRef.current = ws

        ws.onopen = () => {
          reconnectDelay.current = 250
        }

        ws.onmessage = (ev) => {
          try {
            const payload = JSON.parse(ev.data) as { type?: string; data?: unknown }
            if (payload.type) dispatch(payload.type, payload.data)
          } catch {
            // ignore malformed frames
          }
        }

        ws.onclose = () => {
          socketRef.current = null
          if (!unmounted.current) scheduleReconnect()
        }

        ws.onerror = () => {
          ws.close()
        }
      } catch {
        if (!unmounted.current) scheduleReconnect()
      }
    }

    function scheduleReconnect() {
      const delay = reconnectDelay.current
      reconnectDelay.current = Math.min(delay * 2, 30_000)
      setTimeout(connect, delay)
    }

    connect()

    return () => {
      unmounted.current = true
      socketRef.current?.close()
      socketRef.current = null
    }
  }, [dispatch])

  const subscribe = useCallback((type: string, handler: EventHandler) => {
    const map = listenersRef.current
    if (!map.has(type)) map.set(type, new Set())
    map.get(type)!.add(handler)
    return () => {
      map.get(type)?.delete(handler)
    }
  }, [])

  const send = useCallback((msg: object) => {
    const ws = socketRef.current
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify(msg))
    }
  }, [])

  return <WSContext.Provider value={{ subscribe, send }}>{children}</WSContext.Provider>
}

export function useWS(): WSContextValue {
  const ctx = useContext(WSContext)
  if (!ctx) throw new Error("useWS must be used inside WSProvider")
  return ctx
}
