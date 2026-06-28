import {
    AL_MangaDetailsById_Media,
    HibikeManga_ChapterDetails,
    HibikeManga_ChapterProviderOption,
    Manga_Entry,
    Manga_MediaDownloadData,
} from "@/api/generated/types"
import { useEmptyMangaEntryCache } from "@/api/hooks/manga.hooks"
import { SeaCommandInjectableItem, useSeaCommandInject } from "@/app/(main)/_features/sea-command/use-inject"
import { ChapterListBulkActions } from "@/app/(main)/manga/_containers/chapter-list/_components/chapter-list-bulk-actions"
import { DownloadedChapterList, manga_downloadedChapterContainerAtom } from "@/app/(main)/manga/_containers/chapter-list/downloaded-chapter-list"
import { MangaManualMappingModal } from "@/app/(main)/manga/_containers/chapter-list/manga-manual-mapping-modal"
import { MangaProviderDiagnosticsDrawer } from "@/app/(main)/manga/_containers/chapter-list/manga-provider-diagnostics-drawer"
import { ChapterReaderDrawer } from "@/app/(main)/manga/_containers/chapter-reader/chapter-reader-drawer"
import { __manga_selectedChapterAtom } from "@/app/(main)/manga/_lib/handle-chapter-reader"
import { useHandleMangaChapters } from "@/app/(main)/manga/_lib/handle-manga-chapters"
import { useHandleDownloadMangaChapter } from "@/app/(main)/manga/_lib/handle-manga-downloads"
import { getChapterNumberFromChapter, useMangaChapterListRowSelection, useMangaDownloadDataUtils } from "@/app/(main)/manga/_lib/handle-manga-utils"
import { LANGUAGES_LIST } from "@/app/(main)/manga/_lib/language-map"
import { monochromeCheckboxClasses } from "@/components/shared/classnames"
import { ConfirmationDialog, useConfirmationDialog } from "@/components/shared/confirmation-dialog"
import { LuffyError } from "@/components/shared/luffy-error"
import { Alert } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button, IconButton } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { DataGrid, defineDataGridColumns } from "@/components/ui/datagrid"
import { DropdownMenu, DropdownMenuItem, DropdownMenuLabel } from "@/components/ui/dropdown-menu"
import { LoadingSpinner } from "@/components/ui/loading-spinner"
import { Select } from "@/components/ui/select"
import { useAtom, useSetAtom } from "jotai/react"
import React from "react"
import { ErrorBoundary } from "react-error-boundary"
import { FaRedo } from "react-icons/fa"
import { GiOpenBook } from "react-icons/gi"
import { IoBookOutline, IoLibrary } from "react-icons/io5"
import { LuDownload, LuExternalLink, LuSearch } from "react-icons/lu"
import { MdOutlineOfflinePin } from "react-icons/md"

type ChapterListProps = {
    mediaId: string | null
    entry: Manga_Entry
    details: AL_MangaDetailsById_Media | undefined
    downloadData: Manga_MediaDownloadData | undefined
    downloadDataLoading: boolean
}

export function ChapterList(props: ChapterListProps) {

    const {
        mediaId,
        entry,
        details,
        downloadData,
        downloadDataLoading,
        ...rest
    } = props

    /**
     * Find chapter container
     */
    const {
        selectedExtension,
        providerExtensions,
        providerExtensionsLoading,
        // Selected provider
        providerOptions, // For dropdown
        selectedProvider, // Current provider (id)
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
        chapterContainer,
        chapterContainerLoading,
        chapterContainerError,
        chapterContainerErrorMessage,
        sourceMatchRequired,
        sourceMatchCandidates,
        sourceMatchCandidatesErrorMessage,
        sourceMatchCandidatesLoading,
    } = useHandleMangaChapters(mediaId)

    const [isManualMatchOpen, setIsManualMatchOpen] = React.useState(false)
    const autoMatchPromptKeyRef = React.useRef<string | null>(null)

    React.useEffect(() => {
        if (!sourceMatchRequired) {
            autoMatchPromptKeyRef.current = null
            setIsManualMatchOpen(false)
            return
        }

        if (sourceMatchCandidatesLoading || !mediaId || !selectedProvider) return

        const promptKey = `${mediaId}:${selectedProvider}`
        if (autoMatchPromptKeyRef.current === promptKey) return

        autoMatchPromptKeyRef.current = promptKey
        setIsManualMatchOpen(true)
    }, [mediaId, selectedProvider, sourceMatchCandidatesLoading, sourceMatchRequired])

    const openManualMatchModal = React.useCallback(() => {
        setIsManualMatchOpen(true)
    }, [])

    const manualMatchButtonLabel = React.useMemo(() => {
        if (!sourceMatchRequired) return "Manual match"
        return sourceMatchCandidates.length > 0 ? "Select source match" : "Search source"
    }, [sourceMatchCandidates.length, sourceMatchRequired])


    // Keep track of chapter numbers as integers
    // This is used to filter the chapters
    // [id]: number
    const chapterIdToNumbersMap = React.useMemo(() => {
        const map = new Map<string, number>()

        for (const chapter of chapterContainer?.chapters ?? []) {
            map.set(chapter.id, getChapterNumberFromChapter(chapter.chapter))
        }

        return map
    }, [chapterContainer?.chapters])

    const [showUnreadChapter, setShowUnreadChapter] = React.useState(false)
    const [showDownloadedChapters, setShowDownloadedChapters] = React.useState(false)

    /**
     * Set selected chapter
     */
    const setSelectedChapter = useSetAtom(__manga_selectedChapterAtom)
    /**
     * Clear manga cache
     */
    const { mutate: clearMangaCache, isPending: isClearingMangaCache } = useEmptyMangaEntryCache()
    /**
     * Download chapter
     */
    const { downloadChapters, isSendingDownloadRequest } = useHandleDownloadMangaChapter(mediaId)
    /**
     * Download data utils
     */
    const {
        isChapterQueued,
        isChapterDownloaded,
        isProviderChapterDownloaded,
        isProviderChapterQueued,
        isChapterLocal,
    } = useMangaDownloadDataUtils(downloadData, downloadDataLoading)

    const { inject, remove } = useSeaCommandInject()

    const providerNameMap = React.useMemo(() => {
        return new Map(providerExtensions?.map(extension => [extension.id, extension.name]) ?? [])
    }, [providerExtensions])

    const hasChapterSourceProviders = React.useMemo(() => {
        return sourceProviderOptions.length > 0 || !!selectedFilters.sourceProvider
    }, [selectedFilters.sourceProvider, sourceProviderOptions])

    const hasChapterScanlators = React.useMemo(() => {
        return scanlatorOptions.length > 0 || !!selectedFilters.scanlators[0]
    }, [scanlatorOptions, selectedFilters.scanlators])

    const getProviderLabel = React.useCallback((provider: string) => {
        return providerNameMap.get(provider) || provider
    }, [providerNameMap])

    const getChapterProviderChoices = React.useCallback((chapter: HibikeManga_ChapterDetails): HibikeManga_ChapterProviderOption[] => {
        const options: HibikeManga_ChapterProviderOption[] = []
        const seen = new Set<string>()

        const pushOption = (option: HibikeManga_ChapterProviderOption | undefined) => {
            if (!option || option.provider === "local-manga") return

            const key = `${option.provider}:${option.chapterId}`
            if (seen.has(key)) return

            seen.add(key)
            options.push(option)
        }

        pushOption({
            provider: chapter.provider,
            chapterId: chapter.id,
            language: chapter.language,
            scanlator: chapter.scanlator,
        })

        chapter.alternativeProviders?.forEach(pushOption)

        return options
    }, [])

    const getChapterProviderCount = React.useCallback((chapter: HibikeManga_ChapterDetails) => {
        const providers = new Set<string>()

        if (chapter.provider) {
            providers.add(chapter.provider)
        }

        chapter.alternativeProviders?.forEach(option => {
            if (option.provider) {
                providers.add(option.provider)
            }
        })

        return providers.size
    }, [])

    const getProviderOptionLabel = React.useCallback((option: HibikeManga_ChapterProviderOption) => {
        const details = [
            option.language ? (LANGUAGES_LIST[option.language]?.nativeName || option.language) : null,
            option.scanlator || null,
        ].filter(Boolean)

        if (details.length === 0) {
            return getProviderLabel(option.provider)
        }

        return `${getProviderLabel(option.provider)} · ${details.join(" · ")}`
    }, [getProviderLabel])

    /**
     * Function to filter unread chapters
     */
    const retainUnreadChapters = React.useCallback((chapter: HibikeManga_ChapterDetails) => {
        if (!entry.listData || !chapterIdToNumbersMap.has(chapter.id) || !entry.listData?.progress) return true

        const chapterNumber = chapterIdToNumbersMap.get(chapter.id)
        return !!chapterNumber && chapterNumber > entry.listData?.progress
    }, [chapterIdToNumbersMap, chapterContainer, entry])

    const confirmReloadSource = useConfirmationDialog({
        title: "Reload sources",
        actionIntent: "primary",
        actionText: "Reload",
        description: "This action will empty the cache for this manga and fetch the latest data from the selected source.",
        onConfirm: () => {
            if (mediaId) {
                clearMangaCache({ mediaId: Number(mediaId) })
            }
        },
    })

    /**
     * Chapter columns
     */
    const columns = React.useMemo(() => defineDataGridColumns<HibikeManga_ChapterDetails>(() => [
        {
            accessorKey: "title",
            header: "Name",
            size: 90,
            cell: ({ row }) => {
                const providerCount = getChapterProviderCount(row.original)
                const title = row.original.title || `Chapter ${row.original.chapter}`

                return (
                    <div className="flex items-center gap-2 min-w-0">
                        <span className="truncate">{title}</span>
                        {providerCount > 1 && (
                            <Badge intent="info" size="sm" className="shrink-0">
                                {providerCount} providers
                            </Badge>
                        )}
                    </div>
                )
            },
        },
        ...(hasChapterSourceProviders ? [{
            id: "sourceProvider",
            header: "Source",
            size: 36,
            accessorFn: (row: HibikeManga_ChapterDetails) => row.sourceProvider,
            enableSorting: true,
            cell: ({ getValue }: any) => {
                const value = getValue()
                return value ? <span className="text-sm text-[--muted]">{value}</span> : null
            },
        }] : []),
        ...(hasChapterScanlators ? [{
            id: "scanlator",
            header: "Scanlator",
            size: 30,
            accessorFn: (row: any) => row.scanlator,
            enableSorting: true,
            cell: ({ getValue }: any) => <span className="text-sm text-[--muted]">{getValue()}</span>,
        }] : []),
        ...(selectedExtension?.settings?.supportsMultiLanguage ? [{
            id: "language",
            header: "Language",
            size: 40,
            accessorFn: (row: any) => LANGUAGES_LIST[row.language]?.nativeName || row.language,
            enableSorting: true,
            cell: ({ getValue }: any) => <span className="text-sm text-[--muted]">{getValue()}</span>,
        }] : []),
        {
            id: "number",
            header: "Number",
            size: 10,
            enableSorting: true,
            accessorFn: (row) => {
                return chapterIdToNumbersMap.get(row.id)
            },
        },
        {
            id: "_actions",
            size: 10,
            enableSorting: false,
            enableGlobalFilter: false,
            cell: ({ row }) => {
                const providerChoices = getChapterProviderChoices(row.original)
                const hasAlternativeProviders = (row.original.alternativeProviders?.length ?? 0) > 0

                return (
                    <div className="flex justify-end gap-2 items-center w-full">
                        {hasAlternativeProviders ? (
                            <DropdownMenu
                                allowOutsideInteraction
                                trigger={
                                    <IconButton
                                        intent="gray-basic"
                                        size="sm"
                                        disabled={providerChoices.length === 0}
                                        icon={<LuDownload className="text-xl" />}
                                        className="opacity-50 hover:opacity-100"
                                    />
                                }
                            >
                                <DropdownMenuLabel>Download from</DropdownMenuLabel>
                                {providerChoices.map(option => {
                                    const isDownloaded = isProviderChapterDownloaded(option.provider, option.chapterId)
                                    const isQueued = isProviderChapterQueued(option.provider, option.chapterId)

                                    return (
                                        <DropdownMenuItem
                                            key={`${option.provider}:${option.chapterId}`}
                                            disabled={isDownloaded || isQueued || isSendingDownloadRequest}
                                            onClick={() => downloadChapters([option])}
                                        >
                                            <div className="flex items-center gap-2 w-full">
                                                <span className="truncate">{getProviderOptionLabel(option)}</span>
                                                <span className="ml-auto text-xs text-[--muted]">
                                                    {isDownloaded ? "Downloaded" : isQueued ? "Queued" : ""}
                                                </span>
                                            </div>
                                        </DropdownMenuItem>
                                    )
                                })}
                            </DropdownMenu>
                        ) : ((!isChapterLocal(row.original) && !isChapterDownloaded(row.original) && !isChapterQueued(row.original)) && <IconButton
                            intent="gray-basic"
                            size="sm"
                            disabled={isSendingDownloadRequest}
                            onClick={() => downloadChapters([row.original])}
                            icon={<LuDownload className="text-xl" />}
                            className="opacity-50 hover:opacity-100"
                        />)}
                        {isChapterQueued(row.original) && <p className="text-[--muted]">Queued</p>}
                        {isChapterDownloaded(row.original) && <p className="text-[--green] px-1"><MdOutlineOfflinePin className="text-2xl" /></p>}
                        {row.original.url && (
                            <a href={row.original.url} target="_blank" rel="noopener noreferrer">
                                <IconButton
                                    intent="gray-basic"
                                    size="sm"
                                    icon={<LuExternalLink className="text-lg" />}
                                    className="opacity-50 hover:opacity-100"
                                />
                            </a>
                        )}
                        <IconButton
                            intent="gray-subtle"
                            size="md"
                            onClick={() => setSelectedChapter({
                                chapterId: row.original.id,
                                chapterNumber: row.original.chapter,
                                provider: row.original.provider,
                                mediaId: Number(mediaId),
                            })}
                            icon={<GiOpenBook />}
                        />
                    </div>
                )
            },
        },
    ]), [
        chapterIdToNumbersMap,
        downloadChapters,
        getChapterProviderChoices,
        getChapterProviderCount,
        getProviderOptionLabel,
        hasChapterScanlators,
        isChapterDownloaded,
        isChapterLocal,
        isChapterQueued,
        isProviderChapterDownloaded,
        isProviderChapterQueued,
        isSendingDownloadRequest,
        mediaId,
        hasChapterSourceProviders,
        selectedExtension,
    ])

    const unreadChapters = React.useMemo(() => chapterContainer?.chapters?.filter(ch => retainUnreadChapters(ch)) ?? [], [chapterContainer, entry])
    const allChapters = React.useMemo(() => chapterContainer?.chapters?.toReversed() ?? [], [chapterContainer])

    /**
     * Set "showUnreadChapter" state if there are unread chapters
     */
    React.useLayoutEffect(() => {
        setShowUnreadChapter(!!unreadChapters.length)
    }, [unreadChapters?.length])

    /**
     * Filter chapters based on state
     */
    const chapters = React.useMemo(() => {
        let d = showUnreadChapter ? unreadChapters : allChapters
        if (showDownloadedChapters) {
            d = d.filter(ch => isChapterDownloaded(ch) || isChapterQueued(ch))
        }
        return d
    }, [
        showUnreadChapter, unreadChapters, allChapters, showDownloadedChapters, downloadData, selectedExtension,
    ])

    const {
        rowSelectedChapters,
        onRowSelectionChange,
        rowSelection,
        setRowSelection,
        resetRowSelection,
        // setSelectedChapters,
    } = useMangaChapterListRowSelection()

    React.useEffect(() => {
        resetRowSelection()
    }, [])

    // Inject chapter list command
    React.useEffect(() => {
        if (!chapterContainer?.chapters?.length) return

        const nextChapter = unreadChapters[0]
        const upcomingChapters = unreadChapters.slice(0, 10)

        const commandItems: SeaCommandInjectableItem[] = [
            // Next chapter
            ...(nextChapter ? [{
                data: nextChapter,
                id: `next-chapter-${nextChapter.id}`,
                value: `${nextChapter.chapter}`,
                heading: "Next Chapter",
                priority: 2,
                render: () => (
                    <div className="flex gap-1 items-center w-full">
                        <p className="max-w-[70%] truncate">Chapter {nextChapter.chapter}</p>
                        {nextChapter.scanlator && (
                            <p className="text-[--muted]">({nextChapter.scanlator})</p>
                        )}
                    </div>
                ),
                onSelect: ({ ctx }) => {
                    setSelectedChapter({
                        chapterId: nextChapter.id,
                        chapterNumber: nextChapter.chapter,
                        provider: nextChapter.provider,
                        mediaId: Number(mediaId),
                    })
                    ctx.close()
                },
            } as SeaCommandInjectableItem] : []),
            // Upcoming chapters
            ...upcomingChapters.map(chapter => ({
                data: chapter,
                id: `chapter-${chapter.id}`,
                value: `${chapter.chapter}`,
                heading: "Upcoming Chapters",
                priority: 1,
                render: () => (
                    <div className="flex gap-1 items-center w-full">
                        <p className="max-w-[70%] truncate">Chapter {chapter.chapter}</p>
                        {chapter.scanlator && (
                            <p className="text-[--muted]">({chapter.scanlator})</p>
                        )}
                    </div>
                ),
                onSelect: ({ ctx }) => {
                    setSelectedChapter({
                        chapterId: chapter.id,
                        chapterNumber: chapter.chapter,
                        provider: chapter.provider,
                        mediaId: Number(mediaId),
                    })
                    ctx.close()
                },
            } as SeaCommandInjectableItem)),
        ]

        inject("manga-chapters", {
            items: commandItems,
            filter: ({ item, input }) => {
                if (!input) return true
                return item.value.toLowerCase().includes(input.toLowerCase()) ||
                    (item.data.title?.toLowerCase() || "").includes(input.toLowerCase())
            },
            priority: 100,
        })

        return () => remove("manga-chapters")
    }, [chapterContainer?.chapters, unreadChapters, mediaId])

    const [downloadedChapterContainer] = useAtom(manga_downloadedChapterContainerAtom)

    if (providerExtensionsLoading) return <LoadingSpinner />

    return (
        <div
            className="space-y-4"
            data-chapter-list-container
            data-selected-filters={JSON.stringify(selectedFilters)}
            data-selected-provider={JSON.stringify(selectedProvider)}
        >
            <MangaManualMappingModal
                entry={entry}
                open={isManualMatchOpen}
                onOpenChange={setIsManualMatchOpen}
                initialSearchResults={sourceMatchRequired ? sourceMatchCandidates : undefined}
                title={sourceMatchRequired ? "Select source match" : undefined}
                description={sourceMatchRequired
                    ? "Choose the correct manga result from this provider before loading chapters."
                    : undefined}
            />

            <div data-chapter-list-header-container className="flex flex-wrap gap-2 items-center">
                <Select
                    fieldClass="w-fit"
                    options={providerOptions}
                    value={selectedProvider || ""}
                    onValueChange={v => setSelectedProvider({
                        mId: mediaId,
                        provider: v,
                    })}
                    leftAddon="Source"
                    size="sm"
                    disabled={isClearingMangaCache}
                />

                <MangaProviderDiagnosticsDrawer provider={selectedProvider} mediaId={mediaId ? Number(mediaId) : null} />

                <Button
                    leftIcon={<FaRedo />}
                    intent="gray-outline"
                    onClick={() => confirmReloadSource.open()}
                    loading={isClearingMangaCache}
                    size="sm"
                >
                    Reload sources
                </Button>

                <Button
                    leftIcon={<LuSearch className="text-lg" />}
                    intent="gray-outline"
                    size="sm"
                    onClick={openManualMatchModal}
                >
                    {manualMatchButtonLabel}
                </Button>

                {!!chapterContainer?.chapters?.length && (() => {
                    const firstUrl = chapterContainer.chapters[0]?.url
                    if (!firstUrl) return null
                    let hostname: string | null = null
                    try { hostname = new URL(firstUrl).hostname.replace(/^www\./, "") } catch { /* noop */ }
                    return (
                        <a href={firstUrl} target="_blank" rel="noopener noreferrer">
                            <Button
                                leftIcon={<LuExternalLink className="text-base" />}
                                intent="gray-outline"
                                size="sm"
                            >
                                {hostname || "Open source"}
                            </Button>
                        </a>
                    )
                })()}
            </div>

            <ErrorBoundary
                fallbackRender={({ error }) => <Alert
                    intent="alert"
                    title="Client side error"
                    description={`Could not load chapter filters. Please contact the extension developer: "${error}"`}
                />}
            >
                {(hasChapterSourceProviders || hasChapterScanlators || selectedExtension?.settings?.supportsMultiLanguage) && (
                    <div data-chapter-list-header-filters-container className="flex flex-wrap gap-2 items-center">
                        {hasChapterSourceProviders && sourceProviderOptions.length > 0 && (
                            <Select
                                fieldClass="w-64"
                                options={sourceProviderOptions}
                                placeholder="All"
                                value={selectedFilters.sourceProvider}
                                onValueChange={v => setSelectedSourceProvider({
                                    mId: mediaId,
                                    sourceProvider: v,
                                })}
                                leftAddon="Source"
                            />
                        )}
                        {hasChapterScanlators && (
                            <>
                                <Select
                                    fieldClass="w-64"
                                    options={scanlatorOptions}
                                    placeholder="All"
                                    value={selectedFilters.scanlators[0] || ""}
                                    onValueChange={v => setSelectedScanlator({
                                        mId: mediaId,
                                        scanlators: [v],
                                    })}
                                    leftAddon="Scanlator"
                                    // intent="filled"
                                    // size="sm"
                                />
                            </>
                        )}
                        {selectedExtension?.settings?.supportsMultiLanguage && (
                            <Select
                                fieldClass="w-64"
                                options={languageOptions}
                                placeholder="All"
                                value={selectedFilters.language}
                                onValueChange={v => setSelectedLanguage({
                                    mId: mediaId,
                                    language: v,
                                })}
                                leftAddon="Language"
                                // intent="filled"
                                // size="sm"
                            />
                        )}
                    </div>
                )}
            </ErrorBoundary>

            {(chapterContainerLoading || isClearingMangaCache) ? <LoadingSpinner /> : (
                sourceMatchRequired ? <div className="space-y-4">
                    <LuffyError title={sourceMatchCandidates.length > 0 ? "Select a source match" : "Source match required"}>
                        <p className="text-sm text-[--muted] max-w-lg text-center mt-2">
                            {sourceMatchCandidates.length > 0
                                ? `We found ${sourceMatchCandidates.length} possible matches from ${selectedExtension?.name || selectedProvider}. Choose the correct result before loading chapters.`
                                : (sourceMatchCandidatesErrorMessage || "We couldn't automatically confirm a source match. Search the selected provider and choose the correct manga to continue.")}
                        </p>
                        <div className="flex gap-2 items-center mt-3">
                            <Button
                                leftIcon={<LuSearch className="text-lg" />}
                                intent="gray-outline"
                                size="md"
                                onClick={openManualMatchModal}
                            >
                                {sourceMatchCandidates.length > 0 ? "Review matches" : "Search manually"}
                            </Button>
                        </div>
                    </LuffyError>
                </div> : chapterContainerError ? <div className="space-y-4">
                    <LuffyError title="No chapters found">
                        {chapterContainerErrorMessage && (
                            <p className="text-sm text-[--muted] max-w-lg text-center mt-2">
                                {chapterContainerErrorMessage}
                            </p>
                        )}
                        <div className="flex gap-2 items-center mt-3">
                            <Button
                                leftIcon={<LuSearch className="text-lg" />}
                                intent="gray-outline"
                                size="md"
                                onClick={openManualMatchModal}
                            >
                                Manual match
                            </Button>
                        </div>
                    </LuffyError>
                </div> : (
                    <>

                        {chapterContainer?.chapters?.length === 0 && (
                            <LuffyError title="No chapters found"><p>Try another source</p></LuffyError>
                        )}

                        {!!chapterContainer?.chapters?.length && (
                            <>
                                <div data-chapter-list-header-container className="flex gap-2 items-center w-full pb-2">
                                    <h2 className="px-1">Chapters</h2>
                                    <div className="flex flex-1"></div>
                                    <div>
                                        {!!unreadChapters?.length && <Button
                                            intent="white"
                                            rounded
                                            leftIcon={<IoBookOutline />}
                                            disabled={!unreadChapters?.length || (!!entry.listData?.progress && parseInt(unreadChapters[0].chapter) !== entry.listData?.progress + 1)}
                                            onClick={() => {
                                                setSelectedChapter({
                                                    chapterId: unreadChapters[0].id,
                                                    chapterNumber: unreadChapters[0].chapter,
                                                    provider: unreadChapters[0].provider,
                                                    mediaId: Number(mediaId),
                                                })
                                            }}
                                        >
                                            {!!entry.listData?.progress ? "Continue reading" : "Start reading"}
                                        </Button>}
                                    </div>
                                </div>

                                {/* <ChapterListTable
                                 chapters={chapters}
                                 rowSelection={rowSelection}
                                 setRowSelection={setRowSelection}
                                 setSelectedChapters={setSelectedChapters}
                                 onChapterClick={(chapter) => {
                                 setSelectedChapter({
                                 chapterId: chapter.id,
                                 chapterNumber: chapter.chapter,
                                 provider: chapter.provider,
                                 mediaId: Number(mediaId),
                                 })
                                 }}
                                 onDownloadChapter={(chapter) => downloadChapters([chapter])}
                                 isChapterQueued={isChapterQueued}
                                 isChapterDownloaded={isChapterDownloaded}
                                 isChapterLocal={isChapterLocal}
                                 /> */}

                                <div data-chapter-list-bulk-actions-container className="space-y-4 rounded-2xl border bg-[--paper] p-4">

                                    <div data-chapter-list-bulk-actions-checkboxes-container className="flex flex-wrap items-center gap-4">
                                        <Checkbox
                                            label="Show unread"
                                            value={showUnreadChapter}
                                            onValueChange={v => setShowUnreadChapter(v as boolean)}
                                            fieldClass="w-fit"
                                            {...monochromeCheckboxClasses}
                                        />
                                        {selectedProvider !== "local-manga" && <Checkbox
                                            label={<span className="flex gap-2 items-center"><IoLibrary /> Show downloaded</span>}
                                            value={showDownloadedChapters}
                                            onValueChange={v => setShowDownloadedChapters(v as boolean)}
                                            fieldClass="w-fit"
                                            {...monochromeCheckboxClasses}
                                        />}
                                    </div>

                                    <ChapterListBulkActions
                                        rowSelectedChapters={rowSelectedChapters}
                                        onDownloadSelected={chapters => {
                                            downloadChapters(chapters)
                                            resetRowSelection()
                                        }}
                                    />

                                    <DataGrid<HibikeManga_ChapterDetails>
                                        columns={columns}
                                        data={chapters}
                                        rowCount={chapters.length}
                                        isLoading={chapterContainerLoading}
                                        rowSelectionPrimaryKey="id"
                                        enableRowSelection={row => (!isChapterDownloaded(row.original) && !isChapterQueued(row.original))}
                                        initialState={{
                                            pagination: {
                                                pageIndex: 0,
                                                pageSize: 10,
                                            },
                                        }}
                                        state={{
                                            rowSelection,
                                        }}
                                        hideColumns={[
                                            {
                                                below: 800,
                                                hide: ["number"],
                                            },
                                            {
                                                below: 600,
                                                hide: ["sourceProvider", "scanlator", "language"],
                                            },
                                        ]}
                                        onRowSelect={onRowSelectionChange}
                                        onRowSelectionChange={setRowSelection}
                                        className=""
                                        tableClass="table-fixed lg:table-fixed"
                                        tableBodyClass="border-0"
                                        tdClass="border-[rgba(255,255,255,0.05)]"
                                        // tableBodyClass="divide-0 space-y-2"
                                        // trClass="p-3 border-0 bg-[--paper] rounded-lg"
                                        // tdClass="p-3 border-0 rounded-lg"
                                    />
                                </div>
                            </>
                        )}

                    </>
                )
            )}

            {(chapterContainer || downloadedChapterContainer) && <ChapterReaderDrawer
                entry={entry}
                chapterContainer={chapterContainer || downloadedChapterContainer!}
                chapterIdToNumbersMap={chapterIdToNumbersMap}
            />}

            <DownloadedChapterList
                entry={entry}
                data={downloadData}
            />

            <ConfirmationDialog {...confirmReloadSource} />
        </div>
    )
}
