"use server"

import { solCore, type Home } from "@/lib/sol-core"
import { auth, signOut } from "@/auth"

export interface CreateRoomInput {
  homeId: string
  name: string
  floor?: number
}

export async function createHomeAction(name: string): Promise<Home> {
  return solCore.homes.create({ name })
}

export async function createRoomAction(input: CreateRoomInput) {
  return solCore.rooms.create(input.homeId, {
    name: input.name,
    floor: input.floor,
  })
}

export async function buildFirmwareAction(templateId: string, targetBoard: string) {
  return solCore.firmware.build(templateId, targetBoard)
}

export async function getFirmwareBuildStatusAction(buildId: string) {
  const build = await solCore.firmware.getBuild(buildId)
  // Prevent Next.js Server Action payload size limit crash (e.g. 1MB)
  // by truncating massive compiler logs.
  if (build.logs && build.logs.length > 100000) {
    build.logs =
      "...(earlier logs truncated for performance)...\n" +
      build.logs.slice(-100000)
  }
  return build
}

export async function federatedLogout() {
  const session = await auth()
  const issuer = process.env.AUTH_ZITADEL_ISSUER
  const idToken = session?.idToken

  if (issuer && idToken) {
    const endSessionUrl = new URL(`${issuer}/oidc/v1/end_session`)
    endSessionUrl.searchParams.set("id_token_hint", idToken)
    endSessionUrl.searchParams.set("post_logout_redirect_uri", process.env.AUTH_URL || "http://localhost:3000")
    await signOut({ redirectTo: endSessionUrl.toString() })
  } else {
    await signOut({ redirectTo: "/" })
  }
}
