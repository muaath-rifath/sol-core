"use client"

import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import { useRouter } from "next/navigation"
import { useEffect, useRef, useState } from "react"

interface Props {
  name: string | null | undefined
  email: string | null | undefined
  image: string | null | undefined
  accountSettingsUrl: string
  manageMembersUrl?: string
  signOutAction: () => Promise<void>
}

function initials(name: string | null | undefined): string {
  if (!name) return "?"
  return name
    .split(/\s+/)
    .slice(0, 2)
    .map((w) => w[0]?.toUpperCase() ?? "")
    .join("")
}

export default function UserMenu({ name, email, image, accountSettingsUrl, manageMembersUrl, signOutAction }: Props) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  const router = useRouter()

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    if (open) document.addEventListener("mousedown", handleClick)
    return () => document.removeEventListener("mousedown", handleClick)
  }, [open])

  return (
    <div ref={ref} className="relative">
      <button
        onClick={() => setOpen((v) => !v)}
        className="rounded-full transition ring-2 ring-transparent hover:ring-primary/25 focus:outline-none"
      >
        <Avatar>
          <AvatarImage src={image ?? undefined} alt={name ?? "User"} />
          <AvatarFallback>{initials(name)}</AvatarFallback>
        </Avatar>
      </button>

      {open ? (
        <div className="absolute right-0 top-full z-50 mt-2 w-56 overflow-hidden rounded-2xl border border-white/60 bg-surface-container-low shadow-[8px_8px_18px_rgba(87,66,62,0.12),-8px_-8px_18px_rgba(255,255,255,0.92)]">
          <div className="border-b border-outline-variant/45 px-4 py-3">
            <p className="truncate text-sm font-semibold text-on-surface">{name ?? "User"}</p>
            {email ? <p className="truncate text-xs text-on-surface-variant">{email}</p> : null}
          </div>
          <div className="p-1">
            <button
              className="flex w-full items-center rounded-xl px-3 py-2 text-sm text-on-surface transition hover:bg-surface"
              onClick={() => { setOpen(false); router.push(accountSettingsUrl) }}
            >
              Account settings
            </button>
            {manageMembersUrl ? (
              <button
                className="flex w-full items-center rounded-xl px-3 py-2 text-sm text-on-surface transition hover:bg-surface"
                onClick={() => { setOpen(false); router.push(manageMembersUrl) }}
              >
                Manage members
              </button>
            ) : null}
            <div className="my-1 border-t border-outline-variant/45" />
            <form action={signOutAction}>
              <button
                type="submit"
                className="flex w-full items-center rounded-xl px-3 py-2 text-sm text-error transition hover:bg-error-container"
              >
                Sign out
              </button>
            </form>
          </div>
        </div>
      ) : null}
    </div>
  )
}
