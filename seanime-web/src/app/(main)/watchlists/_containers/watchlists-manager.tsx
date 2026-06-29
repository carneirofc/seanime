import { useWatchlists, Watchlist, WatchlistGrouping, WatchlistMedia } from "@/app/(main)/_features/watchlists/watchlists.atoms"
import { InlineCreate } from "@/app/(main)/_features/watchlists/watchlist-inline-create"
import { ConfirmationDialog, useConfirmationDialog } from "@/components/shared/confirmation-dialog"
import { SeaLink } from "@/components/shared/sea-link"
import { IconButton } from "@/components/ui/button"
import { cn } from "@/components/ui/core/styling"
import { TextInput } from "@/components/ui/text-input"
import React from "react"
import { BiTrash } from "react-icons/bi"
import { LuFolderPlus, LuListPlus, LuPencil, LuX } from "react-icons/lu"

function entryLink(media: WatchlistMedia) {
    return media.type === "anime" ? `/entry?id=${media.mediaId}` : `/manga/entry?id=${media.mediaId}`
}

/**
 * Editable title: click the pencil to switch to an input, Enter/blur to commit.
 */
function EditableTitle(props: { value: string, onChange: (v: string) => void, className?: string }) {
    const [editing, setEditing] = React.useState(false)
    const [draft, setDraft] = React.useState(props.value)

    const commit = () => {
        setEditing(false)
        const v = draft.trim()
        if (v && v !== props.value) props.onChange(v)
        else setDraft(props.value)
    }

    if (editing) {
        return (
            <TextInput
                value={draft}
                onValueChange={setDraft}
                size="sm"
                autoFocus
                onBlur={commit}
                onKeyDown={(e: React.KeyboardEvent) => {
                    if (e.key === "Enter") {
                        e.preventDefault()
                        commit()
                    }
                    if (e.key === "Escape") {
                        setDraft(props.value)
                        setEditing(false)
                    }
                }}
                className="max-w-[280px]"
            />
        )
    }

    return (
        <button
            type="button"
            onClick={() => { setDraft(props.value); setEditing(true) }}
            className={cn("group inline-flex items-center gap-1.5 text-left hover:text-(--brand) transition-colors", props.className)}
        >
            <span className="truncate">{props.value}</span>
            <LuPencil className="size-3.5 opacity-0 group-hover:opacity-60 shrink-0" />
        </button>
    )
}

function WatchlistMediaCard(props: { media: WatchlistMedia, onRemove: () => void }) {
    const { media, onRemove } = props
    return (
        <div data-watchlist-media-card className="group relative">
            <SeaLink href={entryLink(media)} className="block">
                <div className="aspect-6/8 w-full rounded-lg overflow-hidden bg-gray-900 border border-(--border)">
                    {media.image
                        ? <img src={media.image} alt={media.title} className="w-full h-full object-cover" loading="lazy" />
                        : <div className="w-full h-full flex items-center justify-center text-(--muted) text-xs p-2 text-center">{media.title}</div>}
                </div>
                <p className="text-sm mt-1.5 line-clamp-2 leading-tight">{media.title}</p>
                <p className="text-xs text-(--muted) capitalize">{media.type}{media.year ? ` • ${media.year}` : ""}</p>
            </SeaLink>
            <IconButton
                icon={<LuX />}
                intent="alert"
                size="sm"
                className="absolute top-1.5 right-1.5 opacity-0 group-hover:opacity-100 transition-opacity rounded-full h-7! w-7!"
                onClick={onRemove}
            />
        </div>
    )
}

function WatchlistSection(props: {
    watchlist: Watchlist
    onRename: (name: string) => void
    onDelete: () => void
    onRemoveMedia: (mediaId: number, type: WatchlistMedia["type"]) => void
}) {
    const { watchlist, onRename, onDelete, onRemoveMedia } = props
    return (
        <div data-watchlist-section className="space-y-3">
            <div className="flex items-center justify-between gap-2">
                <div className="flex items-center gap-2 font-medium min-w-0">
                    <EditableTitle value={watchlist.name} onChange={onRename} />
                    <span className="text-xs text-(--muted) shrink-0">{watchlist.media.length}</span>
                </div>
                <IconButton icon={<BiTrash />} intent="alert-subtle" size="sm" onClick={onDelete} />
            </div>

            {watchlist.media.length === 0 ? (
                <p className="text-sm text-(--muted) italic">
                    No media yet. Add anime or manga from their card's right-click menu.
                </p>
            ) : (
                <div className="grid grid-cols-3 sm:grid-cols-4 md:grid-cols-5 lg:grid-cols-6 xl:grid-cols-8 gap-3">
                    {watchlist.media.map(m => (
                        <WatchlistMediaCard
                            key={`${m.type}-${m.mediaId}`}
                            media={m}
                            onRemove={() => onRemoveMedia(m.mediaId, m.type)}
                        />
                    ))}
                </div>
            )}
        </div>
    )
}

function GroupingSection(props: {
    grouping: WatchlistGrouping
    confirmDelete: (title: string, description: string, onConfirm: () => void) => void
}) {
    const { grouping, confirmDelete } = props
    const { renameGrouping, deleteGrouping, createWatchlist, renameWatchlist, deleteWatchlist, removeMediaFromWatchlist } = useWatchlists()

    return (
        <div data-watchlist-grouping className="rounded-xl border border-(--border) bg-gray-50 dark:bg-white/2 p-4 sm:p-5 space-y-5">
            <div className="flex items-center justify-between gap-2 border-b border-(--border) pb-3">
                <EditableTitle
                    value={grouping.name}
                    onChange={name => renameGrouping(grouping.id, name)}
                    className="text-lg font-semibold"
                />
                <IconButton
                    icon={<BiTrash />}
                    intent="alert-subtle"
                    size="sm"
                    onClick={() => confirmDelete(
                        "Delete grouping",
                        `Delete "${grouping.name}" and all its watchlists? This cannot be undone.`,
                        () => deleteGrouping(grouping.id),
                    )}
                />
            </div>

            {grouping.watchlists.length === 0 && (
                <p className="text-sm text-(--muted) italic">No watchlists in this grouping yet.</p>
            )}

            <div className="space-y-6">
                {grouping.watchlists.map(watchlist => (
                    <WatchlistSection
                        key={watchlist.id}
                        watchlist={watchlist}
                        onRename={name => renameWatchlist(watchlist.id, name)}
                        onDelete={() => confirmDelete(
                            "Delete watchlist",
                            `Delete "${watchlist.name}"? This cannot be undone.`,
                            () => deleteWatchlist(watchlist.id),
                        )}
                        onRemoveMedia={(mediaId, type) => removeMediaFromWatchlist(watchlist.id, mediaId, type)}
                    />
                ))}
            </div>

            <div className="pt-1">
                <InlineCreate
                    placeholder="New watchlist"
                    icon={<LuListPlus />}
                    onCreate={name => createWatchlist(grouping.id, name)}
                />
            </div>
        </div>
    )
}

export function WatchlistsManager() {
    const { groupings, createGrouping } = useWatchlists()

    const pendingAction = React.useRef<(() => void) | null>(null)
    const [confirmText, setConfirmText] = React.useState({ title: "", description: "" })

    const confirmDialog = useConfirmationDialog({
        title: confirmText.title,
        description: confirmText.description,
        actionText: "Delete",
        onConfirm: () => {
            pendingAction.current?.()
            pendingAction.current = null
        },
    })

    const confirmDelete = React.useCallback((title: string, description: string, onConfirm: () => void) => {
        pendingAction.current = onConfirm
        setConfirmText({ title, description })
        confirmDialog.open()
    }, [confirmDialog])

    return (
        <div data-watchlists-manager className="space-y-6">
            <div className="flex flex-col sm:flex-row sm:items-end justify-between gap-3">
                <div>
                    <h2 className="text-2xl font-bold">Watchlists</h2>
                    <p className="text-(--muted) text-sm">
                        Organize anime and manga into watchlists, grouped into folders. Stored in this browser.
                    </p>
                </div>
                <InlineCreate
                    placeholder="New grouping"
                    icon={<LuFolderPlus />}
                    onCreate={name => createGrouping(name)}
                />
            </div>

            {groupings.length === 0 ? (
                <div className="rounded-xl border border-dashed border-(--border) py-16 text-center text-(--muted)">
                    <p className="font-medium">No groupings yet</p>
                    <p className="text-sm mt-1">Create a grouping above to start building watchlists.</p>
                </div>
            ) : (
                <div className="space-y-5">
                    {groupings.map(grouping => (
                        <GroupingSection key={grouping.id} grouping={grouping} confirmDelete={confirmDelete} />
                    ))}
                </div>
            )}

            <ConfirmationDialog {...confirmDialog} />
        </div>
    )
}
