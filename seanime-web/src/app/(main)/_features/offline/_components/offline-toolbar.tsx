import { PageWrapper } from "@/components/shared/page-wrapper"
import React from "react"

type OfflineLibraryToolbarProps = {
    hasEntries: boolean
    isNakamaLibrary: boolean
    isStreamingOnly: boolean
}

export function OfflineLibraryToolbar(props: OfflineLibraryToolbarProps) {
    const { hasEntries, isNakamaLibrary, isStreamingOnly } = props

    if (!hasEntries) return null

    return (
        <PageWrapper className="py-2">
            <div className="flex items-center gap-2">
                {isNakamaLibrary && (
                    <span className="text-sm text-(--muted)">Nakama Library</span>
                )}
                {isStreamingOnly && (
                    <span className="text-sm text-(--muted)">Streaming Only</span>
                )}
            </div>
        </PageWrapper>
    )
}
