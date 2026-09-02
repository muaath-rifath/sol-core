"use client"

import { IconAlertTriangle } from "@tabler/icons-react"
import { useState, useTransition } from "react"

import Portal from "./ui/portal"

type Role = "admin" | "member"

interface Props {
  userId: string
  currentRole: Role
  memberLabel: string
  action: (formData: FormData) => Promise<void>
}

export default function MemberRoleSelect({ userId, currentRole, memberLabel, action }: Props) {
  const [pending, setPending] = useState<Role | null>(null)
  const [selectKey, setSelectKey] = useState(0)
  const [isSubmitting, startTransition] = useTransition()

  function handleConfirm() {
    if (!pending) return
    const fd = new FormData()
    fd.set("user_id", userId)
    fd.set("role", pending)
    startTransition(async () => {
      await action(fd)
      setPending(null)
    })
  }

  function handleCancel() {
    if (isSubmitting) return
    setPending(null)
    setSelectKey((k) => k + 1)
  }

  return (
    <>
      <select
        key={selectKey}
        defaultValue={currentRole}
        onChange={(event) => {
          const value = event.target.value as Role
          if (value !== currentRole) setPending(value)
        }}
        className="clay-inset rounded-lg border border-white/55 bg-surface-container-low px-2 py-1.5 text-xs font-semibold text-on-surface"
      >
        <option value="admin">Admin</option>
        <option value="member">Member</option>
      </select>

      {pending ? (
        <Portal>
          <div
            className="fixed inset-0 z-[80] flex items-center justify-center bg-black/35 p-4"
            onClick={handleCancel}
          >
            <div
              className="w-full max-w-sm rounded-3xl border border-white/65 bg-surface-container p-6 shadow-[16px_16px_34px_rgba(27,28,25,0.18),-16px_-16px_34px_rgba(255,255,255,0.88)]"
              onClick={(event) => event.stopPropagation()}
            >
              <div className="mb-3 flex items-center gap-2">
                <IconAlertTriangle size={20} className="text-tertiary" />
                <h3 className="font-display text-lg font-semibold text-on-surface">
                  Change role?
                </h3>
              </div>
              <p className="mb-5 text-sm text-on-surface-variant">
                Set <span className="font-semibold text-on-surface">{memberLabel}</span> as{" "}
                <span className="font-semibold text-on-surface">{pending}</span>? They&apos;re
                currently a <span className="font-semibold text-on-surface">{currentRole}</span>.
              </p>
              <div className="flex gap-3">
                <button
                  type="button"
                  onClick={handleCancel}
                  disabled={isSubmitting}
                  className="flex-1 rounded-xl border border-outline-variant px-3 py-2 text-sm font-medium text-on-surface-variant transition hover:bg-surface disabled:opacity-60"
                >
                  Cancel
                </button>
                <button
                  type="button"
                  onClick={handleConfirm}
                  disabled={isSubmitting}
                  className="btn-primary flex-1 rounded-xl px-3 py-2 text-sm font-semibold disabled:opacity-60"
                >
                  {isSubmitting ? "Saving…" : "Confirm"}
                </button>
              </div>
            </div>
          </div>
        </Portal>
      ) : null}
    </>
  )
}
