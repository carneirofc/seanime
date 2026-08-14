import {
    HibikeManga_ChapterDetails,
    HibikeManga_ChapterProviderOption,
    Manga_MediaDownloadData,
    Nullish,
} from "@/api/generated/types"
import {
    useClearAllChapterDownloadQueue,
    useDownloadMangaChapters,
    useGetMangaDownloadData,
    useGetMangaDownloadQueue,
    useResetErroredChapterDownloadQueue,
    useStartMangaDownloadQueue,
    useStopMangaDownloadQueue,
} from "@/api/hooks/manga_download.hooks"
import { getServerBaseUrl } from "@/api/client/server-url"
import { useServerHMACAuth } from "@/app/(main)/_hooks/use-server-status"
import { useSelectedMangaProvider } from "@/app/(main)/manga/_lib/handle-manga-selected-provider"
import { atom } from "jotai"
import { useAtomValue, useSetAtom } from "jotai/react"
import React from "react"
import { toast } from "sonner"

/**
 * Stores fetched manga download data
 */
const __manga_entryDownloadDataAtom = atom<Manga_MediaDownloadData | undefined>(undefined)

export type MangaDownloadChapterItem = { provider: string, chapterId: string, chapterNumber: string, queued: boolean, downloaded: boolean }

/**
 * @description
 * - This atom transforms the download data into a list of chapters
 */
const __manga_entryDownloadedChaptersAtom = atom<MangaDownloadChapterItem[]>(get => {
    let d: MangaDownloadChapterItem[] = []
    const data = get(__manga_entryDownloadDataAtom)
    if (data) {
        for (const provider in data.downloaded) {
            d = d.concat(data.downloaded[provider].map(ch => ({
                provider,
                chapterId: ch.chapterId,
                chapterNumber: ch.chapterNumber,
                queued: false,
                downloaded: true,
            })))
        }
        for (const provider in data.queued) {
            d = d.concat(data.queued[provider].map(ch => ({
                provider,
                chapterId: ch.chapterId,
                chapterNumber: ch.chapterNumber,
                queued: true,
                downloaded: false,
            })))
        }
    }
    return d
})

export function useMangaEntryDownloadedChapters() {
    return useAtomValue(__manga_entryDownloadedChaptersAtom)
}

/**
 * @description
 * - Fetch manga download data and store it in a state
 * - We store the download data in a state, so we can handle chapter pagination.
 *      For example, clicking "next chapter" will look for a downloaded chapter, and make a request with the appropriate provider
 */
export function useHandleMangaDownloadData(mediaId: Nullish<string | number>) {
    const { data, isLoading, isError } = useGetMangaDownloadData({
        mediaId: mediaId ? Number(mediaId) : undefined,
    })

    const setDownloadData = useSetAtom(__manga_entryDownloadDataAtom)
    React.useEffect(() => {
        setDownloadData(data)
    }, [data])

    return {
        downloadData: data,
        downloadDataLoading: isLoading,
        downloadDataError: isError,
    }
}

export function useMangaEntryDownloadData() {
    return useAtomValue(__manga_entryDownloadDataAtom)
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

/**
 * Handle downloading manga chapters
 */
export function useHandleDownloadMangaChapter(mediaId: string | undefined | null) {
    const { selectedProvider } = useSelectedMangaProvider(mediaId)

    const { mutateAsync, isPending } = useDownloadMangaChapters(mediaId, selectedProvider)

    const downloadChapters = React.useCallback(async (
        targets: Array<Pick<HibikeManga_ChapterDetails, "provider" | "id"> | HibikeManga_ChapterProviderOption>,
    ) => {
        if (!mediaId || targets.length === 0) return

        const chapterIdsByProvider = targets.reduce<Record<string, string[]>>((acc, target) => {
            const provider = target.provider
            const chapterId = "chapterId" in target ? target.chapterId : target.id

            if (!provider || !chapterId) return acc

            if (!acc[provider]) {
                acc[provider] = []
            }

            if (!acc[provider].includes(chapterId)) {
                acc[provider].push(chapterId)
            }

            return acc
        }, {})

        const providers = Object.keys(chapterIdsByProvider)
        if (providers.length === 0) return

        await Promise.all(providers.map(provider => mutateAsync({
            mediaId: Number(mediaId),
            provider,
            chapterIds: chapterIdsByProvider[provider],
            startNow: false,
        })))

        toast.success(providers.length > 1 ? "Chapters added to download queue from multiple sources" : "Chapters added to download queue")
    }, [mediaId, mutateAsync])

    return {
        downloadChapters,
        isSendingDownloadRequest: isPending,
    }
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

/**
 * Handle saving downloaded chapters to the user's device as CBZ archive files.
 * The server resolves the archive and names it after the media title, which the
 * client reads back from the Content-Disposition header.
 */
export function useMangaEntryArchiveDownload() {
    const hmacAuth = useServerHMACAuth()
    const hmacAuthRef = React.useRef(hmacAuth)
    React.useEffect(() => {
        hmacAuthRef.current = hmacAuth
    })

    const [isDownloadingArchive, setIsDownloadingArchive] = React.useState(false)

    const fetchArchive = React.useCallback(async (endpoint: string, params: Record<string, string>, fallbackFilename: string) => {
        const { password, getHMACTokenQueryParam } = hmacAuthRef.current
        const tokenQuery = await getHMACTokenQueryParam(endpoint, "&")
        if (password && !tokenQuery) {
            throw new Error("Failed to generate download token")
        }

        const search = new URLSearchParams(params).toString()
        const response = await fetch(`${getServerBaseUrl()}${endpoint}?${search}${tokenQuery}`, {
            credentials: "include",
        })
        if (!response.ok) {
            let message = "Failed to download archive"
            try {
                const data: unknown = await response.json()
                if (typeof data === "object" && data !== null && "error" in data && typeof data.error === "string" && data.error.trim()) {
                    message = data.error
                }
            }
            catch {
            }
            throw new Error(message)
        }

        const blob = await response.blob()
        const contentDisposition = response.headers.get("content-disposition")
        const filename = contentDisposition?.match(/filename="?([^";]+)"?/i)?.[1] ?? fallbackFilename
        const url = URL.createObjectURL(blob)
        const link = document.createElement("a")
        link.href = url
        link.download = filename
        document.body.appendChild(link)
        link.click()
        link.remove()
        setTimeout(() => URL.revokeObjectURL(url), 0)
    }, [])

    const downloadChapterArchive = React.useCallback(async (provider: string, mediaId: number, chapterId: string, chapterNumber: string) => {
        setIsDownloadingArchive(true)
        try {
            await fetchArchive("/api/v1/manga/downloads/chapter-archive", {
                provider,
                mediaId: String(mediaId),
                chapterId,
            }, `${provider}_${mediaId} - Chapter ${chapterNumber}.cbz`)
        }
        catch (error) {
            toast.error(error instanceof Error ? error.message : "Failed to download chapter archive")
        }
        finally {
            setIsDownloadingArchive(false)
        }
    }, [fetchArchive])

    const downloadMediaArchives = React.useCallback(async (mediaId: number, providers: string[]) => {
        if (providers.length === 0) return
        setIsDownloadingArchive(true)
        try {
            // One zip per provider, sequentially, so concurrent streams don't compete
            for (const provider of providers) {
                await fetchArchive("/api/v1/manga/downloads/media-archive", {
                    provider,
                    mediaId: String(mediaId),
                }, `${provider}_${mediaId}.zip`)
            }
        }
        catch (error) {
            toast.error(error instanceof Error ? error.message : "Failed to download chapter archives")
        }
        finally {
            setIsDownloadingArchive(false)
        }
    }, [fetchArchive])

    return {
        isDownloadingArchive,
        downloadChapterArchive,
        downloadMediaArchives,
    }
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

/**
 * Handle the manga chapter download queue
 */
export function useHandleMangaChapterDownloadQueue() {

    const { data, isLoading, isError } = useGetMangaDownloadQueue()

    const { mutate: start, isPending: isStarting } = useStartMangaDownloadQueue()

    const { mutate: stop, isPending: isStopping } = useStopMangaDownloadQueue()

    const { mutate: resetErrored, isPending: isResettingErrored } = useResetErroredChapterDownloadQueue()

    const { mutate: clearQueue, isPending: isClearingQueue } = useClearAllChapterDownloadQueue()

    return {
        downloadQueue: data,
        downloadQueueLoading: isLoading,
        downloadQueueError: isError,
        startDownloadQueue: start,
        isStartingDownloadQueue: isStarting,
        stopDownloadQueue: stop,
        isStoppingDownloadQueue: isStopping,
        resetErroredChapters: resetErrored,
        isResettingErroredChapters: isResettingErrored,
        clearDownloadQueue: clearQueue,
        isClearingDownloadQueue: isClearingQueue,
    }
}
