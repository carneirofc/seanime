import { HibikeManga_ChapterDetails, HibikeManga_SearchResult } from "@/api/generated/types"
import { useGetMangaEntryChapters, useGetMangaMapping } from "@/api/hooks/manga.hooks"
import { useHandleMangaProviderExtensions } from "@/app/(main)/manga/_lib/handle-manga-providers"
import {
    chapterMatchesScanlator,
    getChapterScanlatorValues,
    useSelectedMangaFilters,
    useSelectedMangaProvider,
} from "@/app/(main)/manga/_lib/handle-manga-selected-provider"
import { LANGUAGES_LIST } from "@/app/(main)/manga/_lib/language-map"
import uniq from "lodash/uniq"
import React from "react"

export function useHandleMangaChapters(
    mediaId: string | null,
) {

    /**
     * 1. Fetch the provider extensions
     */
    const { providerExtensions, providerOptions, providerExtensionsLoading } = useHandleMangaProviderExtensions(mediaId)

    /**
     * 2. Get the selected provider for this entry
     */
    const {
        selectedExtension,
        selectedProvider,
        setSelectedProvider,
    } = useSelectedMangaProvider(mediaId)


    /**
     * 3. Fetch the chapters for this entry
     */
    const {
        data: existingMapping,
        isLoading: mappingLoading,
    } = useGetMangaMapping({
        provider: selectedProvider || undefined,
        mediaId: Number(mediaId),
    })

    const sourceMatchRequired = !!mediaId && !!selectedProvider && !mappingLoading && !existingMapping?.mangaId

    const sourceMatchCandidates: HibikeManga_SearchResult[] = []
    const sourceMatchCandidatesLoading = false
    const sourceMatchCandidatesError = false
    const sourceMatchCandidatesErrorObj = undefined

    const {
        data: chapterContainer,
        isLoading: rawChapterContainerLoading,
        isError: chapterContainerError,
        error: chapterContainerErrorObj,
    } = useGetMangaEntryChapters({
        mediaId: Number(mediaId),
        provider: selectedProvider || undefined,
    }, !!existingMapping?.mangaId)

    // Extract the detailed error message from the server response
    const chapterContainerErrorMessage = React.useMemo(() => {
        if (!chapterContainerError || !chapterContainerErrorObj) return undefined
        const serverError = (chapterContainerErrorObj as any)?.response?.data?.error
        if (typeof serverError === "string") return serverError
        return chapterContainerErrorObj?.message || "Unknown error"
    }, [chapterContainerError, chapterContainerErrorObj])

    const sourceMatchCandidatesErrorMessage: string | undefined = undefined

    const chapterContainerLoading = mappingLoading
        || (sourceMatchRequired && sourceMatchCandidatesLoading)
        || rawChapterContainerLoading

    /**
     * 4. Filters
     */
    const {
        setSelectedScanlator,
        setSelectedLanguage,
        setSelectedSourceProvider,
        selectedFilters,
    } = useSelectedMangaFilters(
        mediaId,
        selectedExtension,
        selectedProvider,
        !chapterContainerLoading,
    )

    /**
     * 5. Filter chapters based on source provider, language and scanlator
     */
    const matchesSelectedFilters = React.useCallback((
        chapter: HibikeManga_ChapterDetails,
        omit: Array<"sourceProvider" | "language" | "scanlator"> = [],
    ) => {
        if (!chapter) return false

        if (!omit.includes("sourceProvider") && selectedFilters.sourceProvider) {
            if (chapter.sourceProvider !== selectedFilters.sourceProvider) return false
        }

        if (!omit.includes("language") && selectedExtension?.settings?.supportsMultiLanguage && selectedFilters.language) {
            if (chapter.language !== selectedFilters.language) return false
        }

        if (!omit.includes("scanlator") && selectedFilters.scanlators[0]) {
            if (!chapterMatchesScanlator(chapter.scanlator, selectedFilters.scanlators[0])) return false
        }

        return true
    }, [selectedExtension, selectedFilters])

    const filteredChapterContainer = React.useMemo(() => {
        if (!chapterContainer) return chapterContainer

        const filteredChapters = chapterContainer.chapters?.filter(ch => matchesSelectedFilters(ch))

        return {
            ...chapterContainer,
            chapters: filteredChapters,
        }
    }, [chapterContainer, matchesSelectedFilters])

    const sourceProviderOptions = React.useMemo(() => {
        const sourceProviders = uniq(chapterContainer?.chapters
            ?.filter(chapter => matchesSelectedFilters(chapter, ["sourceProvider"]))
            ?.map(chapter => chapter.sourceProvider)
            ?.filter(Boolean) || [])

        return sourceProviders.map(sourceProvider => ({ value: sourceProvider, label: sourceProvider }))
    }, [chapterContainer, matchesSelectedFilters])

    React.useEffect(() => {
        if (!selectedFilters.sourceProvider) return
        if (sourceProviderOptions.some(option => option.value === selectedFilters.sourceProvider)) return

        setSelectedSourceProvider({
            mId: mediaId,
            sourceProvider: "",
        })
    }, [mediaId, selectedFilters.sourceProvider, setSelectedSourceProvider, sourceProviderOptions])

    const languageOptions = React.useMemo(() => {
        if (!selectedExtension?.settings?.supportsMultiLanguage) return []

        const languages = uniq(chapterContainer?.chapters
            ?.filter(chapter => matchesSelectedFilters(chapter, ["language"]))
            ?.map(chapter => chapter.language)
            ?.filter(Boolean) || [])

        return languages.map(language => ({
            value: language,
            label: ((LANGUAGES_LIST as any)[language as any] as any)?.nativeName || language,
        }))
    }, [chapterContainer, matchesSelectedFilters, selectedExtension])

    const scanlatorOptions = React.useMemo(() => {
        const scanlators = uniq(chapterContainer?.chapters
            ?.filter(chapter => matchesSelectedFilters(chapter, ["scanlator"]))
            ?.flatMap(chapter => getChapterScanlatorValues(chapter.scanlator)) || [])

        return scanlators.map(scanlator => ({ value: scanlator, label: scanlator }))
    }, [chapterContainer, matchesSelectedFilters])

    return {
        selectedExtension,
        providerExtensions,
        providerExtensionsLoading,
        // Selected provider
        providerOptions, // For dropdown
        selectedProvider, // Current provider
        setSelectedProvider,
        // Filters
        selectedFilters,
        setSelectedLanguage,
        setSelectedScanlator,
        setSelectedSourceProvider,
        sourceProviderOptions,
        languageOptions,
        scanlatorOptions,
        // Chapters
        chapterContainer: filteredChapterContainer,
        chapterContainerLoading,
        chapterContainerError,
        chapterContainerErrorMessage,
        sourceMatchRequired,
        sourceMatchCandidates: sourceMatchCandidates ?? [],
        sourceMatchCandidatesErrorMessage,
        sourceMatchCandidatesLoading,
    }
}
