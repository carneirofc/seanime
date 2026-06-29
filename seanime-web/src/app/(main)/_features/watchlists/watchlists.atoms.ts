import { atom } from "jotai"
import { useAtom } from "jotai/react"
import { atomWithStorage } from "jotai/utils"
import React from "react"

/**
 * Local, browser-persisted user collections.
 *
 * Hierarchy: Grouping (folder) -> Watchlist -> Media reference.
 * Everything lives in localStorage (no server involvement).
 */

export type WatchlistMediaType = "anime" | "manga"

export type WatchlistMedia = {
    mediaId: number
    type: WatchlistMediaType
    title: string
    image?: string
    format?: string | null
    year?: number | null
    addedAt: number
}

export type Watchlist = {
    id: string
    name: string
    media: WatchlistMedia[]
    createdAt: number
}

export type WatchlistGrouping = {
    id: string
    name: string
    watchlists: Watchlist[]
    createdAt: number
}

export const __watchlists_groupingsAtom = atomWithStorage<WatchlistGrouping[]>("sea-watchlist-groupings", [])

function uid(): string {
    try {
        if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
            return crypto.randomUUID()
        }
    }
    catch {
        // ignore
    }
    return `id-${Date.now()}-${Math.random().toString(36).slice(2)}`
}

function sameMedia(m: WatchlistMedia, mediaId: number, type: WatchlistMediaType) {
    return m.mediaId === mediaId && m.type === type
}

export function useWatchlists() {
    const [groupings, setGroupings] = useAtom(__watchlists_groupingsAtom)

    const createGrouping = React.useCallback((name: string) => {
        const id = uid()
        setGroupings(prev => [...prev, { id, name: name.trim() || "Untitled", watchlists: [], createdAt: Date.now() }])
        return id
    }, [setGroupings])

    const renameGrouping = React.useCallback((groupingId: string, name: string) => {
        setGroupings(prev => prev.map(g => g.id === groupingId ? { ...g, name: name.trim() || g.name } : g))
    }, [setGroupings])

    const deleteGrouping = React.useCallback((groupingId: string) => {
        setGroupings(prev => prev.filter(g => g.id !== groupingId))
    }, [setGroupings])

    const createWatchlist = React.useCallback((groupingId: string, name: string) => {
        const id = uid()
        setGroupings(prev => prev.map(g => g.id === groupingId
            ? { ...g, watchlists: [...g.watchlists, { id, name: name.trim() || "Untitled", media: [], createdAt: Date.now() }] }
            : g))
        return id
    }, [setGroupings])

    const renameWatchlist = React.useCallback((watchlistId: string, name: string) => {
        setGroupings(prev => prev.map(g => ({
            ...g,
            watchlists: g.watchlists.map(w => w.id === watchlistId ? { ...w, name: name.trim() || w.name } : w),
        })))
    }, [setGroupings])

    const deleteWatchlist = React.useCallback((watchlistId: string) => {
        setGroupings(prev => prev.map(g => ({ ...g, watchlists: g.watchlists.filter(w => w.id !== watchlistId) })))
    }, [setGroupings])

    const addMediaToWatchlist = React.useCallback((watchlistId: string, media: Omit<WatchlistMedia, "addedAt">) => {
        setGroupings(prev => prev.map(g => ({
            ...g,
            watchlists: g.watchlists.map(w => {
                if (w.id !== watchlistId) return w
                if (w.media.some(m => sameMedia(m, media.mediaId, media.type))) return w
                return { ...w, media: [...w.media, { ...media, addedAt: Date.now() }] }
            }),
        })))
    }, [setGroupings])

    const removeMediaFromWatchlist = React.useCallback((watchlistId: string, mediaId: number, type: WatchlistMediaType) => {
        setGroupings(prev => prev.map(g => ({
            ...g,
            watchlists: g.watchlists.map(w => w.id === watchlistId
                ? { ...w, media: w.media.filter(m => !sameMedia(m, mediaId, type)) }
                : w),
        })))
    }, [setGroupings])

    return {
        groupings,
        createGrouping,
        renameGrouping,
        deleteGrouping,
        createWatchlist,
        renameWatchlist,
        deleteWatchlist,
        addMediaToWatchlist,
        removeMediaFromWatchlist,
    }
}

/* -------------------------------------------------------------------------------------------------
 * "Add to watchlist" modal manager
 * -----------------------------------------------------------------------------------------------*/

const addToWatchlist_mediaAtom = atom<Omit<WatchlistMedia, "addedAt"> | null>(null)
const addToWatchlist_openAtom = atom(false)

export function useAddToWatchlistManager() {
    const [media, setMedia] = useAtom(addToWatchlist_mediaAtom)
    const [isOpen, setOpen] = useAtom(addToWatchlist_openAtom)

    const openForMedia = React.useCallback((m: Omit<WatchlistMedia, "addedAt">) => {
        setMedia(m)
        React.startTransition(() => {
            setOpen(true)
        })
    }, [setMedia, setOpen])

    return { media, isOpen, setOpen, openForMedia }
}
