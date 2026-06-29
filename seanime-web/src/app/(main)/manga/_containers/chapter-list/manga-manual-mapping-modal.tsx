import { HibikeManga_SearchResult, Manga_Entry } from "@/api/generated/types"
import { useGetMangaMapping, useMangaManualMapping, useMangaManualSearch, useRemoveMangaMapping } from "@/api/hooks/manga.hooks"
import { useSelectedMangaProvider } from "@/app/(main)/manga/_lib/handle-manga-selected-provider"
import { useMangaReaderUtils } from "@/app/(main)/manga/_lib/handle-manga-utils"
import { ConfirmationDialog, useConfirmationDialog } from "@/components/shared/confirmation-dialog"
import { imageShimmer } from "@/components/shared/image-helpers"
import { SeaImage } from "@/components/shared/sea-image"
import { AppLayoutStack } from "@/components/ui/app-layout"
import { Button } from "@/components/ui/button"
import { cn } from "@/components/ui/core/styling"
import { defineSchema, Field, Form, InferType } from "@/components/ui/form"
import { LoadingSpinner } from "@/components/ui/loading-spinner"
import { Modal } from "@/components/ui/modal"
import { Separator } from "@/components/ui/separator"
import { Tooltip } from "@/components/ui/tooltip"
import React from "react"
import { FiExternalLink, FiSearch } from "react-icons/fi"

type MangaManualMappingModalProps = {
    entry: Manga_Entry
    children?: React.ReactElement
    open?: boolean
    onOpenChange?: (open: boolean) => void
    initialSearchResults?: Array<HibikeManga_SearchResult>
    title?: string
    description?: string
}

export function MangaManualMappingModal(props: MangaManualMappingModalProps) {

    const {
        children,
        entry,
        open,
        onOpenChange,
        initialSearchResults,
        title,
        description,
        ...rest
    } = props

    return (
        <>
            <Modal
                data-manga-manual-mapping-modal
                title={title ?? "Manual match"}
                description={description ?? "Match this manga to a search result"}
                trigger={children}
                open={open}
                onOpenChange={onOpenChange}
                contentClass="max-w-4xl"
            >
                <Content
                    entry={entry}
                    initialSearchResults={initialSearchResults}
                    open={open}
                    onOpenChange={onOpenChange}
                />
            </Modal>
        </>
    )
}

const searchSchema = defineSchema(({ z }) => z.object({
    query: z.string().min(1),
}))

function Content({
    entry,
    initialSearchResults,
    open,
    onOpenChange,
}: {
    entry: Manga_Entry
    initialSearchResults?: Array<HibikeManga_SearchResult>
    open?: boolean
    onOpenChange?: (open: boolean) => void
}) {
    const { selectedProvider } = useSelectedMangaProvider(entry.mediaId)
    const { getChapterPageUrl, isReady: imageProxyReady } = useMangaReaderUtils()

    // Get current mapping
    const { data: existingMapping, isLoading: mappingLoading } = useGetMangaMapping({
        provider: selectedProvider || undefined,
        mediaId: entry.mediaId,
    })

    // Search
    const { mutate: search, data: searchResults, isPending: searchLoading, reset } = useMangaManualSearch(entry.mediaId, selectedProvider)

    function handleSearch(data: InferType<typeof searchSchema>) {
        if (selectedProvider) {
            search({
                provider: selectedProvider,
                query: data.query,
            })
        }
    }

    const getSearchResultImageUrl = React.useCallback((image: string | undefined, headers?: Record<string, string>) => {
        if (!image) return "/no-cover.png"
        return imageProxyReady ? getChapterPageUrl(image, false, headers) : "/no-cover.png"
    }, [getChapterPageUrl, imageProxyReady])

    // Match
    const { mutate: match, isPending: isMatching } = useMangaManualMapping()

    // Unmatch
    const { mutate: unmatch, isPending: isUnmatching } = useRemoveMangaMapping()

    const [selectedMatch, setSelectedMatch] = React.useState<HibikeManga_SearchResult | null>(null)
    const isAutomaticMatchFlow = initialSearchResults !== undefined
    const displaySearchResults = searchResults ?? initialSearchResults ?? []

    React.useEffect(() => {
        if (typeof open !== "boolean" || open) return
        reset()
        setSelectedMatch(null)
    }, [open, reset])

    const helperText = React.useMemo(() => {
        if (isAutomaticMatchFlow) {
            if (displaySearchResults.length > 0) {
                return `We found ${displaySearchResults.length} possible matches. Choose one to continue, or search manually below.`
            }
            return "No automatic matches were confirmed. Search manually below and choose the correct result to continue."
        }
        return "Search the selected provider and confirm the correct result."
    }, [displaySearchResults.length, isAutomaticMatchFlow])

    const confirmMatch = useConfirmationDialog({
        title: isAutomaticMatchFlow ? "Confirm source match" : "Manual match",
        description: selectedMatch
            ? `Use "${selectedMatch.title}"${selectedMatch.year ? ` (${selectedMatch.year})` : ""}${selectedMatch.url ? ` — ${selectedMatch.url}` : ""} as the source match?`
            : "Are you sure you want to match this manga to the search result?",
        actionText: "Confirm",
        actionIntent: "success",
        onConfirm: () => {
            if (selectedMatch && selectedProvider) {
                match({
                    provider: selectedProvider,
                    mediaId: entry.mediaId,
                    mangaId: selectedMatch.id,
                }, {
                    onSuccess: () => {
                        reset()
                        setSelectedMatch(null)
                        onOpenChange?.(false)
                    },
                })
            }
        },
    })

    return (
        <>
            {mappingLoading ? (
                <LoadingSpinner />
            ) : (
                <AppLayoutStack>
                    <div className="text-center">
                        {!!existingMapping?.mangaId ? (
                            <AppLayoutStack>
                                <p>
                                    Current mapping: <span>{existingMapping.mangaId}</span>
                                </p>
                                <Button
                                    intent="alert-subtle" loading={isUnmatching} onClick={() => {
                                    if (selectedProvider) {
                                        unmatch({
                                            provider: selectedProvider,
                                            mediaId: entry.mediaId,
                                        })
                                    }
                                }}
                                >
                                    Remove mapping
                                </Button>
                            </AppLayoutStack>
                        ) : (
                            <p className="text-(--muted) italic">No manual match</p>
                        )}
                    </div>

                    <Separator />

                    <p className="text-sm text-(--muted) text-center">
                        {helperText}
                    </p>

                    <Form schema={searchSchema} onSubmit={handleSearch}>
                        <div className="flex gap-2 items-center">
                            <Field.Text
                                name="query"
                                placeholder="Enter a title..."
                                leftIcon={<FiSearch className="text-xl text-(--muted)" />}
                                fieldClass="w-full"
                            />

                            <Field.Submit intent="white" loading={isMatching || searchLoading || mappingLoading} className="">Search</Field.Submit>
                        </div>
                    </Form>

                    {searchLoading ? <LoadingSpinner /> : (
                        <>
                            {displaySearchResults.length === 0 && (searchResults !== undefined || isAutomaticMatchFlow) && (
                                <p className="text-sm text-(--muted) text-center">
                                    {isAutomaticMatchFlow
                                        ? "No automatic matches available right now. Search manually to continue."
                                        : "No search results found."}
                                </p>
                            )}

                            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-5 gap-2">
                                {displaySearchResults.map(item => (
                                    <div
                                        key={item.id}
                                        className={cn(
                                            "group/sr-item col-span-1 aspect-6/7 rounded-md relative bg-(--background) cursor-pointer transition-opacity",
                                        )}
                                        onClick={() => {
                                            setSelectedMatch(item)
                                            React.startTransition(() => {
                                                confirmMatch.open()
                                            })
                                        }}
                                    >

                                        {<SeaImage
                                            src={getSearchResultImageUrl(item.image, item.imageHeaders)}
                                            placeholder={imageShimmer(700, 475)}
                                            sizes="10rem"
                                            fill
                                            alt=""
                                            className={cn(
                                                "object-center object-cover lg:opacity-50 rounded-md transition-opacity lg:group-hover/sr-item:opacity-100",
                                            )}
                                        />}
                                        {item.url && (
                                            <a
                                                href={item.url}
                                                target="_blank"
                                                rel="noopener noreferrer"
                                                className="absolute top-1 right-1 z-15 p-1 rounded-md bg-black/60 text-white hover:bg-black/80 transition-colors"
                                                onClick={(e) => e.stopPropagation()}
                                                title="Open in browser"
                                            >
                                                <FiExternalLink className="text-sm" />
                                            </a>
                                        )}
                                        <Tooltip
                                            trigger={<div className="absolute m-2 bottom-0 z-10 w-[calc(100%-1rem)]">
                                                <p className="line-clamp-2 text-sm font-semibold">
                                                    {item.title} {item.year && `(${item.year})`}
                                                </p>
                                                <p className="text-xs text-(--muted) mt-0.5">ID: {item.id}</p>
                                            </div>}
                                            className="z-150"
                                        >
                                            <div className="max-w-xs">
                                                <p className="font-semibold">
                                                    {item.title} {item.year && `(${item.year})`}
                                                </p>
                                                <p className="text-sm text-(--muted) mt-1">ID: {item.id}</p>
                                                {item.url && (
                                                    <p className="text-sm text-(--muted) mt-0.5 break-all">{item.url}</p>
                                                )}
                                                {item.synonyms && item.synonyms.length > 0 && (
                                                    <p className="text-sm text-(--muted) mt-0.5">
                                                        Alt: {item.synonyms.join(", ")}
                                                    </p>
                                                )}
                                            </div>
                                        </Tooltip>
                                        <div
                                            className="z-5 absolute rounded-br-md rounded-bl-md bottom-0 w-full h-[80%] bg-linear-to-t from-(--background) to-transparent"
                                        />
                                    </div>
                                ))}
                            </div>
                        </>
                    )}

                </AppLayoutStack>
            )}

            <ConfirmationDialog {...confirmMatch} />
        </>
    )
}
