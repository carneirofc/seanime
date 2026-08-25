import { Manga_LocalMangaScanResult, Manga_LocalMangaSeries, Manga_LocalMangaUploadResult } from "@/api/generated/types"
import {
    useDeleteLocalMangaSeries,
    useDownloadLocalManga,
    useGetLocalMangaChapters,
    useGetLocalMangaLibrary,
    useMapLocalMangaSeries,
    useRepackLocalMangaSeries,
    useScanLocalMangaLibrary,
    useUploadLocalMangaArchive,
} from "@/api/hooks/manga-local.hooks"
import { useGetMangaCollection } from "@/api/hooks/manga.hooks"
import { ConfirmationDialog, useConfirmationDialog } from "@/components/shared/confirmation-dialog"
import { Alert } from "@/components/ui/alert"
import { AppLayoutStack } from "@/components/ui/app-layout"
import { Badge } from "@/components/ui/badge"
import { Button, IconButton } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Combobox } from "@/components/ui/combobox"
import { cn } from "@/components/ui/core/styling"
import { Disclosure, DisclosureContent, DisclosureItem, DisclosureTrigger } from "@/components/ui/disclosure"
import { LoadingSpinner } from "@/components/ui/loading-spinner"
import { Modal } from "@/components/ui/modal"
import { Separator } from "@/components/ui/separator"
import { SimpleDropzone } from "@/components/ui/simple-dropzone"
import { TextInput } from "@/components/ui/text-input"
import { Tooltip } from "@/components/ui/tooltip"
import React from "react"
import { BiTrash } from "react-icons/bi"
import { LuChevronDown, LuDownload, LuFolderSearch, LuPackage, LuUpload } from "react-icons/lu"

// Mirrors MaxLocalMangaUploadSize in internal/manga/local_upload.go. Checked here
// only so an oversized file is rejected before it is uploaded, not after.
const MAX_UPLOAD_BYTES = 2 * 1024 * 1024 * 1024

type LocalMangaLibraryModalProps = {
    open: boolean
    onOpenChange: (open: boolean) => void
    returnFocusRef: React.RefObject<HTMLButtonElement | null>
}

export function LocalMangaLibraryModal({ open, onOpenChange, returnFocusRef }: LocalMangaLibraryModalProps) {
    return (
        <Modal
            data-local-manga-library-modal
            open={open}
            onOpenChange={onOpenChange}
            title="Local manga library"
            description="Upload archives and match your local series to your AniList entries."
            contentClass="max-w-3xl"
            onCloseAutoFocus={event => {
                event.preventDefault()
                returnFocusRef.current?.focus()
            }}
        >
            {open && <Content />}
        </Modal>
    )
}

function Content() {
    const { data: library, isLoading } = useGetLocalMangaLibrary()
    const { data: collection } = useGetMangaCollection()

    const entryOptions = React.useMemo(() => {
        const seen = new Set<number>()
        const options: { value: string, textValue: string, label: React.ReactNode }[] = []

        for (const list of collection?.lists ?? []) {
            for (const entry of list?.entries ?? []) {
                const title = entry?.media?.title?.userPreferred || entry?.media?.title?.romaji
                if (!entry?.mediaId || !title || seen.has(entry.mediaId)) continue
                seen.add(entry.mediaId)
                options.push({ value: String(entry.mediaId), textValue: title, label: title })
            }
        }

        return options.sort((a, b) => a.textValue.localeCompare(b.textValue))
    }, [collection])

    if (isLoading) return <LoadingSpinner />

    return (
        <AppLayoutStack className="mt-2">
            {!library?.configured && (
                <Alert
                    intent="info-basic"
                    description="No local source directory is set, so uploads go to Seanime's own manga directory. Set one under Settings > Manga to use a folder of your own."
                />
            )}

            <UploadSection entryOptions={entryOptions} series={library?.series ?? []} />

            <Separator />

            <ScanSection unmappedCount={library?.unmappedCount ?? 0} />

            <Separator />

            <SeriesSection series={library?.series ?? []} entryOptions={entryOptions} />
        </AppLayoutStack>
    )
}

type EntryOption = { value: string, textValue: string, label: React.ReactNode }

function UploadSection({ entryOptions, series }: { entryOptions: EntryOption[], series: Manga_LocalMangaSeries[] }) {
    const { mutate: upload, isPending } = useUploadLocalMangaArchive()

    const [file, setFile] = React.useState<File | null>(null)
    const [seriesName, setSeriesName] = React.useState("")
    const [seriesTouched, setSeriesTouched] = React.useState(false)
    const [mediaId, setMediaId] = React.useState<string[]>([])
    const [overwrite, setOverwrite] = React.useState(false)
    const [uploaded, setUploaded] = React.useState<Manga_LocalMangaUploadResult | null>(null)

    const { downloadChapter, isDownloading } = useDownloadLocalManga()

    const handleFiles = React.useCallback((files: File[]) => {
        const next = files[0] ?? null
        setFile(next)
        // The series name follows the filename until the user edits it, which is
        // right most of the time and always visible before uploading.
        if (next && !seriesTouched) {
            setSeriesName(stripArchiveExtension(next.name))
        }
    }, [seriesTouched])

    const tooLarge = !!file && file.size > MAX_UPLOAD_BYTES
    const targetExists = series.some(s => s.dirName?.toLowerCase() === seriesName.trim().toLowerCase())

    function handleUpload() {
        if (!file || !seriesName.trim() || tooLarge) return

        // The server reads the parts in order and needs the metadata before the
        // archive, so the file is appended last.
        const form = new FormData()
        form.append("series", seriesName.trim())
        if (mediaId[0]) form.append("mediaId", mediaId[0])
        if (overwrite) form.append("overwrite", "true")
        form.append("file", file)

        upload(form, {
            onSuccess: result => {
                setUploaded(result ?? null)
                setFile(null)
                setSeriesTouched(false)
                setSeriesName("")
                setMediaId([])
                setOverwrite(false)
            },
        })
    }

    return (
        <div className="space-y-3" data-local-manga-upload-section>
            <div>
                <h4 className="flex items-center gap-2"><LuUpload className="text-xl" /> Upload an archive</h4>
                <p className="text-sm text-(--muted)">
                    Drop a <code>.zip</code> or <code>.cbz</code> file. An archive that holds one folder per chapter is split into
                    a chapter each.
                </p>
            </div>

            <SimpleDropzone
                accept={{ "application/zip": [".zip", ".cbz"], "application/vnd.comicbook+zip": [".cbz"] }}
                multiple={false}
                onValueChange={handleFiles}
                dropzoneText="Click or drag a .zip / .cbz archive here"
            />

            {tooLarge && <Alert intent="alert-basic" description="This archive is larger than the 2 GiB upload limit." />}

            <TextInput
                label="Series"
                help="The folder this chapter is stored in. Series already in the library are added to."
                value={seriesName}
                onValueChange={value => {
                    setSeriesTouched(true)
                    setSeriesName(value)
                }}
            />

            {targetExists && (
                <p className="text-sm text-(--muted)">Adding to the existing “{seriesName.trim()}” series.</p>
            )}

            <Combobox
                label="Map to entry (optional)"
                emptyMessage="No manga found"
                placeholder="Leave empty to map it later"
                options={entryOptions}
                value={mediaId}
                onValueChange={value => setMediaId(value.slice(-1))}
            />

            <Checkbox
                label="Replace chapters that already exist"
                value={overwrite}
                onValueChange={value => setOverwrite(value === true)}
            />

            <Button
                intent="primary"
                leftIcon={<LuUpload />}
                loading={isPending}
                disabled={!file || !seriesName.trim() || tooLarge}
                onClick={handleUpload}
            >
                Upload
            </Button>

            {!!uploaded?.chapters?.length && (
                <div className="rounded-(--radius) border p-3 space-y-2" data-local-manga-upload-result>
                    <p className="text-sm">
                        Stored {uploaded.pageCount} {uploaded.pageCount === 1 ? "page" : "pages"} in{" "}
                        <span className="font-medium">{uploaded.series}</span> as {uploaded.chapters.length}{" "}
                        {uploaded.chapters.length === 1 ? "chapter" : "chapters"}.
                    </p>
                    <div className="flex flex-wrap gap-2">
                        {uploaded.chapters.map(chapter => (
                            <Button
                                key={chapter}
                                size="sm"
                                intent="gray-subtle"
                                leftIcon={<LuDownload />}
                                loading={isDownloading === chapter}
                                onClick={() => downloadChapter(uploaded.series, chapter)}
                            >
                                {chapter}
                            </Button>
                        ))}
                    </div>
                </div>
            )}
        </div>
    )
}

function ScanSection({ unmappedCount }: { unmappedCount: number }) {
    const { mutate: scan, data: result, isPending, reset } = useScanLocalMangaLibrary()

    const [remap, setRemap] = React.useState(false)
    const [selectAsSource, setSelectAsSource] = React.useState(true)

    return (
        <div className="space-y-3" data-local-manga-scan-section>
            <div>
                <h4 className="flex items-center gap-2"><LuFolderSearch className="text-xl" /> Match series to your collection</h4>
                <p className="text-sm text-(--muted)">
                    Matches each folder against your AniList manga collection.
                    {unmappedCount > 0 && ` ${unmappedCount} ${unmappedCount === 1 ? "series is" : "series are"} not mapped yet.`}
                </p>
            </div>

            <Checkbox
                label="Re-match series that are already mapped"
                help="Off by default so mappings you made by hand are left alone."
                value={remap}
                onValueChange={value => setRemap(value === true)}
            />

            <Checkbox
                label="Read matched entries from the local provider"
                help="Only applies to entries that have no source selected yet."
                value={selectAsSource}
                onValueChange={value => setSelectAsSource(value === true)}
            />

            <div className="flex gap-2">
                <Button
                    intent="white-subtle"
                    leftIcon={<LuFolderSearch />}
                    loading={isPending}
                    onClick={() => scan({ remap, selectAsSource })}
                >
                    Scan library
                </Button>
                {!!result && <Button intent="gray-subtle" onClick={() => reset()}>Clear results</Button>}
            </div>

            {!!result && <ScanReport result={result} />}
        </div>
    )
}

function ScanReport({ result }: { result: Manga_LocalMangaScanResult }) {
    const matched = result.matched ?? []
    const skipped = result.skipped ?? []

    return (
        <div className="space-y-2" data-local-manga-scan-report>
            <p className="text-sm">
                Scanned {result.seriesCount} {result.seriesCount === 1 ? "series" : "series"} — matched {matched.length},
                skipped {skipped.length}.
            </p>

            <Disclosure type="multiple" defaultValue={matched.length ? ["matched"] : []}>
                {matched.length > 0 && (
                    <DisclosureItem value="matched">
                        <DisclosureTrigger>
                            <Button intent="gray-basic" size="sm" rightIcon={<LuChevronDown />}>Matched ({matched.length})</Button>
                        </DisclosureTrigger>
                        <DisclosureContent className="pt-2 space-y-1">
                            {matched.map(match => (
                                <div key={match.dirName} className="flex items-center gap-2 text-sm flex-wrap">
                                    <span className="truncate">{match.dirName}</span>
                                    <span className="text-(--muted)">→</span>
                                    <span className="font-medium truncate">{match.mediaTitle}</span>
                                    <Badge size="sm" intent="gray">{Math.round(match.rating * 100)}%</Badge>
                                    {match.selectedAsSource && <Badge size="sm" intent="success">source set</Badge>}
                                </div>
                            ))}
                        </DisclosureContent>
                    </DisclosureItem>
                )}

                {skipped.length > 0 && (
                    <DisclosureItem value="skipped">
                        <DisclosureTrigger>
                            <Button intent="gray-basic" size="sm" rightIcon={<LuChevronDown />}>Skipped ({skipped.length})</Button>
                        </DisclosureTrigger>
                        <DisclosureContent className="pt-2 space-y-1">
                            {skipped.map(entry => (
                                <div key={entry.dirName} className="text-sm">
                                    <div className="flex items-center gap-2 flex-wrap">
                                        <span className="truncate">{entry.dirName}</span>
                                        <Badge size="sm" intent="gray">{scanSkipReasonLabel(entry.reason)}</Badge>
                                    </div>
                                    {!!entry.candidates?.length && (
                                        <p className="text-(--muted) text-xs">
                                            Closest: {entry.candidates.slice(0, 3)
                                            .map(candidate => `${candidate.title} (${Math.round(candidate.rating * 100)}%)`)
                                            .join(", ")}
                                        </p>
                                    )}
                                </div>
                            ))}
                        </DisclosureContent>
                    </DisclosureItem>
                )}
            </Disclosure>
        </div>
    )
}

function SeriesSection({ series, entryOptions }: { series: Manga_LocalMangaSeries[], entryOptions: EntryOption[] }) {
    const { mutate: mapSeries, isPending: isMapping } = useMapLocalMangaSeries()
    const { mutate: deleteSeries, isPending: isDeleting } = useDeleteLocalMangaSeries()
    const { mutate: repackSeries, isPending: isRepacking } = useRepackLocalMangaSeries()
    const { downloadSeries, isDownloading } = useDownloadLocalManga()

    const [pendingDeletion, setPendingDeletion] = React.useState<string | null>(null)
    const [pendingRepack, setPendingRepack] = React.useState<string | null>(null)
    const [expanded, setExpanded] = React.useState<string | null>(null)

    const confirmDelete = useConfirmationDialog({
        title: "Delete series",
        description: "This permanently deletes the folder and everything in it from disk. This cannot be undone.",
        actionText: "Delete",
        actionIntent: "alert",
        onConfirm: () => {
            if (pendingDeletion) deleteSeries({ series: pendingDeletion })
            setPendingDeletion(null)
        },
    })

    const confirmRepack = useConfirmationDialog({
        title: "Rebuild as CBZ",
        description: "This rewrites the chapter files of this series in place: pages are flattened into reading order and the metadata is refreshed from AniList. Formats Seanime cannot read are left alone. The originals are replaced.",
        actionText: "Rebuild",
        actionIntent: "warning",
        onConfirm: () => {
            if (pendingRepack) repackSeries({ series: pendingRepack })
            setPendingRepack(null)
        },
    })

    if (!series.length) {
        return (
            <div data-local-manga-series-section>
                <h4>Series</h4>
                <p className="text-sm text-(--muted)">Your local manga library is empty. Upload an archive to get started.</p>
            </div>
        )
    }

    return (
        <div className="space-y-3" data-local-manga-series-section>
            <h4>Series ({series.length})</h4>

            <div className="divide-y divide-(--border) rounded-(--radius) border">
                {series.map(entry => (
                    <div key={entry.dirName} className="p-3 space-y-2">
                        <div className="flex items-center gap-2 flex-wrap">
                            <p className="font-medium truncate">{entry.dirName}</p>
                            <Badge size="sm" intent="gray">
                                {entry.chapterCount} {entry.chapterCount === 1 ? "chapter" : "chapters"}
                            </Badge>
                            <Badge size="sm" intent="gray">{formatBytes(entry.size)}</Badge>
                            <div className="flex flex-1" />

                            <Tooltip trigger={<IconButton
                                size="sm"
                                intent="gray-subtle"
                                icon={<LuDownload />}
                                aria-label={`Download ${entry.dirName}`}
                                loading={isDownloading === `${entry.dirName}.zip`}
                                disabled={entry.chapterCount === 0}
                                onClick={() => downloadSeries(entry.dirName)}
                            />}
                            >
                                Download every chapter as CBZ
                            </Tooltip>

                            <Tooltip trigger={<IconButton
                                size="sm"
                                intent="gray-subtle"
                                icon={<LuPackage />}
                                aria-label={`Rebuild ${entry.dirName}`}
                                loading={isRepacking && pendingRepack === entry.dirName}
                                disabled={entry.chapterCount === 0}
                                onClick={() => {
                                    setPendingRepack(entry.dirName)
                                    confirmRepack.open()
                                }}
                            />}
                            >
                                Rebuild the stored files as CBZ with fresh metadata
                            </Tooltip>

                            <Tooltip trigger={<IconButton
                                size="sm"
                                intent="alert-subtle"
                                icon={<BiTrash />}
                                aria-label={`Delete ${entry.dirName}`}
                                disabled={isDeleting}
                                onClick={() => {
                                    setPendingDeletion(entry.dirName)
                                    confirmDelete.open()
                                }}
                            />}
                            >
                                Delete from disk
                            </Tooltip>
                        </div>

                        <Combobox
                            emptyMessage="No manga found"
                            placeholder={entry.mediaTitle || "Not mapped — pick an entry"}
                            options={entryOptions}
                            value={entry.mediaId ? [String(entry.mediaId)] : []}
                            disabled={isMapping}
                            onValueChange={value => {
                                const selected = value.slice(-1)[0]
                                if (selected) mapSeries({ mediaId: Number(selected), series: entry.dirName })
                            }}
                        />

                        {!entry.mediaId && (
                            <p className={cn("text-xs text-(--muted)")}>
                                Not mapped — chapters in this folder are not shown against any entry.
                            </p>
                        )}

                        {entry.chapterCount > 0 && (
                            <Button
                                intent="gray-basic"
                                size="sm"
                                className="px-0!"
                                rightIcon={<LuChevronDown className={cn(expanded === entry.dirName && "rotate-180")} />}
                                onClick={() => setExpanded(current => current === entry.dirName ? null : entry.dirName)}
                            >
                                {expanded === entry.dirName ? "Hide chapters" : "Show chapters"}
                            </Button>
                        )}

                        {expanded === entry.dirName && <ChapterList series={entry.dirName} />}
                    </div>
                ))}
            </div>

            <ConfirmationDialog {...confirmDelete} />
            <ConfirmationDialog {...confirmRepack} />
        </div>
    )
}

function ChapterList({ series }: { series: string }) {
    const { data: chapters, isLoading } = useGetLocalMangaChapters(series)
    const { downloadChapter, isDownloading } = useDownloadLocalManga()

    if (isLoading) return <LoadingSpinner />
    if (!chapters?.length) return <p className="text-sm text-(--muted)">No chapter files in this folder.</p>

    return (
        <div className="space-y-1 rounded-(--radius) bg-(--subtle) p-2" data-local-manga-chapter-list>
            {chapters.map(chapter => (
                <div key={chapter.filename} className="flex items-center gap-2 text-sm">
                    <span className="truncate">{chapter.filename}</span>
                    <Badge size="sm" intent="gray">{chapter.format}</Badge>
                    {chapter.hasMetadata && <Badge size="sm" intent="success">metadata</Badge>}
                    <span className="text-xs text-(--muted) shrink-0">{formatBytes(chapter.size)}</span>
                    <div className="flex flex-1" />
                    {chapter.downloadable ? (
                        <IconButton
                            size="xs"
                            intent="gray-subtle"
                            icon={<LuDownload />}
                            aria-label={`Download ${chapter.filename}`}
                            loading={isDownloading === chapter.filename}
                            onClick={() => downloadChapter(series, chapter.filename)}
                        />
                    ) : (
                        <Tooltip trigger={<Badge size="sm" intent="gray">not readable</Badge>}>
                            Seanime cannot read {chapter.format} files, so this chapter cannot be converted or downloaded.
                        </Tooltip>
                    )}
                </div>
            ))}
        </div>
    )
}

function scanSkipReasonLabel(reason: string): string {
    switch (reason) {
        case "already_mapped":
            return "already mapped"
        case "no_chapters":
            return "no chapters found"
        case "low_confidence":
            return "no confident match"
        case "media_taken":
            return "entry already used"
        default:
            return "no match"
    }
}

function stripArchiveExtension(filename: string): string {
    return filename.replace(/\.(zip|cbz)$/i, "").trim()
}

function formatBytes(size: number): string {
    if (!size) return "0 B"
    const units = ["B", "KB", "MB", "GB", "TB"]
    const exponent = Math.min(Math.floor(Math.log(size) / Math.log(1024)), units.length - 1)
    return `${(size / Math.pow(1024, exponent)).toFixed(exponent === 0 ? 0 : 1)} ${units[exponent]}`
}
