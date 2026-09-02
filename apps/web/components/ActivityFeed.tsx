"use client"

import { useCallback, useEffect, useRef, useState } from "react"
import { listActivityAction } from "@/lib/actions"
import type { ActivityLog } from "@/lib/sol-core"
import {
  IconBolt,
  IconCloudDownload,
  IconLoader2,
  IconPower,
  IconWifiOff,
} from "@tabler/icons-react"

type Props = {
  homeId: string
  roomId: string
  initialData: ActivityLog[]
  initialNextCursor: string | null
  initialHasMore: boolean
}

function activityIcon(badge: string) {
  if (badge === "Online" || badge === "Success") return <IconCloudDownload size={17} />
  if (badge === "On") return <IconBolt size={17} />
  if (badge === "Off") return <IconPower size={17} />
  return <IconWifiOff size={17} />
}

function iconColor(badge: string) {
  if (badge === "Online" || badge === "Success" || badge === "On") return "text-tertiary"
  if (badge === "Off") return "text-outline"
  return "text-error"
}

export default function ActivityFeed({
  homeId,
  roomId,
  initialData,
  initialNextCursor,
  initialHasMore,
}: Props) {
  const [logs, setLogs] = useState<ActivityLog[]>(initialData)
  const [nextCursor, setNextCursor] = useState<string | null>(initialNextCursor)
  const [hasMore, setHasMore] = useState(initialHasMore)
  const [isLoading, setIsLoading] = useState(false)
  const sentinelRef = useRef<HTMLDivElement | null>(null)

  const loadMore = useCallback(async () => {
    if (isLoading || !hasMore) return
    setIsLoading(true)
    const res = await listActivityAction(homeId, roomId, nextCursor ?? undefined, 20)
    setLogs((prev) => [...prev, ...res.data])
    setNextCursor(res.next_cursor)
    setHasMore(res.has_more)
    setIsLoading(false)
  }, [homeId, roomId, nextCursor, isLoading, hasMore])

  useEffect(() => {
    const el = sentinelRef.current
    if (!el) return
    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting) loadMore()
      },
      { threshold: 1.0 },
    )
    observer.observe(el)
    return () => observer.unobserve(el)
  }, [loadMore])

  return (
    <div className="w-full space-y-3">
      {logs.length === 0 && !isLoading && (
        <p className="text-sm text-outline px-2">No activity yet.</p>
      )}
      {logs.map((log, index) => (
        <div
          key={index}
          className="flex w-full items-center gap-3 overflow-hidden rounded-xl border border-white/40 bg-surface p-3 shadow-[inset_2px_2px_6px_rgba(87,66,62,0.03),inset_-2px_-2px_6px_rgba(255,255,255,0.9)]"
        >
          <div
            className={`flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-surface-container-high ${iconColor(log.badge_text)}`}
          >
            {activityIcon(log.badge_text)}
          </div>
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-medium text-on-surface">{log.title}</p>
            <p className="truncate mt-0.5 text-xs text-outline">
              {new Date(log.timestamp).toLocaleString()} • {log.description}
            </p>
          </div>
          <span
            className={`shrink-0 rounded-full border border-white/55 px-2 py-1 text-[10px] font-medium sm:px-3 sm:text-xs ${log.badge_color}`}
          >
            {log.badge_text}
          </span>
        </div>
      ))}

      <div ref={sentinelRef} className="flex h-10 items-center justify-center">
        {isLoading && (
          <IconLoader2 size={18} className="animate-spin text-outline" />
        )}
        {!hasMore && logs.length > 0 && (
          <p className="text-xs text-outline">All caught up.</p>
        )}
      </div>
    </div>
  )
}
