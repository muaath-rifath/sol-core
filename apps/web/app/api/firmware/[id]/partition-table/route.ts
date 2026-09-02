import { auth } from "@/auth"

const SOL_CORE_URL = process.env.SOL_CORE_URL

export async function GET(
  _request: Request,
  context: { params: Promise<{ id: string }> },
) {
  if (!SOL_CORE_URL) {
    return Response.json({ error: "SOL_CORE_URL is not configured" }, { status: 500 })
  }

  const session = await auth()
  if (!session?.accessToken) {
    return Response.json({ error: "Unauthorized" }, { status: 401 })
  }

  const { id } = await context.params
  const upstream = await fetch(`${SOL_CORE_URL}/api/v1/firmware/versions/${id}/partition-table`, {
    headers: {
      Authorization: `Bearer ${session.accessToken}`,
    },
    cache: "no-store",
  })

  if (!upstream.ok) {
    const text = await upstream.text()
    return new Response(text || JSON.stringify({ error: "Failed to fetch partition table" }), {
      status: upstream.status,
      headers: { "Content-Type": upstream.headers.get("Content-Type") ?? "application/json" },
    })
  }

  const headers = new Headers()
  headers.set("Content-Type", upstream.headers.get("Content-Type") ?? "application/octet-stream")
  const disposition = upstream.headers.get("Content-Disposition")
  if (disposition) {
    headers.set("Content-Disposition", disposition)
  }

  return new Response(upstream.body, {
    status: 200,
    headers,
  })
}
