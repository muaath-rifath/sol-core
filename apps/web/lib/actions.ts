"use server"

import { solCore } from "@/lib/sol-core"
import type { CursorResponse, ActivityLog } from "@/lib/sol-core"
import { revalidatePath } from "next/cache"

export async function listActivityAction(
  homeId: string,
  roomId: string,
  cursor?: string,
  limit: number = 20,
): Promise<CursorResponse<ActivityLog>> {
  try {
    return await solCore.rooms.activity(homeId, roomId, { cursor, limit })
  } catch {
    return { data: [], has_more: false, next_cursor: null }
  }
}

export async function getDeviceProvisioning(deviceId: string) {
  try {
    return await solCore.devices.provision(deviceId)
  } catch (error) {
    const msg = String(error);
    if (msg.includes("mTLS service not configured")) {
      return null;
    }
    console.error("failed to get device provisioning", error)
    throw new Error(error instanceof Error ? error.message : "Failed to generate certificates")
  }
}

export async function listOTAAttemptsAction(homeId: string, roomId: string, limit: number = 50) {
  try {
    const res = await solCore.rooms.otaAttempts.list(homeId, roomId, limit)
    return { data: res.data }
  } catch (error) {
    console.error("Failed to list OTA attempts:", error)
    return { data: [], error: "Failed to fetch attempts" }
  }
}

export async function retryOTAAction(homeId: string, roomId: string, attemptId: string) {
  try {
    const res = await solCore.rooms.otaAttempts.retry(homeId, roomId, attemptId)
    revalidatePath(`/dashboard/homes/${homeId}/rooms/${roomId}`)
    return { data: res }
  } catch (error) {
    console.error("Failed to retry OTA:", error)
    throw error
  }
}

export async function cancelOTAAction(homeId: string, roomId: string, attemptId: string) {
  try {
    const res = await solCore.rooms.otaAttempts.cancel(homeId, roomId, attemptId)
    revalidatePath(`/dashboard/homes/${homeId}/rooms/${roomId}`)
    return { data: res }
  } catch (error) {
    console.error("Failed to cancel OTA:", error)
    throw error
  }
}
