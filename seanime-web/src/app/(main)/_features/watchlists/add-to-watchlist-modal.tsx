import { useAddToWatchlistManager, useWatchlists } from "@/app/(main)/_features/watchlists/watchlists.atoms"
import { InlineCreate } from "@/app/(main)/_features/watchlists/watchlist-inline-create"
import { Button } from "@/components/ui/button"
import { cn } from "@/components/ui/core/styling"
import { Modal } from "@/components/ui/modal"
import { ScrollArea } from "@/components/ui/scroll-area"
import React from "react"
import { LuCheck, LuFolderPlus, LuListPlus } from "react-icons/lu"

export function AddToWatchlistModal() {
    const { media, isOpen, setOpen } = useAddToWatchlistManager()
    const { groupings, createGrouping, createWatchlist, addMediaToWatchlist, removeMediaFromWatchlist } = useWatchlists()

    if (!media) {
        // Keep the modal mounted but render nothing meaningful when no media is selected.
        return <Modal open={isOpen} onOpenChange={setOpen} title="Add to watchlist" />
    }

    return (
        <Modal
            open={isOpen}
            onOpenChange={setOpen}
            title="Add to watchlist"
            description={media.title}
            contentClass="max-w-xl"
        >
            <div className="space-y-4">
                {groupings.length === 0 ? (
                    <div className="text-center space-y-3 py-2">
                        <p className="text-(--muted) text-sm">
                            You don't have any groupings yet. Create one to start organizing your watchlists.
                        </p>
                        <InlineCreate
                            placeholder="New grouping name"
                            icon={<LuFolderPlus />}
                            onCreate={name => createGrouping(name)}
                        />
                    </div>
                ) : (
                    <>
                        <ScrollArea className="max-h-[55vh] pr-3">
                            <div className="space-y-4">
                                {groupings.map(grouping => (
                                    <div key={grouping.id} data-add-to-watchlist-grouping className="space-y-1.5">
                                        <p className="text-xs font-semibold uppercase tracking-wide text-(--muted)">
                                            {grouping.name}
                                        </p>

                                        {grouping.watchlists.length === 0 && (
                                            <p className="text-xs text-(--muted) italic px-1">No watchlists yet</p>
                                        )}

                                        <div className="space-y-1">
                                            {grouping.watchlists.map(watchlist => {
                                                const inIt = watchlist.media.some(m => m.mediaId === media.mediaId && m.type === media.type)
                                                return (
                                                    <button
                                                        key={watchlist.id}
                                                        type="button"
                                                        onClick={() => {
                                                            if (inIt) {
                                                                removeMediaFromWatchlist(watchlist.id, media.mediaId, media.type)
                                                            } else {
                                                                addMediaToWatchlist(watchlist.id, media)
                                                            }
                                                        }}
                                                        className={cn(
                                                            "w-full flex items-center justify-between gap-2 rounded-lg px-3 py-2 text-sm transition-colors text-left",
                                                            "border",
                                                            inIt
                                                                ? "border-brand-400/40 bg-brand-500/10 text-(--brand)"
                                                                : "border-transparent bg-gray-100 dark:bg-white/5 hover:bg-gray-200 dark:hover:bg-white/10",
                                                        )}
                                                    >
                                                        <span className="truncate">
                                                            {watchlist.name}
                                                            <span className="text-(--muted) ml-2 text-xs">{watchlist.media.length}</span>
                                                        </span>
                                                        {inIt && <LuCheck className="shrink-0" />}
                                                    </button>
                                                )
                                            })}
                                        </div>

                                        <div className="pt-1">
                                            <InlineCreate
                                                placeholder={`New watchlist in "${grouping.name}"`}
                                                icon={<LuListPlus />}
                                                onCreate={name => createWatchlist(grouping.id, name)}
                                            />
                                        </div>
                                    </div>
                                ))}
                            </div>
                        </ScrollArea>

                        <div className="border-t border-(--border) pt-3">
                            <p className="text-xs font-semibold uppercase tracking-wide text-(--muted) mb-1.5">New grouping</p>
                            <InlineCreate
                                placeholder="New grouping name"
                                icon={<LuFolderPlus />}
                                onCreate={name => createGrouping(name)}
                            />
                        </div>
                    </>
                )}

                <div className="flex justify-end">
                    <Button intent="white-subtle" size="sm" onClick={() => setOpen(false)}>Done</Button>
                </div>
            </div>
        </Modal>
    )
}
