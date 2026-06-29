import { Badge } from "@/components/ui/badge"
import { Button, ButtonAnatomy } from "@/components/ui/button"
import { cn } from "@/components/ui/core/styling"
import { Modal } from "@/components/ui/modal"
import { ScrollArea } from "@/components/ui/scroll-area"
import { TextInput } from "@/components/ui/text-input"
import { Tooltip } from "@/components/ui/tooltip"
import React from "react"
import { BiX } from "react-icons/bi"
import { FiSearch } from "react-icons/fi"
import { TbTagsFilled } from "react-icons/tb"

export type TagsPanelTag = {
    name: string
    description?: string
    category?: string
    isAdult?: boolean
}

type TagsPanelModalProps = {
    /**
     * The list of tags to display, already filtered (e.g. for adult content).
     */
    tags: TagsPanelTag[]
    /**
     * Currently selected tag names.
     */
    value: string[]
    onValueChange: (value: string[]) => void
    className?: string
}

const UNCATEGORIZED = "Other"

/**
 * Turns a raw AniList category (e.g. "Theme-Game-Card & Board Game") into a
 * readable header ("Theme › Game › Card & Board Game").
 */
function prettifyCategory(category: string): string {
    return category.split("-").map(s => s.trim()).join(" › ")
}

export function TagsPanelModal(props: TagsPanelModalProps) {
    const { tags, value, onValueChange, className } = props

    const [open, setOpen] = React.useState(false)
    const [search, setSearch] = React.useState("")

    const selected = React.useMemo(() => new Set(value), [value])

    const toggleTag = React.useCallback((name: string) => {
        if (selected.has(name)) {
            onValueChange(value.filter(v => v !== name))
        } else {
            onValueChange([...value, name])
        }
    }, [value, selected, onValueChange])

    // Group tags by their category, applying the search filter.
    const groups = React.useMemo(() => {
        const q = search.trim().toLowerCase()
        const map = new Map<string, TagsPanelTag[]>()

        for (const tag of tags) {
            if (q.length > 0) {
                const matches = tag.name.toLowerCase().includes(q)
                    || (tag.description?.toLowerCase().includes(q) ?? false)
                if (!matches) continue
            }
            const category = tag.category?.trim() || UNCATEGORIZED
            const arr = map.get(category)
            if (arr) {
                arr.push(tag)
            } else {
                map.set(category, [tag])
            }
        }

        return Array.from(map.entries())
            .map(([category, items]) => ({
                category,
                label: category === UNCATEGORIZED ? UNCATEGORIZED : prettifyCategory(category),
                items: items.sort((a, b) => a.name.localeCompare(b.name)),
            }))
            .sort((a, b) => a.label.localeCompare(b.label))
    }, [tags, search])

    return (
        <Modal
            open={open}
            onOpenChange={setOpen}
            title="Tags"
            description="Browse tags by group and hover for a description."
            contentClass="max-w-3xl"
            trigger={
                <Button
                    intent="gray-outline"
                    className={cn("w-full justify-start font-normal", className)}
                    leftIcon={<TbTagsFilled className={cn(value.length > 0 && "text-indigo-300")} />}
                    data-advanced-search-options-tags
                >
                    {value.length > 0
                        ? `${value.length} tag${value.length > 1 ? "s" : ""} selected`
                        : "All tags"}
                </Button>
            }
        >
            <div data-tags-panel-modal-content className="space-y-3">
                <TextInput
                    leftIcon={<FiSearch />}
                    placeholder="Search tags..."
                    value={search}
                    onValueChange={setSearch}
                />

                {value.length > 0 && (
                    <div data-tags-panel-selected className="flex flex-wrap gap-1.5">
                        {value.map(name => (
                            <Badge
                                key={name}
                                size="lg"
                                intent="primary"
                                className="cursor-pointer hover:opacity-80"
                                rightIcon={<BiX />}
                                onClick={() => toggleTag(name)}
                            >
                                {name}
                            </Badge>
                        ))}
                        <Button
                            intent="alert-subtle"
                            size="sm"
                            onClick={() => onValueChange([])}
                        >
                            Clear all
                        </Button>
                    </div>
                )}

                <ScrollArea className="h-[60vh] pr-3">
                    {groups.length === 0 ? (
                        <p className="text-[--muted] text-center py-8">No tags found</p>
                    ) : (
                        <div className="space-y-5">
                            {groups.map(group => (
                                <div key={group.category} data-tags-panel-group>
                                    <h5 className="text-[--muted] font-medium mb-2 sticky top-0 bg-[--background] py-1 z-[1]">
                                        {group.label}
                                    </h5>
                                    <div className="flex flex-wrap gap-1.5">
                                        {group.items.map(tag => {
                                            const isSelected = selected.has(tag.name)
                                            return (
                                                <Tooltip
                                                    key={tag.name}
                                                    className="max-w-[300px] text-xs"
                                                    trigger={
                                                        <button
                                                            type="button"
                                                            onClick={() => toggleTag(tag.name)}
                                                            className={cn(
                                                                ButtonAnatomy.root({
                                                                    size: "sm",
                                                                    intent: isSelected ? "primary" : "gray-subtle",
                                                                }),
                                                                tag.isAdult && "border border-pink-400 dark:border-pink-500",
                                                            )}
                                                        >
                                                            {tag.name}
                                                        </button>
                                                    }
                                                >
                                                    {tag.description ?? "No description available."}
                                                </Tooltip>
                                            )
                                        })}
                                    </div>
                                </div>
                            ))}
                        </div>
                    )}
                </ScrollArea>
            </div>
        </Modal>
    )
}
