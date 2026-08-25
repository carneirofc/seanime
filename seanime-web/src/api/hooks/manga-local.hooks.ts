import { useServerMutation, useServerQuery } from "@/api/client/requests"
import { getServerBaseUrl } from "@/api/client/server-url"
import {
    DeleteLocalMangaSeries_Variables,
    MapLocalMangaSeries_Variables,
    RepackLocalMangaSeries_Variables,
    ScanLocalMangaLibrary_Variables,
} from "@/api/generated/endpoint.types"
import { API_ENDPOINTS } from "@/api/generated/endpoints"
import {
    Manga_LocalMangaChapter,
    Manga_LocalMangaLibrary,
    Manga_LocalMangaRepackResult,
    Manga_LocalMangaScanResult,
    Manga_LocalMangaUploadResult,
} from "@/api/generated/types"
import { serverAuthTokenAtom } from "@/app/(main)/_atoms/server-status.atoms"
import { useQueryClient } from "@tanstack/react-query"
import { useAtomValue } from "jotai"
import React from "react"
import { toast } from "sonner"

/**
 * Invalidates everything that can change when the local library does: the
 * library listing itself, and the chapter/page data of whichever entry a series
 * is now mapped to.
 */
function useInvalidateLocalMangaLibrary() {
    const queryClient = useQueryClient()

    return React.useCallback(async () => {
        await queryClient.invalidateQueries({ queryKey: [API_ENDPOINTS.MANGA_LOCAL.GetLocalMangaLibrary.key] })
        await queryClient.invalidateQueries({ queryKey: [API_ENDPOINTS.MANGA.GetMangaEntryChapters.key] })
        await queryClient.invalidateQueries({ queryKey: [API_ENDPOINTS.MANGA.GetMangaEntryPages.key] })
        await queryClient.invalidateQueries({ queryKey: [API_ENDPOINTS.MANGA.GetMangaMapping.key] })
        await queryClient.invalidateQueries({ queryKey: [API_ENDPOINTS.MANGA.GetMangaPreferences.key] })
    }, [queryClient])
}

export function useGetLocalMangaLibrary(enabled = true) {
    return useServerQuery<Manga_LocalMangaLibrary>({
        endpoint: API_ENDPOINTS.MANGA_LOCAL.GetLocalMangaLibrary.endpoint,
        method: API_ENDPOINTS.MANGA_LOCAL.GetLocalMangaLibrary.methods[0],
        queryKey: [API_ENDPOINTS.MANGA_LOCAL.GetLocalMangaLibrary.key],
        enabled,
        retry: false,
    })
}

export function useScanLocalMangaLibrary() {
    const invalidate = useInvalidateLocalMangaLibrary()

    return useServerMutation<Manga_LocalMangaScanResult, ScanLocalMangaLibrary_Variables>({
        endpoint: API_ENDPOINTS.MANGA_LOCAL.ScanLocalMangaLibrary.endpoint,
        method: API_ENDPOINTS.MANGA_LOCAL.ScanLocalMangaLibrary.methods[0],
        mutationKey: [API_ENDPOINTS.MANGA_LOCAL.ScanLocalMangaLibrary.key],
        onSuccess: async result => {
            const matched = result?.matched?.length ?? 0
            if (matched > 0) {
                toast.success(matched === 1 ? "Mapped 1 series" : `Mapped ${matched} series`)
            } else {
                toast.info("No new series could be matched")
            }
            await invalidate()
        },
    })
}

export function useUploadLocalMangaArchive() {
    const invalidate = useInvalidateLocalMangaLibrary()

    return useServerMutation<Manga_LocalMangaUploadResult, FormData>({
        endpoint: API_ENDPOINTS.MANGA_LOCAL.UploadLocalMangaArchive.endpoint,
        method: API_ENDPOINTS.MANGA_LOCAL.UploadLocalMangaArchive.methods[0],
        mutationKey: [API_ENDPOINTS.MANGA_LOCAL.UploadLocalMangaArchive.key],
        onSuccess: async result => {
            const chapters = result?.chapters?.length ?? 0
            toast.success(`Uploaded ${chapters} ${chapters === 1 ? "chapter" : "chapters"} to ${result?.series}`)
            await invalidate()
        },
    })
}

export function useMapLocalMangaSeries() {
    const invalidate = useInvalidateLocalMangaLibrary()

    return useServerMutation<boolean, MapLocalMangaSeries_Variables>({
        endpoint: API_ENDPOINTS.MANGA_LOCAL.MapLocalMangaSeries.endpoint,
        method: API_ENDPOINTS.MANGA_LOCAL.MapLocalMangaSeries.methods[0],
        mutationKey: [API_ENDPOINTS.MANGA_LOCAL.MapLocalMangaSeries.key],
        onSuccess: async () => {
            toast.success("Series mapped")
            await invalidate()
        },
    })
}

export function useGetLocalMangaChapters(series: string | null) {
    return useServerQuery<Array<Manga_LocalMangaChapter>>({
        endpoint: API_ENDPOINTS.MANGA_LOCAL.GetLocalMangaChapters.endpoint,
        method: API_ENDPOINTS.MANGA_LOCAL.GetLocalMangaChapters.methods[0],
        queryKey: [API_ENDPOINTS.MANGA_LOCAL.GetLocalMangaChapters.key, series],
        params: { series },
        enabled: !!series,
        retry: false,
    })
}

export function useRepackLocalMangaSeries() {
    const queryClient = useQueryClient()
    const invalidate = useInvalidateLocalMangaLibrary()

    return useServerMutation<Manga_LocalMangaRepackResult, RepackLocalMangaSeries_Variables>({
        endpoint: API_ENDPOINTS.MANGA_LOCAL.RepackLocalMangaSeries.endpoint,
        method: API_ENDPOINTS.MANGA_LOCAL.RepackLocalMangaSeries.methods[0],
        mutationKey: [API_ENDPOINTS.MANGA_LOCAL.RepackLocalMangaSeries.key],
        onSuccess: async result => {
            const repacked = result?.repacked?.length ?? 0
            const skipped = result?.skipped?.length ?? 0
            if (repacked > 0) {
                toast.success(`Rebuilt ${repacked} ${repacked === 1 ? "chapter" : "chapters"} as CBZ`)
            } else if (skipped > 0) {
                toast.info("Nothing to rebuild — no chapter could be rewritten")
            } else {
                toast.info("This series has no chapters to rebuild")
            }
            await queryClient.invalidateQueries({ queryKey: [API_ENDPOINTS.MANGA_LOCAL.GetLocalMangaChapters.key] })
            await invalidate()
        },
    })
}

export function useDeleteLocalMangaSeries() {
    const invalidate = useInvalidateLocalMangaLibrary()

    return useServerMutation<boolean, DeleteLocalMangaSeries_Variables>({
        endpoint: API_ENDPOINTS.MANGA_LOCAL.DeleteLocalMangaSeries.endpoint,
        method: API_ENDPOINTS.MANGA_LOCAL.DeleteLocalMangaSeries.methods[0],
        mutationKey: [API_ENDPOINTS.MANGA_LOCAL.DeleteLocalMangaSeries.key],
        onSuccess: async () => {
            toast.success("Series deleted")
            await invalidate()
        },
    })
}

/**
 * Downloads a local chapter, or a whole series, as a file.
 *
 * The request goes through fetch rather than an anchor href because these
 * endpoints need the server credential, and a plain link cannot carry a header.
 * The response is turned into a blob and handed to a synthetic anchor.
 */
export function useDownloadLocalManga() {
    const password = useAtomValue(serverAuthTokenAtom)
    const [isDownloading, setIsDownloading] = React.useState<string | null>(null)

    const download = React.useCallback(async (endpoint: string, params: Record<string, string>, key: string) => {
        setIsDownloading(key)

        try {
            const url = new URL(getServerBaseUrl() + endpoint, window.location.origin)
            for (const [name, value] of Object.entries(params)) {
                url.searchParams.set(name, value)
            }

            const headers: Record<string, string> = {}
            if (password) {
                headers["X-Seanime-Token"] = password
            }

            const response = await fetch(url.toString(), { method: "GET", headers, credentials: "include" })
            if (!response.ok) {
                // The server sends a JSON error for anything it refuses before the
                // stream starts, which is the useful message to show.
                let message = `Download failed (${response.status})`
                try {
                    const body = await response.json() as { error?: unknown }
                    if (typeof body?.error === "string" && body.error) message = body.error
                } catch {
                }
                throw new Error(message)
            }

            const blob = await response.blob()
            const objectUrl = window.URL.createObjectURL(blob)
            const link = document.createElement("a")
            link.href = objectUrl
            link.setAttribute("download", filenameFromResponse(response) || key)
            link.style.display = "none"
            document.body.appendChild(link)
            link.click()
            document.body.removeChild(link)
            window.URL.revokeObjectURL(objectUrl)
        }
        catch (error) {
            toast.error(error instanceof Error ? error.message : "Download failed")
        }
        finally {
            setIsDownloading(null)
        }
    }, [password])

    return {
        isDownloading,
        downloadChapter: React.useCallback((series: string, chapter: string) => {
            return download(API_ENDPOINTS.MANGA_LOCAL.DownloadLocalMangaChapter.endpoint, { series, chapter }, chapter)
        }, [download]),
        downloadSeries: React.useCallback((series: string) => {
            return download(API_ENDPOINTS.MANGA_LOCAL.DownloadLocalMangaSeries.endpoint, { series }, `${series}.zip`)
        }, [download]),
    }
}

/**
 * Reads the filename the server chose out of Content-Disposition, so the saved
 * file matches what the download endpoint named it.
 */
function filenameFromResponse(response: Response): string {
    const disposition = response.headers.get("content-disposition")
    if (!disposition) return ""

    const quoted = disposition.match(/filename="([^"]+)"/)
    if (quoted?.[1]) return quoted[1]

    const bare = disposition.match(/filename=([^;]+)/)
    return bare?.[1]?.trim() ?? ""
}
