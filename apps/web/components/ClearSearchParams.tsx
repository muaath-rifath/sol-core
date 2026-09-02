"use client"

import { useEffect } from "react"
import { useRouter, usePathname } from "next/navigation"

/**
 * Strips the given query param keys from the URL via router.replace
 * so that flash messages disappear on refresh.
 */
export default function ClearSearchParams({ keys = [] }: { keys?: string[] }) {
    const router = useRouter()
    const pathname = usePathname()

    useEffect(() => {
        const url = new URL(window.location.href)
        let changed = false
        for (const key of keys) {
            if (url.searchParams.has(key)) {
                url.searchParams.delete(key)
                changed = true
            }
        }
        if (changed) {
            router.replace(pathname + (url.searchParams.size ? `?${url.searchParams.toString()}` : ""), {
                scroll: false,
            })
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [])

    return null
}
