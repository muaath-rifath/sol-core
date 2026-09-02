import { auth } from "@/auth"
import { NextResponse } from "next/server"
import type { NextRequest } from "next/server"

export async function proxy(request: NextRequest) {
  const { pathname } = request.nextUrl

  // Always allow auth routes, root page, and static assets
  if (
    pathname.startsWith("/api/auth") ||
    pathname === "/" ||
    pathname.startsWith("/invitations/") ||
    pathname.startsWith("/_next") ||
    pathname.startsWith("/favicon")
  ) {
    return NextResponse.next()
  }

  const session = await auth()

  if (!session) {
    const rootUrl = new URL("/", request.url)
    rootUrl.searchParams.set("callbackUrl", pathname)
    return NextResponse.redirect(rootUrl)
  }

  // Refresh token failed — force re-login
  if (session.error === "RefreshAccessTokenError") {
    const rootUrl = new URL("/", request.url)
    rootUrl.searchParams.set("callbackUrl", pathname)
    rootUrl.searchParams.set("reauth", "1")
    return NextResponse.redirect(rootUrl)
  }

  return NextResponse.next()
}

export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico).*)"],
}
