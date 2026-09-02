import NextAuth from "next-auth"
import Credentials from "next-auth/providers/credentials"
import Zitadel from "next-auth/providers/zitadel"
import type { JWT } from "next-auth/jwt"

const developmentAuth = process.env.AUTH_DEV_MODE !== "false"
export const authProvider = developmentAuth ? "credentials" : "zitadel"

export const { handlers, auth, signIn, signOut } = NextAuth({
  providers: developmentAuth
    ? [
        Credentials({
          name: "Local development",
          credentials: {},
          async authorize() {
            if (!process.env.AUTH_DEV_TOKEN) return null
            return { id: "local-development", name: "Local Developer", email: "developer@sol.local" }
          },
        }),
      ]
    : [
        Zitadel({
      clientId: process.env.AUTH_ZITADEL_ID!,
      issuer: process.env.AUTH_ZITADEL_ISSUER!,
      authorization: {
        params: {
          scope: "openid email profile",
        },
      },
      profile(profile) {
        console.debug("[auth] Zitadel profile:", JSON.stringify(profile, null, 2))
        const composed = [profile.given_name, profile.family_name]
          .filter((s): s is string => typeof s === "string" && s.trim().length > 0)
          .join(" ")
          .trim()
        return {
          id: profile.sub,
          name:
            (typeof profile.name === "string" && profile.name.trim()) ||
            composed ||
            profile.preferred_username ||
            profile.email ||
            null,
          email: profile.email,
          image: profile.picture,
        }
      },
        }),
      ],

  debug: process.env.NODE_ENV === "development",

  session: {
    strategy: "jwt",
  },

  callbacks: {
    async jwt({ token, account, user }) {
      // First sign-in: account contains the raw token response
      if (account && user) {
        token.accessToken = developmentAuth ? process.env.AUTH_DEV_TOKEN! : account.access_token!
        token.idToken = account.id_token
        token.refreshToken = account.refresh_token ?? ""
        token.accessTokenExpires = Date.now() + (developmentAuth ? 365 * 24 * 60 * 60 : (account.expires_in ?? 300)) * 1000
        
        // Ensure name/email/image are explicitly stored in the token
        token.name = user.name
        token.email = user.email
        token.picture = user.image
      }

      if (developmentAuth) return token

      // Token still valid
      if (Date.now() < (token.accessTokenExpires as number)) {
        return token
      }

      // Access token expired — refresh it
      return refreshAccessToken(token)
    },

    async session({ session, token }) {
      session.accessToken = token.accessToken as string
      session.idToken = token.idToken as string
      
      if (session.user) {
        session.user.name = token.name
        session.user.email = token.email as string
        session.user.image = token.picture as string
      }

      if (token.error) {
        session.error = token.error
      }
      return session
    },
  },

  pages: {
    signIn: "/",
  },
})

async function refreshAccessToken(token: JWT): Promise<JWT> {
  try {
    const tokenUrl = `${process.env.AUTH_ZITADEL_ISSUER}/oauth/v2/token`

    const response = await fetch(tokenUrl, {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body: new URLSearchParams({
        client_id: process.env.AUTH_ZITADEL_ID!,
        grant_type: "refresh_token",
        refresh_token: token.refreshToken,
      }),
    })

    const refreshed = await response.json()

    if (!response.ok) throw refreshed

    return {
      ...token,
      accessToken: refreshed.access_token,
      refreshToken: refreshed.refresh_token ?? token.refreshToken,
      accessTokenExpires: Date.now() + refreshed.expires_in * 1000,
      error: undefined,
    }
  } catch {
    return { ...token, error: "RefreshAccessTokenError" }
  }
}
