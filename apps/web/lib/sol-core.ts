// auth is dynamically imported in solFetch on the server side

const SOL_CORE_URL = process.env.SOL_CORE_URL!

async function fetchWithRetry(url: string, init: RequestInit, retries = 3): Promise<Response> {
  let lastError: unknown
  for (let attempt = 0; attempt < retries; attempt++) {
    try {
      return await fetch(url, init)
    } catch (err) {
      lastError = err
      if (attempt < retries - 1) {
        await new Promise((res) => setTimeout(res, 100 * 2 ** attempt))
      }
    }
  }
  throw lastError
}

function buildQuery(params: Record<string, string | number | undefined>): string {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === "") {
      continue
    }
    search.set(key, String(value))
  }
  const encoded = search.toString()
  return encoded ? `?${encoded}` : ""
}

async function getErrorMessage(response: Response): Promise<string> {
  const contentType = response.headers.get("Content-Type") || ""
  const text = await response.text()

  if (!text) {
    return `${response.status} ${response.statusText}`
  }

  if (contentType.includes("application/json")) {
    try {
      const parsed = JSON.parse(text) as { error?: string }
      if (parsed.error) return parsed.error
    } catch {
      // Fallback
    }
  }

  // If it's HTML (likely from a proxy like Traefik or an OIDC provider), don't return the whole thing
  if (text.trim().startsWith("<!DOCTYPE") || text.trim().startsWith("<html")) {
    return `Server Error: ${response.status} ${response.statusText}`
  }

  // Truncate long non-JSON responses
  return text.length > 200 ? `${text.substring(0, 200)}...` : text
}

async function solFetch(path: string, init: RequestInit = {}): Promise<Response> {
  // Determine if we are on the server (where `auth` is available) or client.
  let session: { accessToken?: string } | null = null
  if (typeof window === "undefined") {
    // Server side: dynamically import auth to avoid bundling it on the client.
    const { auth } = await import("@/auth")
    session = await auth()
  }
  // On the client, we skip auth and rely on cookies/session handling by the backend.

  const headers = new Headers(init.headers)
  if (!headers.has("Content-Type") && init.body && !(init.body instanceof FormData)) {
    headers.set("Content-Type", "application/json")
  }
  if (session?.accessToken) {
    headers.set("Authorization", `Bearer ${session.accessToken}`)
  }

  const response = await fetchWithRetry(`${SOL_CORE_URL}${path}`, {
    ...init,
    cache: "no-store",
    headers,
  })

  if (!response.ok) {
    throw new Error(await getErrorMessage(response))
  }
  return response
}

async function solPublicFetch(path: string, init: RequestInit = {}): Promise<Response> {
  const headers = new Headers(init.headers)
  if (!headers.has("Content-Type") && init.body && !(init.body instanceof FormData)) {
    headers.set("Content-Type", "application/json")
  }

  const response = await fetchWithRetry(`${SOL_CORE_URL}${path}`, {
    ...init,
    cache: "no-store",
    headers,
  })

  if (!response.ok) {
    throw new Error(await getErrorMessage(response))
  }
  return response
}

async function safeJson<T>(response: Response, defaultValue: T): Promise<T> {
  if (response.status === 204) return defaultValue
  const text = await response.text()
  if (!text) return defaultValue
  try {
    return JSON.parse(text) as T
  } catch {
    return defaultValue
  }
}

async function jsonOrNull<T>(response: Response): Promise<T | null> {
  return safeJson<T | null>(response, null)
}

async function collectCursorPages<T>(
  fetchPage: (cursor?: string) => Promise<CursorResponse<T>>,
): Promise<T[]> {
  const data: T[] = []
  let cursor: string | undefined
  do {
    const page = await fetchPage(cursor)
    data.push(...page.data)
    cursor = page.has_more ? (page.next_cursor ?? undefined) : undefined
  } while (cursor)
  return data
}

export type InvitationStatus = "pending" | "accepted" | "declined" | "expired"
export type MemberRole = "owner" | "admin" | "member"

export interface CursorResponse<T> {
  data: T[]
  next_cursor: string | null
  has_more: boolean
}

interface RawCursorResponse<T> {
  data?: T[] | null
  next_cursor?: string | null
  has_more?: boolean
}

function normalizeCursorResponse<T>(raw: RawCursorResponse<T>): CursorResponse<T> {
  return {
    data: Array.isArray(raw.data) ? raw.data : [],
    next_cursor: raw.next_cursor ?? null,
    has_more: Boolean(raw.has_more),
  }
}

export interface Home {
  id: string
  name: string
  owner_id: string
  my_role?: MemberRole
  member_count?: number
  created_at: string
  updated_at: string
}

export interface HomeMember {
  home_id: string
  user_id: string
  role: MemberRole
  invited_by: string | null
  joined_at: string
  user_email: string
  user_name: string
}

export interface HomeInvitation {
  id: string
  home_id: string
  inviter_id: string
  invitee_email: string
  invitee_is_user: boolean
  token?: string
  status: InvitationStatus
  expires_at: string
  created_at: string
}

export interface InvitationDetail {
  id: string
  home_id: string
  home_name: string
  inviter_id: string
  inviter_name: string
  invitee_email: string
  invitee_is_user: boolean
  status: InvitationStatus
  expires_at: string
  created_at: string
}

export interface Room {
  id: string
  home_id: string
  name: string
  floor?: number
  can_manage: boolean
  metadata?: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface RoomDevice {
  id: string
  name: string
  type: string
  room_id: string
  state: Record<string, unknown>
  metadata: Record<string, string>
  firmware_id?: string
  online: boolean
  created_at: string
  updated_at: string
}

export interface Appliance {
  id: string
  device_id: string
  room_id?: string
  name: string
  type: string
  channel?: number
  gpio_pin?: number
  active_low: boolean
  state: Record<string, unknown>
  created_at: string
  updated_at: string
}

export type PermissionScopeType = "room" | "device" | "appliance"

export interface PermissionScopeRef {
  type: PermissionScopeType
  id: string
}

export interface PermissionTreeAppliance {
  id: string
  name: string
  type: string
  channel?: number
  granted_directly: boolean
}

export interface PermissionTreeDevice {
  id: string
  name: string
  granted_directly: boolean
  appliances: PermissionTreeAppliance[]
}

export interface PermissionTreeRoom {
  id: string
  name: string
  granted_directly: boolean
  can_manage_devices: boolean
  devices: PermissionTreeDevice[]
}

export interface PermissionTree {
  home_id: string
  user_id: string
  role: MemberRole
  all_access: boolean
  rooms: PermissionTreeRoom[]
}

export interface FirmwareVersion {
  id: string
  template_id: string
  version: string
  bootloader_key: string
  partition_key: string
  app_key: string
  source_key?: string
  size_bytes: number | null
  built_at?: string
  created_at: string
}

export interface FirmwareBuild {
  id: string
  template_id: string
  target_board: string
  status: "queued" | "building" | "success" | "failed"
  logs: string
  firmware_version_id?: string
  created_at: string
  updated_at: string
}

export type DeviceProvisioning = {
  CertificatePEM: string
  PrivateKeyPEM: string
}

export type CommandResponse = Record<string, unknown>
export type Automation = Record<string, unknown>

export interface TelemetryPoint {
  device_id: string
  timestamp: string
  data: Record<string, unknown>
}

export interface ActivityLog {
  room_id: string
  timestamp: string
  title: string
  description: string
  badge_text: string
  badge_color: string
}

export type OTAAttemptStatus =
  | "initiated"
  | "acknowledged"
  | "downloading"
  | "verifying"
  | "updating"
  | "cancelling"
  | "cancelled"
  | "timed_out"
  | "updated"
  | "failed"

export interface OTAAttempt {
  id: string
  device_id: string
  room_id: string
  home_id: string
  firmware_version_id: string
  requested_by?: string
  request_id: string
  status: OTAAttemptStatus
  progress_pct: number
  logs: string
  error_text?: string
  started_at: string
  finished_at?: string
  created_at: string
  updated_at: string
}

export const solCore = {
  homes: {
    list: (params?: { cursor?: string; limit?: number }) =>
      solFetch(`/api/v1/homes${buildQuery(params ?? {})}`).then(async (r) =>
        normalizeCursorResponse<Home>(await safeJson<RawCursorResponse<Home>>(r, {})),
      ),
    get: (id: string) =>
      solFetch(`/api/v1/homes/${id}`).then((r) => safeJson<Home>(r, {} as Home)),
    create: (body: { name: string }) =>
      solFetch("/api/v1/homes", {
        method: "POST",
        body: JSON.stringify(body),
      }).then((r) => safeJson<Home>(r, {} as Home)),
    listMembers: (id: string, params?: { cursor?: string; limit?: number }) =>
      solFetch(`/api/v1/homes/${id}/members${buildQuery(params ?? {})}`).then(async (r) =>
        normalizeCursorResponse<HomeMember>(await safeJson<RawCursorResponse<HomeMember>>(r, {})),
      ),
    inviteByEmail: (id: string, body: { email: string }) =>
      solFetch(`/api/v1/homes/${id}/invitations`, {
        method: "POST",
        body: JSON.stringify(body),
      }).then((r) => safeJson<HomeInvitation>(r, {} as HomeInvitation)),
    listInvitations: (
      id: string,
      params?: { status?: InvitationStatus; cursor?: string; limit?: number },
    ) =>
      solFetch(`/api/v1/homes/${id}/invitations${buildQuery(params ?? {})}`).then(async (r) =>
        normalizeCursorResponse<HomeInvitation>(await safeJson<RawCursorResponse<HomeInvitation>>(r, {})),
      ),
    cancelInvitation: (homeID: string, invitationID: string) =>
      solFetch(`/api/v1/homes/${homeID}/invitations/${invitationID}`, {
        method: "DELETE",
      }).then((r) => jsonOrNull(r)),
    delete: (id: string) =>
      solFetch(`/api/v1/homes/${id}`, { method: "DELETE" }).then((r) => jsonOrNull(r)),
    transferOwnership: (homeID: string, userID: string) =>
      solFetch(`/api/v1/homes/${homeID}/transfer-ownership`, {
        method: "POST",
        body: JSON.stringify({ user_id: userID }),
      }).then((r) => jsonOrNull(r)),
    removeMember: (homeID: string, userID: string) =>
      solFetch(`/api/v1/homes/${homeID}/members/${userID}`, {
        method: "DELETE",
      }).then((r) => jsonOrNull(r)),
    updateMemberRole: (homeID: string, userID: string, role: MemberRole) =>
      solFetch(`/api/v1/homes/${homeID}/members/${userID}/role`, {
        method: "PATCH",
        body: JSON.stringify({ role }),
      }).then((r) => jsonOrNull(r)),
    permissions: {
      get: (homeID: string, userID: string) =>
        solFetch(`/api/v1/homes/${homeID}/members/${userID}/permissions`).then((r) =>
          safeJson<PermissionTree>(r, {} as PermissionTree),
        ),
      set: (homeID: string, userID: string, grants: PermissionScopeRef[], manageRooms: string[] = []) =>
        solFetch(`/api/v1/homes/${homeID}/members/${userID}/permissions`, {
          method: "PUT",
          body: JSON.stringify({ grants, manage_rooms: manageRooms }),
        }).then((r) => jsonOrNull(r)),
    },
  },

  invitations: {
    getPublic: (token: string) =>
      solPublicFetch(`/api/v1/invitations/${token}`).then((r) =>
        safeJson<InvitationDetail>(r, {} as InvitationDetail),
      ),
    accept: (token: string) =>
      solFetch(`/api/v1/invitations/${token}/accept`, {
        method: "POST",
      }).then((r) => safeJson<Home>(r, {} as Home)),
    declinePublic: (token: string) =>
      solPublicFetch(`/api/v1/invitations/${token}/decline`, {
        method: "POST",
      }).then((r) => jsonOrNull(r)),
  },

  devices: {
    list: (params?: { cursor?: string; limit?: number }) =>
      solFetch(`/api/v1/devices${buildQuery(params ?? {})}`).then(async (r) =>
        normalizeCursorResponse<RoomDevice>(await safeJson<RawCursorResponse<RoomDevice>>(r, {})),
      ),
    listAll: (limit = 100) =>
      collectCursorPages<RoomDevice>((cursor) =>
        solCore.devices.list({ cursor, limit }),
      ),
    get: (id: string) => solFetch(`/api/v1/devices/${id}`).then((r) => safeJson<RoomDevice>(r, {} as RoomDevice)),
    provision: (id: string) =>
      solFetch(`/api/v1/devices/${id}/provision`).then((r) =>
        safeJson<DeviceProvisioning>(r, {} as DeviceProvisioning),
      ),
    create: (body: unknown) =>
      solFetch("/api/v1/devices", {
        method: "POST",
        body: JSON.stringify(body),
      }).then((r) => safeJson<RoomDevice>(r, {} as RoomDevice)),
    update: (id: string, body: unknown) =>
      solFetch(`/api/v1/devices/${id}`, {
        method: "PUT",
        body: JSON.stringify(body),
      }).then((r) => jsonOrNull(r)),
    delete: (id: string) => solFetch(`/api/v1/devices/${id}`, { method: "DELETE" }).then((r) => jsonOrNull(r)),
    command: (id: string, body: unknown) =>
      solFetch(`/api/v1/devices/${id}/command`, {
        method: "POST",
        body: JSON.stringify(body),
      }).then((r) => safeJson<CommandResponse>(r, {})),
    getTelemetry: (id: string, limit: number = 100) =>
      solFetch(`/api/v1/devices/${id}/telemetry?limit=${limit}`).then((r) => safeJson<TelemetryPoint[]>(r, [])),
  },

  rooms: {
    list: (homeID: string, params?: { cursor?: string; limit?: number }) =>
      solFetch(`/api/v1/homes/${homeID}/rooms${buildQuery(params ?? {})}`).then(async (r) =>
        normalizeCursorResponse<Room>(await safeJson<RawCursorResponse<Room>>(r, {})),
      ),
    listAll: (homeID: string, limit = 100) =>
      collectCursorPages<Room>((cursor) => solCore.rooms.list(homeID, { cursor, limit })),
    create: (homeID: string, body: { name: string; floor?: number }) =>
      solFetch(`/api/v1/homes/${homeID}/rooms`, {
        method: "POST",
        body: JSON.stringify(body),
      }).then((r) => safeJson<Room>(r, {} as Room)),
    get: (homeID: string, roomID: string) =>
      solFetch(`/api/v1/homes/${homeID}/rooms/${roomID}`).then((r) => safeJson<Room>(r, {} as Room)),
    update: (homeID: string, roomID: string, body: Partial<{ name: string; floor: number }>) =>
      solFetch(`/api/v1/homes/${homeID}/rooms/${roomID}`, {
        method: "PUT",
        body: JSON.stringify(body),
      }).then((r) => safeJson<Room>(r, {} as Room)),
    delete: (homeID: string, roomID: string) =>
      solFetch(`/api/v1/homes/${homeID}/rooms/${roomID}`, { method: "DELETE" }).then((r) =>
        jsonOrNull(r),
      ),
    activity: (homeID: string, roomID: string, params?: { cursor?: string; limit?: number }) =>
      solFetch(
        `/api/v1/homes/${homeID}/rooms/${roomID}/activity${buildQuery(params ?? {})}`,
      ).then(async (r) =>
        normalizeCursorResponse<ActivityLog>(await safeJson<RawCursorResponse<ActivityLog>>(r, {})),
      ),
    devices: {
      list: (homeID: string, roomID: string, params?: { cursor?: string; limit?: number }) =>
        solFetch(`/api/v1/homes/${homeID}/rooms/${roomID}/devices${buildQuery(params ?? {})}`).then(
          async (r) =>
            normalizeCursorResponse<RoomDevice>(await safeJson<RawCursorResponse<RoomDevice>>(r, {})),
        ),
      listAll: (homeID: string, roomID: string, limit = 100) =>
        collectCursorPages<RoomDevice>((cursor) =>
          solCore.rooms.devices.list(homeID, roomID, { cursor, limit }),
        ),
      create: (
        homeID: string,
        roomID: string,
        body: { name: string; type: string; metadata?: Record<string, string> },
      ) =>
        solFetch(`/api/v1/homes/${homeID}/rooms/${roomID}/devices`, {
          method: "POST",
          body: JSON.stringify(body),
        }).then((r) => safeJson<RoomDevice>(r, {} as RoomDevice)),
      command: (
        homeID: string,
        roomID: string,
        deviceID: string,
        body: { action: string; params?: Record<string, unknown> },
      ) =>
        solFetch(`/api/v1/homes/${homeID}/rooms/${roomID}/devices/${deviceID}/command`, {
          method: "POST",
          body: JSON.stringify(body),
        }).then((r) => jsonOrNull(r)),
      ota: (
        homeID: string,
        roomID: string,
        deviceID: string,
        body: { firmware_version_id: string; idempotency_key?: string },
      ) =>
        solFetch(`/api/v1/homes/${homeID}/rooms/${roomID}/devices/${deviceID}/ota`, {
          method: "POST",
          body: JSON.stringify(body),
        }).then((r) => safeJson<{ attempt_id: string; request_id: string; status: OTAAttemptStatus }>(r, {
          attempt_id: "",
          request_id: "",
          status: "initiated",
        })),
    },
    otaAttempts: {
      list: (homeID: string, roomID: string, limit: number = 50) =>
        solFetch(`/api/v1/homes/${homeID}/rooms/${roomID}/ota-attempts?limit=${limit}`).then((r) =>
          safeJson<{ data: OTAAttempt[] }>(r, { data: [] }),
        ),
      retry: (homeID: string, roomID: string, attemptID: string) =>
        solFetch(`/api/v1/homes/${homeID}/rooms/${roomID}/ota-attempts/${attemptID}/retry`, {
          method: "POST",
        }).then((r) => safeJson<{ attempt_id: string; request_id: string; status: OTAAttemptStatus }>(r, {
          attempt_id: "",
          request_id: "",
          status: "initiated",
        })),
      cancel: (homeID: string, roomID: string, attemptID: string) =>
        solFetch(`/api/v1/homes/${homeID}/rooms/${roomID}/ota-attempts/${attemptID}/cancel`, {
          method: "POST",
        }).then((r) => safeJson<{ attempt_id: string; request_id: string; status: OTAAttemptStatus }>(r, {
          attempt_id: "",
          request_id: "",
          status: "cancelling",
        })),
    },
  },

  appliances: {
    create: (body: Partial<Appliance>) =>
      solFetch("/api/v1/appliances", {
        method: "POST",
        body: JSON.stringify(body),
      }).then((r) => safeJson<Appliance>(r, {} as Appliance)),
    get: (id: string) =>
      solFetch(`/api/v1/appliances/${id}`).then((r) => safeJson<Appliance>(r, {} as Appliance)),
    update: (id: string, body: Partial<Appliance>) =>
      solFetch(`/api/v1/appliances/${id}`, {
        method: "PUT",
        body: JSON.stringify(body),
      }).then((r) => jsonOrNull(r)),
    delete: (id: string) =>
      solFetch(`/api/v1/appliances/${id}`, { method: "DELETE" }).then((r) => jsonOrNull(r)),
    listByRoom: (homeID: string, roomID: string) =>
      solFetch(`/api/v1/homes/${homeID}/rooms/${roomID}/appliances`).then((r) =>
        safeJson<{ data: Appliance[] }>(r, { data: [] }),
      ),
  },

  automations: {
    list: () => solFetch("/api/v1/automations").then((r) => safeJson<Automation[]>(r, [])),
    get: (id: string) => solFetch(`/api/v1/automations/${id}`).then((r) => safeJson<Automation>(r, {})),
    create: (body: unknown) =>
      solFetch("/api/v1/automations", {
        method: "POST",
        body: JSON.stringify(body),
      }).then((r) => safeJson<Automation>(r, {})),
    update: (id: string, body: unknown) =>
      solFetch(`/api/v1/automations/${id}`, {
        method: "PUT",
        body: JSON.stringify(body),
      }).then((r) => safeJson<Automation>(r, {})),
    delete: (id: string) => solFetch(`/api/v1/automations/${id}`, { method: "DELETE" }),
  },

  firmware: {
    list: (templateID?: string) =>
      solFetch(`/api/v1/firmware${templateID ? `?template_id=${encodeURIComponent(templateID)}` : ""}`).then(
        (r) => safeJson<FirmwareVersion[]>(r, []),
      ),
    upload: (formData: FormData) =>
      solFetch("/api/v1/firmware/upload", {
        method: "POST",
        body: formData,
      }).then((r) => safeJson<FirmwareVersion>(r, {} as FirmwareVersion)),
    build: (templateID: string, targetBoard: string) =>
      solFetch("/api/v1/firmware/build", {
        method: "POST",
        body: JSON.stringify({ template_id: templateID, target_board: targetBoard }),
      }).then((r) => safeJson<{ id: string }>(r, { id: "" })),
    getBuild: (id: string) =>
      solFetch(`/api/v1/firmware/builds/${id}`).then((r) => safeJson<FirmwareBuild>(r, {} as FirmwareBuild)),
    listTargets: () =>
      solFetch("/api/v1/firmware/targets").then((r) => safeJson<string[]>(r, [])),
    presignedUrl: (id: string) =>
      solFetch(`/api/v1/firmware/versions/${id}/presigned-url`).then((r) =>
        safeJson<{ url: string }>(r, { url: "" }),
      ),
  },
}
