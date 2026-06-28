import React from "react"

type AL_MediaTag = {
    name: string
    category?: string | null
    isAdult?: boolean
    rank?: number
    isMediaSpoiler?: boolean | null
    isGeneralSpoiler?: boolean | null
}

export function useMediaTags() {
    const tagsForCategory = React.useMemo(() => ({} as Record<string, AL_MediaTag[]>), [])

    return { tagsForCategory, isLoading: false }
}
