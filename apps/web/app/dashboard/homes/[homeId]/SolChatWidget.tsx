"use client"

import { useCallback, useEffect, useRef, useState } from "react"

interface Message {
  id: string
  role: "user" | "assistant"
  content: string
  streaming?: boolean
}

// Maximum user+assistant pairs to replay into a fresh Azure session on reconnect.
// Older turns beyond this are silently dropped.
const MAX_CONTEXT_TURNS = 10

const SOL_WS_BASE = (process.env.NEXT_PUBLIC_SOL_CORE_WS_URL ?? "").replace(/\/ws$/, "")

export function SolChatWidget({ homeId }: { homeId: string }) {
  const [messages, setMessages] = useState<Message[]>([])
  const [connected, setConnected] = useState(false)
  const [input, setInput] = useState("")
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectDelay = useRef(250)
  const unmounted = useRef(false)
  const pendingAssistantId = useRef<string | null>(null)
  const bottomRef = useRef<HTMLDivElement>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  // Always-current snapshot of messages for use inside async/closure callbacks.
  // Using a ref avoids stale closures without adding messages to effect deps.
  const messagesRef = useRef<Message[]>([])

  useEffect(() => {
    messagesRef.current = messages
  }, [messages])

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" })
  }, [messages])

  const dispatch = useCallback((event: Record<string, unknown>) => {
    const type = event.type as string

    // Sent by the backend when a tool call is intercepted mid-stream.
    // Removes the partial text bubble the model generated before calling the tool.
    if (type === "sol.discard_last") {
      setMessages((prev) => {
        const last = prev[prev.length - 1]
        if (last?.role === "assistant") return prev.slice(0, -1)
        return prev
      })
      pendingAssistantId.current = null
      return
    }

    if (type === "response.created") {
      pendingAssistantId.current = null
      return
    }

    if (type === "response.text.delta" || type === "response.audio_transcript.delta") {
      const delta = (event.delta as string) ?? ""
      setMessages((prev) => {
        const last = prev[prev.length - 1]
        if (last && last.id === pendingAssistantId.current) {
          return [...prev.slice(0, -1), { ...last, content: last.content + delta }]
        }
        const id = crypto.randomUUID()
        pendingAssistantId.current = id
        return [...prev, { id, role: "assistant", content: delta, streaming: true }]
      })
      return
    }

    if (type === "response.text.done") {
      const text = (event.text as string) ?? ""
      setMessages((prev) => {
        const last = prev[prev.length - 1]
        if (last && last.id === pendingAssistantId.current) {
          return [...prev.slice(0, -1), { ...last, content: text, streaming: false }]
        }
        return prev
      })
      return
    }

    if (type === "response.done" || type === "response.output_item.done") {
      setMessages((prev) => {
        const last = prev[prev.length - 1]
        if (last && last.streaming) {
          return [...prev.slice(0, -1), { ...last, streaming: false }]
        }
        return prev
      })
      pendingAssistantId.current = null
    }
  }, [])

  useEffect(() => {
    let cancelled = false
    unmounted.current = false

    async function connect() {
      if (cancelled || !SOL_WS_BASE) return
      try {
        const res = await fetch("/api/ws-token", { cache: "no-store" })
        if (cancelled) return
        if (!res.ok) throw new Error("token fetch failed")
        const { token } = (await res.json()) as { token?: string }
        if (cancelled) return
        if (!token) throw new Error("no token")

        const url = `${SOL_WS_BASE}/api/v1/homes/${homeId}/chat/ws?token=${encodeURIComponent(token)}`
        const ws = new WebSocket(url)
        if (cancelled) { ws.close(); return }
        wsRef.current = ws

        ws.onopen = () => {
          // Replay up to MAX_CONTEXT_TURNS exchanges into the fresh Azure session so
          // the model has context from this browser session. messagesRef is always
          // current; the state is React-only and clears on page refresh, so this
          // session history is naturally temporary.
          const history = messagesRef.current
            .filter((m) => !m.streaming)
            .slice(-(MAX_CONTEXT_TURNS * 2))
          for (const msg of history) {
            ws.send(JSON.stringify({
              type: "conversation.item.create",
              item: {
                type: "message",
                role: msg.role,
                content: [{
                  type: msg.role === "user" ? "input_text" : "text",
                  text: msg.content,
                }],
              },
            }))
          }

          reconnectDelay.current = 250
          setConnected(true)
        }
        ws.onmessage = (ev) => {
          if (wsRef.current !== ws) return
          try {
            dispatch(JSON.parse(ev.data as string) as Record<string, unknown>)
          } catch {
            // ignore malformed frames
          }
        }
        ws.onclose = () => {
          if (wsRef.current === ws) {
            wsRef.current = null
            setConnected(false)
            if (!cancelled) scheduleReconnect()
          }
        }
        ws.onerror = () => ws.close()
      } catch {
        if (!cancelled) scheduleReconnect()
      }
    }

    function scheduleReconnect() {
      const delay = reconnectDelay.current
      reconnectDelay.current = Math.min(delay * 2, 30_000)
      setTimeout(() => { if (!cancelled) connect() }, delay)
    }

    connect()
    return () => {
      cancelled = true
      unmounted.current = true
      wsRef.current?.close()
      wsRef.current = null
    }
  }, [homeId, dispatch])

  const sendMessage = useCallback(() => {
    const text = input.trim()
    const ws = wsRef.current
    if (!text || !ws || ws.readyState !== WebSocket.OPEN) return

    const id = crypto.randomUUID()
    setMessages((prev) => [...prev, { id, role: "user", content: text }])
    setInput("")
    pendingAssistantId.current = null
    textareaRef.current?.focus()

    ws.send(
      JSON.stringify({
        type: "conversation.item.create",
        item: { type: "message", role: "user", content: [{ type: "input_text", text }] },
      }),
    )
    ws.send(JSON.stringify({ type: "response.create" }))
  }, [input])

  function onKeyDown(e: React.KeyboardEvent<HTMLTextAreaElement>) {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault()
      sendMessage()
    }
  }

  return (
    <article className="flex flex-col overflow-hidden rounded-xl border border-white/40 bg-surface-container shadow-[12px_12px_24px_rgba(27,28,25,0.06),-12px_-12px_24px_rgba(255,255,255,0.9),inset_2px_2px_4px_rgba(255,255,255,0.8),inset_-2px_-2px_4px_rgba(27,28,25,0.02)]" style={{ minHeight: "420px" }}>
      <div className="flex items-center justify-between border-b border-white/40 px-5 py-4">
        <div>
          <h3 className="text-sm font-semibold text-on-surface">Sol</h3>
          <p className="text-xs text-on-surface-variant">AI Assistant</p>
        </div>
        <div className="flex items-center gap-1.5">
          <span className={`h-2 w-2 rounded-full ${connected ? "bg-green-500" : "bg-surface-variant"}`} />
          <span className="text-xs text-on-surface-variant">{connected ? "Connected" : "Connecting…"}</span>
        </div>
      </div>

      <div className="flex flex-1 flex-col gap-3 overflow-y-auto px-4 py-4">
        {messages.length === 0 ? (
          <div className="flex flex-1 flex-col items-center justify-center gap-1 text-center">
            <p className="text-sm font-semibold text-on-surface">Ask Sol anything</p>
            <p className="text-xs text-on-surface-variant">Try &ldquo;turn on the kitchen light&rdquo;</p>
          </div>
        ) : (
          messages.map((msg) => (
            <div key={msg.id} className={`flex ${msg.role === "user" ? "justify-end" : "justify-start"}`}>
              <div
                className={`max-w-[80%] rounded-2xl px-3.5 py-2 text-sm leading-relaxed ${
                  msg.role === "user"
                    ? "bg-primary text-on-primary"
                    : "bg-surface-container-high text-on-surface"
                }`}
              >
                {msg.content}
                {msg.streaming && (
                  <span className="ml-1 inline-block h-3 w-0.5 animate-pulse bg-current opacity-70" />
                )}
              </div>
            </div>
          ))
        )}
        <div ref={bottomRef} />
      </div>

      <div className="border-t border-white/40 px-4 py-3">
        <div className="flex items-end gap-2">
          <textarea
            ref={textareaRef}
            rows={1}
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={onKeyDown}
            disabled={!connected}
            placeholder={connected ? "Message Sol…" : "Connecting…"}
            className="flex-1 resize-none rounded-xl border border-white/60 bg-surface-container-low px-3 py-2 text-sm text-on-surface outline-none placeholder:text-on-surface-variant/60 disabled:opacity-50"
            style={{ minHeight: "2.5rem", maxHeight: "7rem" }}
          />
          <button
            onClick={sendMessage}
            disabled={!connected || !input.trim()}
            className="flex h-10 w-10 items-center justify-center rounded-xl bg-primary text-on-primary transition-opacity disabled:opacity-40"
            aria-label="Send"
          >
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" className="h-5 w-5">
              <path d="M3.478 2.405a.75.75 0 00-.926.94l2.432 7.905H13.5a.75.75 0 010 1.5H4.984l-2.432 7.905a.75.75 0 00.926.94 60.519 60.519 0 0018.445-8.986.75.75 0 000-1.218A60.517 60.517 0 003.478 2.405z" />
            </svg>
          </button>
        </div>
      </div>
    </article>
  )
}
