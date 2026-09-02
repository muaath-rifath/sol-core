"use client"

import { useState, useEffect, useRef } from "react"
import { type FirmwareBuild } from "@/lib/sol-core"
import { buildFirmwareAction, getFirmwareBuildStatusAction } from "@/app/actions"
import { fetchBoards, type Board } from "@/lib/boards"
import { IconBolt, IconLoader2, IconX, IconCheck, IconAlertCircle } from "@tabler/icons-react"

import Portal from "./ui/portal"

export default function FirmwareBuildModal({ templateId, homeId }: { templateId?: string, homeId: string }) {
  const [isOpen, setIsOpen] = useState(false)
  const [boards, setBoards] = useState<Board[]>([])
  const [selectedBoard, setSelectedBoard] = useState<string>("")
  const [search, setSearch] = useState("")
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [activeBuildId, setActiveBuildId] = useState<string | null>(null)
  const [buildStatus, setBuildStatus] = useState<FirmwareBuild | null>(null)
  const [error, setError] = useState<string | null>(null)
  
  const logEndRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (isOpen && boards.length === 0) {
      fetchBoards().then(setBoards)
    }
  }, [isOpen, boards.length])

  useEffect(() => {
    if (activeBuildId) {
      const interval = setInterval(async () => {
        try {
          const status = await getFirmwareBuildStatusAction(activeBuildId)
          setBuildStatus(status)
          if (status.status === "success" || status.status === "failed") {
            setActiveBuildId(null)
          }
        } catch (err) {
          console.error("Error polling build status:", err)
        }
      }, 2000)
      return () => clearInterval(interval)
    }
  }, [activeBuildId])

  useEffect(() => {
    if (logEndRef.current) {
      logEndRef.current.scrollIntoView({ behavior: "smooth" })
    }
  }, [buildStatus?.logs])

  const filteredBoards = boards.filter(b => 
    b.name.toLowerCase().includes(search.toLowerCase()) || 
    b.family.toLowerCase().includes(search.toLowerCase())
  )

  async function handleBuild() {
    if (!selectedBoard || !templateId) return
    
    setIsSubmitting(true)
    setError(null)
    setBuildStatus(null)
    
    try {
      const board = boards.find(b => b.id === selectedBoard)
      if (!board) throw new Error("Selected board not found")
      
      const { id } = await buildFirmwareAction(templateId || "switch", board.family)
      setActiveBuildId(id)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to start build")
    } finally {
      setIsSubmitting(false)
    }
  }

  if (!isOpen) {
    return (
      <button
        onClick={() => setIsOpen(true)}
        className="btn-primary flex items-center justify-center gap-2 px-4 py-2 text-sm font-semibold"
      >
        <IconBolt size={16} />
        Build Firmware
      </button>
    )
  }

  return (
    <Portal>
      <div className="fixed inset-0 z-50 flex items-center justify-center bg-stone-900/40 p-4 backdrop-blur-sm">
        <div className="bg-clay-canvas w-full max-w-2xl overflow-hidden rounded-[2.5rem] border border-white/60 shadow-2xl">
          <div className="flex items-center justify-between border-b border-white/40 p-6">
            <h2 className="font-display text-xl font-bold text-on-surface">Build Firmware</h2>
            <button onClick={() => setIsOpen(false)} className="text-on-surface-variant hover:text-on-surface">
              <IconX size={24} />
            </button>
          </div>

          <div className="p-6 space-y-4">
            {!buildStatus ? (
              <>
                <div>
                  <label className="text-xs font-semibold uppercase tracking-wider text-on-surface-variant">Template ID</label>
                  <input
                    type="text"
                    readOnly
                    value={templateId || "switch"}
                    className="clay-inset mt-1 w-full rounded-xl border border-white/55 bg-surface-container-low px-4 py-2 text-sm text-on-surface"
                  />
                </div>

                <div>
                  <label className="text-xs font-semibold uppercase tracking-wider text-on-surface-variant">Select Board</label>
                  <div className="mt-1 space-y-2">
                    <input
                      type="text"
                      placeholder="Search boards (e.g. DevKit, S3, C3...)"
                      value={search}
                      onChange={(e) => setSearch(e.target.value)}
                      className="clay-inset w-full rounded-xl border border-white/55 px-4 py-2 text-sm text-on-surface"
                    />
                    <div className="clay-inset max-h-48 overflow-y-auto rounded-xl border border-white/55 bg-surface-container-low p-1">
                      {boards.length === 0 ? (
                        <div className="flex items-center justify-center py-8">
                          <IconLoader2 className="animate-spin text-primary" size={24} />
                        </div>
                      ) : (
                        filteredBoards.map(board => (
                          <button
                            key={board.id}
                            onClick={() => setSelectedBoard(board.id)}
                            className={`w-full rounded-lg px-3 py-2 text-left text-sm transition-colors ${selectedBoard === board.id
                                ? "bg-primary text-on-primary shadow-md"
                                : "text-on-surface hover:bg-surface-container-high"
                              }`}
                          >
                            <div className="font-semibold">{board.name}</div>
                            <div className={`text-xs ${selectedBoard === board.id ? "text-on-primary/80" : "text-on-surface-variant"}`}>
                              Target: {board.family}
                            </div>
                          </button>
                        ))
                      )}
                    </div>
                  </div>
                </div>

                {error && (
                  <p className="rounded-xl bg-error-container px-4 py-2 text-xs text-on-error-container">
                    {error}
                  </p>
                )}

                <button
                  disabled={!selectedBoard || isSubmitting}
                  onClick={handleBuild}
                  className="btn-primary flex w-full items-center justify-center gap-2 py-3 text-sm font-bold disabled:opacity-50"
                >
                  {isSubmitting ? <IconLoader2 className="animate-spin" size={18} /> : <IconBolt size={18} />}
                  Start Build
                </button>
              </>
            ) : (
              <div className="space-y-4">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    {buildStatus.status === "building" || buildStatus.status === "queued" ? (
                      <IconLoader2 className="animate-spin text-primary" size={20} />
                    ) : buildStatus.status === "success" ? (
                      <IconCheck className="text-emerald-500" size={20} />
                    ) : (
                      <IconAlertCircle className="text-error" size={20} />
                    )}
                    <span className="font-semibold capitalize text-on-surface">Status: {buildStatus.status}</span>
                  </div>
                  {buildStatus.status === "success" && (
                    <button
                      onClick={() => window.location.reload()}
                      className="text-xs font-bold text-primary hover:underline"
                    >
                      Reload to see version
                    </button>
                  )}
                </div>

                <div className="relative h-64 overflow-hidden rounded-2xl border border-white/55 bg-stone-950 font-mono text-xs text-stone-100">
                  <div className="absolute inset-0 overflow-y-auto p-4">
                    <pre className="whitespace-pre-wrap">{buildStatus.logs}</pre>
                    <div ref={logEndRef} />
                  </div>
                </div>

                {(buildStatus.status === "success" || buildStatus.status === "failed") && (
                  <button
                    onClick={() => {
                      setBuildStatus(null)
                      setActiveBuildId(null)
                    }}
                    className="btn-outline w-full py-2 text-sm font-semibold"
                  >
                    Close & New Build
                  </button>
                )}
              </div>
            )}
          </div>
        </div>
      </div>
    </Portal>
  )
}
