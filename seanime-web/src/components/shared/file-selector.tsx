import { useFileSelector } from "@/api/hooks/file_selector.hooks"
import { IconButton } from "@/components/ui/button"
import { LoadingSpinner } from "@/components/ui/loading-spinner"
import { Modal } from "@/components/ui/modal"
import { ScrollArea } from "@/components/ui/scroll-area"
import { TextInput } from "@/components/ui/text-input"
import { upath } from "@/lib/helpers/upath"
import React from "react"
import { FaFolder } from "react-icons/fa"
import { FiChevronLeft, FiFile, FiFolder } from "react-icons/fi"
import { useDebounce } from "use-debounce"

export type FileSelectorModalProps = {
    open: boolean
    onOpenChange: (open: boolean) => void
    // File extensions to show (e.g. [".json"]). Empty shows every file.
    extensions?: string[]
    // Called with the absolute path of the file the user picks.
    onSelect: (path: string) => void
    // Directory (or file path) to start browsing from. Empty starts at the home directory.
    initialPath?: string
    title?: string
}

// FileSelectorModal is a local filesystem browser: navigate directories and pick
// a file. Directories are always shown; files are filtered by `extensions`.
export function FileSelectorModal(props: FileSelectorModalProps) {
    const {
        open,
        onOpenChange,
        extensions,
        onSelect,
        initialPath,
        title = "Select a file",
    } = props

    const sanitizePath = React.useCallback((path: string) => {
        if (!path) return ""
        return upath.normalizeSafe(path.replace(/[<>"]/g, ""))
    }, [])

    const [input, setInput] = React.useState(initialPath ? sanitizePath(initialPath) : "")
    const [debouncedInput] = useDebounce(input, 300)

    // Reset to the initial path each time the modal opens.
    React.useEffect(() => {
        if (open) {
            setInput(initialPath ? sanitizePath(initialPath) : "")
        }
    }, [open])

    const exts = React.useMemo(() => extensions ?? [], [JSON.stringify(extensions ?? [])])

    const { data, isLoading } = useFileSelector(debouncedInput, exts, open)

    return (
        <Modal
            open={open}
            onOpenChange={onOpenChange}
            title={title}
            contentClass="mt-4 space-y-2 max-w-4xl"
        >
            <div className="flex gap-2 items-center">
                <IconButton
                    onClick={() => data?.basePath && setInput(data.basePath)}
                    intent="gray-basic"
                    rounded
                    size="sm"
                    icon={<FiChevronLeft />}
                    disabled={(!data?.basePath?.length || data?.basePath?.length === 1)}
                />
                <TextInput
                    leftIcon={<FaFolder />}
                    value={input}
                    placeholder="Type a path or browse below…"
                    onValueChange={setInput}
                />
            </div>

            {isLoading && <LoadingSpinner />}

            {(data && !!data?.content?.length) &&
                <ScrollArea className="h-72 rounded-md border mt-0!">
                    {data.content.map(entry => (
                        <div
                            key={entry.fullPath}
                            className="flex items-center gap-2 py-2 px-3 cursor-pointer hover:bg-gray-800"
                            onClick={() => {
                                if (entry.isDir) {
                                    setInput(entry.fullPath)
                                } else {
                                    onSelect(entry.fullPath)
                                    onOpenChange(false)
                                }
                            }}
                        >
                            {entry.isDir
                                ? <FiFolder className="w-4 h-4 text-(--brand) shrink-0" />
                                : <FiFile className="w-4 h-4 text-(--muted) shrink-0" />}
                            <span className="break-all">{entry.name}</span>
                        </div>
                    ))}
                </ScrollArea>}

            {(data && !data?.content?.length && !isLoading) &&
                <p className="text-center text-(--muted) py-8">
                    No matching files in this folder.
                </p>}
        </Modal>
    )
}
