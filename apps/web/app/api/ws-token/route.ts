import { auth } from "@/auth"
import { NextResponse } from "next/server"

export async function GET() {
  const session = await auth()

  if (!session?.accessToken) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 })
  }

  return NextResponse.json({ token: session.accessToken })
}
