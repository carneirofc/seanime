import { INTERNAL_FeatureKey } from "@/api/generated/types"
import { serverAuthTokenAtom, serverStatusAtom } from "@/app/(main)/_atoms/server-status.atoms"
import { createServerPasswordHMACAuth } from "@/lib/server/hmac-auth"
import { TORRENT_PROVIDER } from "@/lib/server/settings"
import { useAtomValue } from "jotai"
import { useAtom } from "jotai"
import { useSetAtom } from "jotai/react"
import React from "react"
import { toast } from "sonner"

export function useServerStatus() {
    return useAtomValue(serverStatusAtom)
}

export function useSetServerStatus() {
    return useSetAtom(serverStatusAtom)
}

export function useCurrentUser() {
    const serverStatus = useServerStatus()
    return React.useMemo(() => serverStatus?.user, [serverStatus?.user])
}

export function useIsSimulatedUser() {
    const serverStatus = useServerStatus()
    return React.useMemo(() => !!serverStatus?.user?.isSimulated, [serverStatus?.user?.isSimulated])
}

export function useHasTorrentProvider() {
    const serverStatus = useServerStatus()
    return {
        hasTorrentProvider: React.useMemo(() => !!serverStatus?.settings?.library?.torrentProvider && serverStatus?.settings?.library?.torrentProvider !== TORRENT_PROVIDER.NONE,
            [serverStatus?.settings?.library?.torrentProvider]),
    }
}

export function useHasDebridService() {
    const serverStatus = useServerStatus()
    return {
        hasDebridService: React.useMemo(() => !!serverStatus?.debridSettings?.enabled && !!serverStatus?.debridSettings?.provider,
            [serverStatus?.debridSettings]),
    }
}

export function useHasTorrentStreaming() {
    const serverStatus = useServerStatus()
    return {
        hasTorrentStreaming: React.useMemo(() => !!serverStatus?.torrentstreamSettings?.enabled,
            [serverStatus?.torrentstreamSettings]),
    }
}

export function useHasOnlineStreaming() {
    const serverStatus = useServerStatus()
    return {
        hasOnlineStreaming: React.useMemo(() =>
                (!!serverStatus?.settings?.library?.enableOnlinestream),
            [serverStatus?.settings?.library]),
    }
}

export function useSelectedDebridService() {
    const serverStatus = useServerStatus()
    return {
        selectedDebridService: React.useMemo(() => serverStatus?.debridSettings?.provider,
            [serverStatus?.debridSettings]),
    }
}

export function useHasTorrentOrDebridInclusion() {
    const serverStatus = useServerStatus()
    const { hasDebridService } = useHasDebridService()
    const { hasTorrentStreaming } = useHasTorrentStreaming()
    const { hasOnlineStreaming } = useHasOnlineStreaming()
    return {
        hasStreamingEnabled: hasOnlineStreaming || hasTorrentStreaming || hasDebridService,
        hasTorrentOrDebridInclusion: React.useMemo(() =>
                (!!serverStatus?.debridSettings?.enabled && !!serverStatus?.debridSettings?.provider && !!serverStatus?.debridSettings?.includeDebridStreamInLibrary) ||
                (!!serverStatus?.torrentstreamSettings?.enabled && serverStatus?.torrentstreamSettings?.includeInLibrary),
            [serverStatus?.debridSettings, serverStatus?.torrentstreamSettings]),
    }
}

export function useServerHMACAuth() {
    const serverStatus = useServerStatus()
    const [password] = useAtom(serverAuthTokenAtom)
    const isOidc = serverStatus?.authMethod === "oidc"

    // With OIDC login the HMAC secret lives server-side only; the session-authenticated
    // client asks the server to mint tokens (needed for external player URLs).
    const fetchServerMintedToken = async (endpoint: string): Promise<string> => {
        const res = await fetch(`/api/v1/auth/media-token?endpoint=${encodeURIComponent(endpoint)}`, { credentials: "same-origin" })
        if (!res.ok) return ""
        const json = await res.json() as { data?: string }
        return json?.data ?? ""
    }

    return {
        password,
        getHMACTokenQueryParam: async (endpoint: string, symbol?: string): Promise<string> => {
            try {
                if (isOidc) {
                    const token = await fetchServerMintedToken(endpoint)
                    return token ? `${symbol ?? "?"}token=${token}` : ""
                }

                if (!serverStatus?.serverHasPassword || !password) return ""
                const hmacAuth = createServerPasswordHMACAuth(password)
                return await hmacAuth.generateQueryParam(endpoint, symbol)
            }
            catch (error) {
                console.error("Failed to generate HMAC token:", error)
                return ""
            }
        },
        generateHMACToken: async (endpoint: string) => {
            try {
                if (isOidc) {
                    return await fetchServerMintedToken(endpoint)
                }

                if (!serverStatus?.serverHasPassword || !password) return ""
                const hmacAuth = createServerPasswordHMACAuth(password)
                return await hmacAuth.generateToken(endpoint)
            }
            catch (error) {
                console.error("Failed to generate HMAC token:", error)
                return ""
            }
        },
    }
}

export function useServerDisabledFeatures() {
    const status = useServerStatus()

    return {
        isFeatureDisabled: (feature: INTERNAL_FeatureKey) => {
            if (!status?.disabledFeatures?.length) return false
            return status?.disabledFeatures?.includes(feature)
        },
        showFeatureWarning: () => {
            return toast.warning("This feature is disabled")
        },
    }
}
