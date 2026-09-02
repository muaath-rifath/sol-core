"use client"

import {
  IconArmchair,
  IconBed,
  IconChefHat,
  IconDeviceDesktop,
} from "@tabler/icons-react"
import Link from "next/link"
import { usePathname } from "next/navigation"
import type { ForwardRefExoticComponent, RefAttributes } from "react"
import type { IconProps } from "@tabler/icons-react"
import { cn } from "@/lib/utils"

type TablerIcon = ForwardRefExoticComponent<IconProps & RefAttributes<SVGSVGElement>>

interface NavRoom {
  id: string
  name: string
}

interface DashboardSidebarNavProps {
  homeId: string
  navRooms: NavRoom[]
}

function roomIcon(name: string): TablerIcon {
  const lower = name.toLowerCase()
  if (lower.includes("living")) return IconArmchair
  if (lower.includes("kitchen")) return IconChefHat
  if (lower.includes("bed")) return IconBed
  if (lower.includes("bath")) return IconBed
  if (lower.includes("garage")) return IconDeviceDesktop
  if (lower.includes("office")) return IconDeviceDesktop
  return IconArmchair
}

export default function DashboardSidebarNav({ homeId, navRooms }: DashboardSidebarNavProps) {
  const pathname = usePathname()
  const dashboardPath = `/dashboard/homes/${homeId}`

  return (
    <ul className="flex flex-1 flex-col gap-2 text-base font-semibold">
      <li>
        <Link
          href={dashboardPath}
          className={cn(
            "m-2 flex items-center rounded-[24px] px-4 py-4 transition-colors duration-150",
            pathname === dashboardPath
              ? "bg-primary-fixed text-primary shadow-[inset_1px_1px_2px_rgba(255,255,255,0.9),4px_4px_10px_rgba(0,0,0,0.07)]"
              : "text-stone-600 hover:bg-white/60",
          )}
        >
          <IconDeviceDesktop size={18} className="mr-2" />
          Dashboard
        </Link>
      </li>


{navRooms.length === 0 ? (
        <li>
          <span className="m-2 flex items-center rounded-[24px] p-4 text-stone-400">No rooms yet</span>
        </li>
      ) : (
        navRooms.map((room) => {
          const RoomNavIcon = roomIcon(room.name)
          const roomPath = `${dashboardPath}/rooms/${room.id}`
          const isActive = pathname === roomPath || pathname.startsWith(`${roomPath}/`)

          return (
            <li key={room.id}>
              <Link
                href={roomPath}
                className={cn(
                  "m-2 flex items-center rounded-[24px] px-4 py-4 transition-colors duration-150",
                  isActive
                    ? "bg-primary-fixed text-primary shadow-[inset_1px_1px_2px_rgba(255,255,255,0.9),4px_4px_10px_rgba(0,0,0,0.07)]"
                    : "text-stone-600 hover:bg-white/60",
                )}
              >
                <RoomNavIcon size={18} className="mr-2" />
                {room.name}
              </Link>
            </li>
          )
        })
      )}
    </ul>
  )
}
