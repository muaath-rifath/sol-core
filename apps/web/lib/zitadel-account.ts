const ISSUER = process.env.AUTH_ZITADEL_ISSUER!

async function accountFetch(path: string, token: string, init: RequestInit = {}): Promise<Response> {
  const headers = new Headers(init.headers)
  headers.set("Authorization", `Bearer ${token}`)
  headers.set("Accept", "application/json")
  if (init.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json")
  }
  const res = await fetch(`${ISSUER}/auth/v1${path}`, { ...init, cache: "no-store", headers })
  if (!res.ok) {
    const text = await res.text().catch(() => "")
    let message = `${res.status}`
    try { message = (JSON.parse(text) as { message?: string }).message ?? message } catch {}
    throw new Error(message)
  }
  return res
}

export interface AccountProfile {
  username: string
  email: string
  firstName: string
  lastName: string
}

export const zitadelAccount = {
  getProfile: async (token: string): Promise<AccountProfile> => {
    const data = await accountFetch("/users/me", token).then((r) => r.json())
    return {
      username: data.user.userName ?? "",
      email: data.user.human?.email?.email ?? "",
      firstName: data.user.human?.profile?.firstName ?? "",
      lastName: data.user.human?.profile?.lastName ?? "",
    }
  },

  updateProfile: (token: string, body: Partial<Pick<AccountProfile, "firstName" | "lastName">>) =>
    accountFetch("/users/me/profile", token, {
      method: "PUT",
      body: JSON.stringify({
        firstName: body.firstName,
        lastName: body.lastName,
        displayName: [body.firstName, body.lastName].filter(Boolean).join(" "),
      }),
    }),

  changePassword: (token: string, body: { currentPassword: string; newPassword: string }) =>
    accountFetch("/users/me/password", token, {
      method: "POST",
      body: JSON.stringify({ currentPassword: body.currentPassword, newPassword: body.newPassword }),
    }),
}
