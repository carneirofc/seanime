// Default AniList API client per build target, overridable per-deployment via
// config.toml `[anilist] clientid` (surfaced as status.anilistClientId):
//   dev  -> 43777 (deedlit-seanime-local-dev,  redirect http://127.0.0.1:43210/auth/callback)
//   prod -> 44460 (deedlit-seanime-local,      redirect http://127.0.0.1:43211/auth/callback)
// The redirect URI is derived from the current origin (see getAnilistAuthorizeUrl), so each
// client's registered redirect must match the port the app is served on (dev 43210 / prod 43211).
// seanime uses the OAuth implicit grant (response_type=token); the client *secret* is never used.
const ANILIST_DEFAULT_CLIENT_ID = import.meta.env.MODE === "development" ? "43777" : "44460"

/**
 * Builds the AniList implicit-grant authorize URL.
 * - clientId: pass status.anilistClientId to honor the config.toml override; falls back
 *   to ANILIST_DEFAULT_CLIENT_ID when empty.
 * - redirect_uri is derived from the current origin (`<origin>/auth/callback`) so it
 *   adapts to whatever host/port the app is served on (dev 43210, prod 43211, remote…).
 *   The matching redirect URI must be registered on the AniList client.
 */
export function getAnilistAuthorizeUrl(clientId?: string | null): string {
    const id = clientId && clientId.length > 0 ? clientId : ANILIST_DEFAULT_CLIENT_ID
    const params = new URLSearchParams({ client_id: id, response_type: "token" })
    if (typeof window !== "undefined" && window.location?.origin) {
        params.set("redirect_uri", `${window.location.origin}/auth/callback`)
    }
    return `https://anilist.co/api/v2/oauth/authorize?${params.toString()}`
}
export const ANILIST_PIN_URL = `https://anilist.co/api/v2/oauth/authorize?client_id=13985&response_type=token`
export const MAL_CLIENT_ID = `51cb4294feb400f3ddc66a30f9b9a00f`
export const __DEV_SERVER_PORT = 43000
