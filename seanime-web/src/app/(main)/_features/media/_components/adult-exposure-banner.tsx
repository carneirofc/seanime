import { useGetAnimeCollection } from "@/api/hooks/anilist.hooks"
import { usePrivatizeAdultEntries } from "@/api/hooks/anilist.hooks"
import { useGetMangaCollection } from "@/api/hooks/manga.hooks"
import { Button } from "@/components/ui/button"
import React from "react"
import { BiLockOpen, BiX } from "react-icons/bi"

type AdultExposureBannerProps = {
    type: "anime" | "manga"
}

/**
 * AdultExposureBanner warns the user when adult (isAdult) list entries are publicly visible
 * (i.e. their AniList `private` flag is not set). It offers a one-click bulk "Make all private"
 * action. The banner is session-dismissible and independent of the "make adult private by default"
 * setting — it reflects real exposure regardless of the policy.
 */
export function AdultExposureBanner({ type }: AdultExposureBannerProps) {
    const dismissKey = `sea-adult-exposure-dismissed-${type}`
    const [dismissed, setDismissed] = React.useState(false)

    React.useEffect(() => {
        try {
            setDismissed(sessionStorage.getItem(dismissKey) === "1")
        } catch {
        }
    }, [dismissKey])

    const { data: animeCollection } = useGetAnimeCollection()
    const { data: mangaCollection } = useGetMangaCollection()

    const { mutate: privatize, isPending } = usePrivatizeAdultEntries(type)

    const exposedCount = React.useMemo(() => {
        const ids = new Set<number>()
        if (type === "anime") {
            for (const list of animeCollection?.MediaListCollection?.lists ?? []) {
                for (const entry of list?.entries ?? []) {
                    if (entry?.media?.isAdult && !entry?.private && entry?.media?.id != null) {
                        ids.add(entry.media.id)
                    }
                }
            }
        } else {
            for (const list of mangaCollection?.lists ?? []) {
                for (const entry of list?.entries ?? []) {
                    if (entry?.media?.isAdult && !entry?.listData?.private && entry?.mediaId != null) {
                        ids.add(entry.mediaId)
                    }
                }
            }
        }
        return ids.size
    }, [type, animeCollection, mangaCollection])

    const handleDismiss = React.useCallback(() => {
        try {
            sessionStorage.setItem(dismissKey, "1")
        } catch {
        }
        setDismissed(true)
    }, [dismissKey])

    if (dismissed || exposedCount === 0) return null

    return (
        <div
            data-adult-exposure-banner
            data-media-type={type}
            className="flex items-center gap-3 rounded-lg border border-orange-500/40 bg-orange-500/10 px-4 py-3 text-sm text-orange-200"
        >
            <BiLockOpen className="text-xl shrink-0" />
            <div className="flex-1">
                <span className="font-medium">
                    {exposedCount} adult {type === "anime" ? "title" : "manga"}{exposedCount === 1 ? "" : "s"} {exposedCount === 1 ? "is" : "are"} publicly visible on AniList.
                </span>
            </div>
            <Button
                intent="warning-subtle"
                size="sm"
                rounded
                loading={isPending}
                onClick={() => privatize({ type })}
            >
                Make all private
            </Button>
            <Button
                intent="gray-subtle"
                size="sm"
                rounded
                leftIcon={<BiX />}
                disabled={isPending}
                onClick={handleDismiss}
            >
                Dismiss
            </Button>
        </div>
    )
}
