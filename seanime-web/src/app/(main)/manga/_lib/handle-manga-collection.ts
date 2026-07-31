import { Manga_Collection, Manga_MangaLatestChapterNumberItem } from "@/api/generated/types"
import { useListMangaProviderExtensions } from "@/api/hooks/extensions.hooks"
import { useGetMangaCollection, useGetMangaLatestChapterNumbersMap } from "@/api/hooks/manga.hooks"
import { useServerStatus } from "@/app/(main)/_hooks/use-server-status"
import { CollectionParams, DEFAULT_COLLECTION_PARAMS, filterCollectionEntries, filterMangaCollectionEntries } from "@/lib/helpers/filtering"
import { useRouter } from "@/lib/navigation"
import { useThemeSettings } from "@/lib/theme/theme-hooks"
import { atomWithImmer } from "jotai-immer"
import { useAtom } from "jotai/react"
import { atomWithStorage } from "jotai/utils"
import React from "react"
import { MangaEntryFilters, useStoredMangaFilters, useStoredMangaProviders } from "./handle-manga-selected-provider"

export const MANGA_LIBRARY_DEFAULT_PARAMS: CollectionParams<"manga"> = {
    ...DEFAULT_COLLECTION_PARAMS,
    sorting: "TITLE",
    unreadOnly: false,
}

export const __mangaLibrary_unreadOnlyAtom = atomWithStorage("sea-manga-library-unread-only", false, undefined, { getOnInit: true })

export const __mangaLibrary_isAdultAtom = atomWithStorage("sea-manga-library-is-adult", false, undefined, { getOnInit: true })

export const __mangaLibrary_paramsAtom = atomWithImmer<CollectionParams<"manga">>(MANGA_LIBRARY_DEFAULT_PARAMS)

export const __mangaLibrary_paramsInputAtom = atomWithImmer<CollectionParams<"manga">>(MANGA_LIBRARY_DEFAULT_PARAMS)

export const __mangaLibrary_latestChapterNumbersAtom = atomWithImmer<{
    latestChapterNumbers: Record<number, Manga_MangaLatestChapterNumberItem[]>
    storedProviders: Record<string, string>
    storedFilters: Record<string, MangaEntryFilters>
}>({
    latestChapterNumbers: {},
    storedProviders: {},
    storedFilters: {},
})

/**
 * Get the manga collection
 */
export function useHandleMangaCollection() {
    const router = useRouter()
    const { data, isLoading, isError, refetch } = useGetMangaCollection()

    // const { data: chapterCounts } = useGetMangaChapterCountMap()
    const { data: latestChapterNumbers } = useGetMangaLatestChapterNumbersMap()
    const { data: _extensions } = useListMangaProviderExtensions()

    const { mangaLibraryCollectionDefaultSorting } = useThemeSettings()

    React.useEffect(() => {
        if (isError) {
            router.push("/")
        }
    }, [isError])

    const { storedProviders } = useStoredMangaProviders(_extensions)
    const { storedFilters } = useStoredMangaFilters(_extensions, storedProviders)

    const [, setLatestChapterNumbers] = useAtom(__mangaLibrary_latestChapterNumbersAtom)
    React.useEffect(() => {
        if (latestChapterNumbers) {
            setLatestChapterNumbers({
                latestChapterNumbers: latestChapterNumbers,
                storedProviders,
                storedFilters,
            })
        }
    }, [storedProviders, storedFilters, latestChapterNumbers])

    const serverStatus = useServerStatus()
    const [params, setParams] = useAtom(__mangaLibrary_paramsAtom)
    const [unreadOnly, setUnreadOnly] = useAtom(__mangaLibrary_unreadOnlyAtom)
    const [isAdult, setIsAdult] = useAtom(__mangaLibrary_isAdultAtom)

    const mountedRef = React.useRef(false)
    React.useEffect(() => {
        if (mountedRef.current) return
        setParams(draft => {
            draft.unreadOnly = unreadOnly
            draft.isAdult = isAdult
            return
        })
        setTimeout(() => {
            mountedRef.current = true
        }, 500)
    }, [])

    // Reset params when data changes
    React.useEffect(() => {
        if (!!data) {
            const defaultParams = { ...MANGA_LIBRARY_DEFAULT_PARAMS, unreadOnly, isAdult }
            setParams(defaultParams)
        }
    }, [data, unreadOnly, isAdult])

    // Sync unreadOnly to persistent storage when params change
    React.useEffect(() => {
        if (mountedRef.current && params.unreadOnly !== unreadOnly) {
            setUnreadOnly(params.unreadOnly)
        }
    }, [params.unreadOnly])

    // Sync isAdult to persistent storage when params change
    React.useEffect(() => {
        if (mountedRef.current && params.isAdult !== isAdult) {
            setIsAdult(params.isAdult)
        }
    }, [params.isAdult])

    const genres = React.useMemo(() => {
        const genresSet = new Set<string>()
        data?.lists?.forEach(l => {
            l.entries?.forEach(e => {
                e.media?.genres?.forEach(g => {
                    genresSet.add(g)
                })
            })
        })
        return Array.from(genresSet)?.sort((a, b) => a.localeCompare(b))
    }, [data])

    const configs = React.useMemo(() => ({
        enableAdultContent: serverStatus?.settings?.anilist?.enableAdultContent || false,
        splitAdultContent: serverStatus?.settings?.anilist?.splitAdultContent || false,
    }), [serverStatus?.settings?.anilist?.enableAdultContent, serverStatus?.settings?.anilist?.splitAdultContent])

    const sortedCollection = React.useMemo(() => {
        if (!data || !data.lists) return data

        let _lists = data.lists.map(obj => {
            if (!obj) return obj

            const newParams = { ...params, sorting: mangaLibraryCollectionDefaultSorting as any }
            let arr = filterMangaCollectionEntries(obj.entries, newParams, configs.enableAdultContent, configs.splitAdultContent, storedProviders, storedFilters, latestChapterNumbers)

            // fall back to all manga when the unread filter empties the list
            if (arr.length === 0 && newParams.unreadOnly) {
                const newParams = { ...params, unreadOnly: false, sorting: mangaLibraryCollectionDefaultSorting as any }
                arr = filterMangaCollectionEntries(obj.entries, newParams, configs.enableAdultContent, configs.splitAdultContent, storedProviders, storedFilters, latestChapterNumbers)
            }

            return {
                type: obj.type,
                status: obj.status,
                entries: arr,
            }
        })

        return {
            lists: [
                _lists.find(n => n.type === "CURRENT"),
                _lists.find(n => n.type === "PAUSED"),
                _lists.find(n => n.type === "PLANNING"),
                // data.lists.find(n => n.type === "COMPLETED"), // DO NOT SHOW THIS LIST IN MANGA VIEW
                // data.lists.find(n => n.type === "DROPPED"), // DO NOT SHOW THIS LIST IN MANGA VIEW
            ].filter(Boolean),
        } as Manga_Collection
    }, [data, params, configs, storedProviders, storedFilters, latestChapterNumbers])

    const filteredCollection = React.useMemo(() => {
        if (!data || !data.lists) return data

        let _lists = data.lists.map(obj => {
            if (!obj) return obj

            const newParams = { ...params, sorting: mangaLibraryCollectionDefaultSorting as any }
            const arr = filterCollectionEntries("manga", obj.entries, newParams, configs.enableAdultContent, configs.splitAdultContent)
            return {
                type: obj.type,
                status: obj.status,
                entries: arr,
            }
        })
        return {
            lists: [
                _lists.find(n => n.type === "CURRENT"),
                _lists.find(n => n.type === "PAUSED"),
                _lists.find(n => n.type === "PLANNING"),
                // data.lists.find(n => n.type === "COMPLETED"), // DO NOT SHOW THIS LIST IN MANGA VIEW
                // data.lists.find(n => n.type === "DROPPED"), // DO NOT SHOW THIS LIST IN MANGA VIEW
            ].filter(Boolean),
        } as Manga_Collection
    }, [data, params, configs])

    const libraryGenres = React.useMemo(() => {
        const allGenres = filteredCollection?.lists?.flatMap(l => {
            return l.entries?.flatMap(e => e.media?.genres) ?? []
        })
        return [...new Set(allGenres)].filter(Boolean)?.sort((a, b) => a.localeCompare(b))
    }, [filteredCollection])

    return {
        genres,
        hasManga: !!data?.lists?.some(l => !!l.entries?.length),
        mangaCollection: sortedCollection,
        filteredMangaCollection: filteredCollection,
        mangaCollectionGenres: libraryGenres,
        mangaCollectionLoading: isLoading,
        mangaCollectionIsError: isError,
        refetchMangaCollection: refetch,
        storedFilters,
        storedProviders,
    }
}
