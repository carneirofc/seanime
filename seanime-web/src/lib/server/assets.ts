import { getServerBaseUrl } from "@/api/client/server-url"

const IMAGE_PROXY_PATH = "/api/v1/image-proxy"

function tryParseUrl(path: string) {
    try {
        return new URL(path, "http://localhost")
    } catch {
        return null
    }
}

function normalizeHeaders(headers: Record<string, string>) {
    const entries = Object.entries(headers)
        .filter(([, value]) => value != null && value !== "")
        .sort(([a], [b]) => a.localeCompare(b))

    if (entries.length === 0) {
        return ""
    }

    return JSON.stringify(Object.fromEntries(entries))
}

export function buildImageProxyUrl(imageUrl: string, headers?: Record<string, string>) {
    const baseUrl = `${getServerBaseUrl()}${IMAGE_PROXY_PATH}?url=${encodeURIComponent(imageUrl)}`
    const payload = headers ? normalizeHeaders(headers) : ""

    if (!payload) {
        return baseUrl
    }

    return `${baseUrl}&headers=${encodeURIComponent(payload)}`
}

export function isImageProxyUrl(path: string) {
    const parsed = tryParseUrl(path)
    return parsed?.pathname.endsWith(IMAGE_PROXY_PATH) ?? false
}

export function getImageProxyFallbackUrl(path: string) {
    const parsed = tryParseUrl(path)
    if (!parsed?.pathname.endsWith(IMAGE_PROXY_PATH)) {
        return undefined
    }

    if (parsed.searchParams.has("headers")) {
        return undefined
    }

    return parsed.searchParams.get("url") || undefined
}

function stripQueryAndHash(path: string) {
    return path.split(/[?#]/)[0]
}

export function getImageUrl(path: string, headers?: Record<string, string>) {
    if (!path) return path

    if (path.startsWith("{{LOCAL_ASSETS}}")) {
        return `${getServerBaseUrl()}/${path.replace("{{LOCAL_ASSETS}}", "offline-assets")}`
    }

    const imageExtensions = [
        ".jpg",
        ".jpeg",
        ".png",
        ".gif",
        ".webp",
        ".avif",
        ".svg",
        ".bmp",
        ".tiff",
        ".webm",
    ]

    const normalizedPath = stripQueryAndHash(path).toLowerCase()
    if (imageExtensions.some(ext => normalizedPath.endsWith(ext))) {
        return buildImageProxyUrl(path, headers)
    }

    return path
}

export function getAssetUrl(path: string) {
    let p = path.replaceAll("\\", "/")

    if (p.startsWith("/")) {
        p = p.substring(1)
    }

    p = encodeURIComponent(p).replace(/\(/g, "%28").replace(/\)/g, "%29")

    if (p.startsWith("{{LOCAL_ASSETS}}")) {
        return `${getServerBaseUrl()}/${p.replace("{{LOCAL_ASSETS}}", "offline-assets")}`
    }

    return `${getServerBaseUrl()}/assets/${p}`
}

export function legacy_getAssetUrl(path: string) {
    let p = path.replaceAll("\\", "/")

    if (p.startsWith("/")) {
        p = p.substring(1)
    }

    p = encodeURIComponent(p).replace(/\(/g, "%28").replace(/\)/g, "%29")

    return `${getServerBaseUrl()}/assets/${p}`
}
