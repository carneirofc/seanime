import { Drawer } from "@/components/ui/drawer"
import { IconButton } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Tooltip } from "@/components/ui/tooltip"
import React from "react"
import { LuTerminal } from "react-icons/lu"
import { cn } from "@/components/ui/core/styling"

type Manga_CacheKeyInfo = {
    key: string
    isExpired: boolean
    expiration?: string
    updatedAt?: string
}

type Manga_ProviderCacheInfo = {
    provider: string
    bucketType: string
    fileSizeBytes: number
    keyCount: number
    fileName: string
    filePath: string
    keys?: Manga_CacheKeyInfo[]
}

type Manga_ProviderRequestLog = {
    timestamp?: string
    method: string
    url: string
    statusCode: number
    statusText: string
    durationMs: number
    error?: string
    requestHeaders?: Record<string, string>
    responseHeaders?: Record<string, string>
}

type MangaProviderDiagnosticsDrawerProps = {
    provider: string | null | undefined
    mediaId: number | null | undefined
}

export function MangaProviderDiagnosticsDrawer({ provider, mediaId }: MangaProviderDiagnosticsDrawerProps) {
    const [open, setOpen] = React.useState(false)
    const [activeTab, setActiveTab] = React.useState<"logs" | "cache">("logs")

    const logs = undefined as Manga_ProviderRequestLog[] | undefined
    const logsLoading = false

    const cacheInfo = undefined as Manga_ProviderCacheInfo[] | undefined
    const cacheLoading = false

    // Reverse logs so newest are first
    const sortedLogs = React.useMemo((): Manga_ProviderRequestLog[] => {
        if (!logs) return []
        return [...logs].reverse()
    }, [logs])

    const hasErrors = React.useMemo(() => {
        return sortedLogs.some(log => !!log.error || log.statusCode >= 400)
    }, [sortedLogs])

    return (
        <>
            <Tooltip trigger={
                <IconButton
                    icon={<LuTerminal />}
                    intent={hasErrors ? "alert-subtle" : "gray-outline"}
                    size="sm"
                    onClick={() => setOpen(true)}
                />
            }>
                Provider diagnostics
            </Tooltip>
            <Drawer
                open={open}
                onOpenChange={v => setOpen(v)}
                title="Provider Diagnostics"
                description={provider ? `Debugging info for "${provider}"` : "No provider selected"}
                size="xl"
                side="right"
            >
                {/* Tab Buttons */}
                <div className="flex gap-1 mb-4 border-b border-[--border] pb-2">
                    <button
                        className={cn(
                            "px-3 py-1.5 text-sm rounded-md transition-colors",
                            activeTab === "logs"
                                ? "bg-[--subtle] text-[--foreground] font-medium"
                                : "text-[--muted] hover:text-[--foreground] hover:bg-[--subtle]",
                        )}
                        onClick={() => setActiveTab("logs")}
                    >
                        Request Logs
                    </button>
                    <button
                        className={cn(
                            "px-3 py-1.5 text-sm rounded-md transition-colors",
                            activeTab === "cache"
                                ? "bg-[--subtle] text-[--foreground] font-medium"
                                : "text-[--muted] hover:text-[--foreground] hover:bg-[--subtle]",
                        )}
                        onClick={() => setActiveTab("cache")}
                    >
                        Cache Info
                    </button>
                </div>

                {/* Request Logs Tab */}
                {activeTab === "logs" && (
                    <>
                        {!provider && (
                            <p className="text-[--muted] text-sm">No provider selected.</p>
                        )}

                        {provider && logsLoading && (
                            <p className="text-[--muted] text-sm">Loading request logs...</p>
                        )}

                        {provider && !logsLoading && sortedLogs.length === 0 && (
                            <p className="text-[--muted] text-sm">No requests recorded yet for this provider. Requests are logged in-memory and reset on server restart.</p>
                        )}

                        {provider && !logsLoading && sortedLogs.length > 0 && (
                            <div className="space-y-1">
                                <p className="text-[--muted] text-xs mb-3">
                                    Showing {sortedLogs.length} recent request(s). Newest first.
                                </p>
                                {sortedLogs.map((log, i) => (
                                    <RequestLogRow key={i} log={log} />
                                ))}
                            </div>
                        )}
                    </>
                )}

                {/* Cache Info Tab */}
                {activeTab === "cache" && (
                    <>
                        {!mediaId && (
                            <p className="text-[--muted] text-sm">No media selected.</p>
                        )}

                        {mediaId && cacheLoading && (
                            <p className="text-[--muted] text-sm">Loading cache info...</p>
                        )}

                        {mediaId && !cacheLoading && (!cacheInfo || cacheInfo.length === 0) && (
                            <p className="text-[--muted] text-sm">No cached data found for this media. Cache files are created when chapters or pages are fetched from a provider.</p>
                        )}

                        {mediaId && !cacheLoading && cacheInfo && cacheInfo.length > 0 && (
                            <div className="space-y-2">
                                <p className="text-[--muted] text-xs mb-3">
                                    Found {cacheInfo.length} cache bucket(s) for media ID {mediaId}.
                                </p>
                                {cacheInfo.map((info, i) => (
                                    <CacheBucketRow key={i} info={info} />
                                ))}
                            </div>
                        )}
                    </>
                )}
            </Drawer>
        </>
    )
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

function formatFileSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
    return `${(bytes / (1024 * 1024)).toFixed(2)} MB`
}

function formatDateTime(isoString: string | undefined): string {
    if (!isoString) return "N/A"
    try {
        const d = new Date(isoString)
        return d.toLocaleString([], {
            year: "numeric",
            month: "short",
            day: "numeric",
            hour: "2-digit",
            minute: "2-digit",
            second: "2-digit",
        })
    }
    catch {
        return isoString
    }
}

function getExpirationStatus(key: Manga_CacheKeyInfo): { label: string; color: string } {
    if (key.isExpired) {
        return { label: "Expired", color: "text-red-400" }
    }
    if (!key.expiration) {
        return { label: "Permanent", color: "text-blue-400" }
    }
    const exp = new Date(key.expiration)
    const now = new Date()
    const hoursUntilExpiry = (exp.getTime() - now.getTime()) / (1000 * 60 * 60)
    if (hoursUntilExpiry < 24) {
        return { label: `Expires in ${Math.round(hoursUntilExpiry)}h`, color: "text-yellow-400" }
    }
    return { label: `Expires in ${Math.round(hoursUntilExpiry / 24)}d`, color: "text-green-400" }
}

function getBucketTypeLabel(bucketType: string): string {
    switch (bucketType) {
        case "chapters":
            return "Chapter List"
        case "pages":
            return "Page Data"
        case "page-dimensions":
            return "Page Dimensions"
        default:
            return bucketType
    }
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

function CacheBucketRow({ info }: { info: Manga_ProviderCacheInfo }) {
    const [expanded, setExpanded] = React.useState(false)

    const hasExpiredKeys = info.keys?.some(k => k.isExpired) ?? false

    return (
        <div
            className={cn(
                "rounded-md border border-[--border] px-3 py-2 cursor-pointer hover:bg-[--subtle] transition-colors",
                hasExpiredKeys && "border-yellow-800/40 bg-yellow-950/10",
            )}
            onClick={() => setExpanded(!expanded)}
        >
            <div className="flex items-center gap-2 text-xs">
                <Badge
                    intent="gray"
                    size="sm"
                    className="font-mono"
                >
                    {info.provider}
                </Badge>
                <Badge
                    intent="primary"
                    size="sm"
                >
                    {getBucketTypeLabel(info.bucketType)}
                </Badge>
                <span className="text-[--muted] font-mono shrink-0">
                    {formatFileSize(info.fileSizeBytes)}
                </span>
                <span className="text-[--muted] shrink-0">
                    {info.keyCount} key(s)
                </span>
            </div>

            {expanded && (
                <div className="mt-3 space-y-2 text-xs">
                    <div className="flex gap-2">
                        <span className="text-[--muted] w-20 shrink-0">File:</span>
                        <span className="font-mono break-all select-all text-[--foreground]">{info.fileName}</span>
                    </div>
                    <div className="flex gap-2">
                        <span className="text-[--muted] w-20 shrink-0">Full Path:</span>
                        <span className="font-mono break-all select-all text-[--foreground]">{info.filePath}</span>
                    </div>
                    <div className="flex gap-2">
                        <span className="text-[--muted] w-20 shrink-0">Size:</span>
                        <span className="font-mono">{formatFileSize(info.fileSizeBytes)} ({info.fileSizeBytes.toLocaleString()} bytes)</span>
                    </div>
                    <div className="flex gap-2">
                        <span className="text-[--muted] w-20 shrink-0">Provider:</span>
                        <span className="font-mono">{info.provider}</span>
                    </div>
                    <div className="flex gap-2">
                        <span className="text-[--muted] w-20 shrink-0">Bucket Type:</span>
                        <span className="font-mono">{info.bucketType} ({getBucketTypeLabel(info.bucketType)})</span>
                    </div>

                    {/* Cache Keys */}
                    {info.keys && info.keys.length > 0 && (
                        <div className="mt-2">
                            <p className="text-[--muted] mb-1 font-medium">Cached Keys:</p>
                            <div className="space-y-1 ml-2">
                                {info.keys.map((key, j) => {
                                    const status = getExpirationStatus(key)
                                    return (
                                        <div key={j} className="rounded border border-[--border] px-2 py-1.5 bg-[--background]">
                                            <div className="flex items-center gap-2 flex-wrap">
                                                <span className="font-mono text-[--foreground] break-all select-all">{key.key}</span>
                                                <span className={cn("font-mono shrink-0", status.color)}>{status.label}</span>
                                            </div>
                                            <div className="flex gap-4 mt-1 text-[--muted]">
                                                {key.expiration && (
                                                    <span>Expires: {formatDateTime(key.expiration)}</span>
                                                )}
                                                {key.updatedAt && (
                                                    <span>Updated: {formatDateTime(key.updatedAt)}</span>
                                                )}
                                            </div>
                                        </div>
                                    )
                                })}
                            </div>
                        </div>
                    )}
                </div>
            )}
        </div>
    )
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

function RequestLogRow({ log }: { log: Manga_ProviderRequestLog }) {
    const [expanded, setExpanded] = React.useState(false)

    const isError = !!log.error || log.statusCode >= 400
    const isWarning = !isError && log.statusCode >= 300
    const isSuccess = log.statusCode >= 200 && log.statusCode < 300

    const statusColor = isError
        ? "text-red-400"
        : isWarning
            ? "text-yellow-400"
            : isSuccess
                ? "text-green-400"
                : "text-[--muted]"

    const timestamp = React.useMemo(() => {
        try {
            if (!log.timestamp) return "N/A"
            const d = new Date(log.timestamp)
            return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" })
        }
        catch {
            return log.timestamp ?? "N/A"
        }
    }, [log.timestamp])

    const hasRequestHeaders = log.requestHeaders && Object.keys(log.requestHeaders).length > 0
    const hasResponseHeaders = log.responseHeaders && Object.keys(log.responseHeaders).length > 0

    return (
        <div
            className={cn(
                "rounded-md border border-[--border] px-3 py-2 cursor-pointer hover:bg-[--subtle] transition-colors",
                isError && "border-red-800/40 bg-red-950/20",
            )}
            onClick={() => setExpanded(!expanded)}
        >
            <div className="flex items-center gap-2 text-xs">
                <span className="text-[--muted] font-mono shrink-0">{timestamp}</span>
                <Badge
                    intent={isError ? "alert" : isSuccess ? "success" : "gray"}
                    size="sm"
                    className="font-mono"
                >
                    {log.method}
                </Badge>
                {log.statusCode > 0 && (
                    <span className={cn("font-mono font-semibold", statusColor)}>
                        {log.statusCode}
                    </span>
                )}
                <span className="text-[--muted] font-mono truncate flex-1" title={log.url}>
                    {log.url}
                </span>
                <span className="text-[--muted] font-mono shrink-0">
                    {log.durationMs}ms
                </span>
            </div>

            {expanded && (
                <div className="mt-2 space-y-1 text-xs">
                    <div className="flex gap-2">
                        <span className="text-[--muted] w-16 shrink-0">URL:</span>
                        <span className="font-mono break-all select-all">{log.url}</span>
                    </div>
                    <div className="flex gap-2">
                        <span className="text-[--muted] w-16 shrink-0">Method:</span>
                        <span className="font-mono">{log.method}</span>
                    </div>
                    <div className="flex gap-2">
                        <span className="text-[--muted] w-16 shrink-0">Status:</span>
                        <span className={cn("font-mono", statusColor)}>
                            {log.statusCode > 0 ? `${log.statusCode} ${log.statusText}` : "N/A (request failed)"}
                        </span>
                    </div>
                    <div className="flex gap-2">
                        <span className="text-[--muted] w-16 shrink-0">Duration:</span>
                        <span className="font-mono">{log.durationMs}ms</span>
                    </div>
                    {log.error && (
                        <div className="flex gap-2">
                            <span className="text-[--muted] w-16 shrink-0">Error:</span>
                            <span className="font-mono text-red-400 break-all">{log.error}</span>
                        </div>
                    )}

                    {/* Request Headers */}
                    {hasRequestHeaders && (
                        <div className="mt-2">
                            <p className="text-[--muted] mb-1 font-medium">Request Headers:</p>
                            <div className="rounded border border-[--border] bg-[--background] p-2 space-y-0.5">
                                {Object.entries(log.requestHeaders!).map(([key, value]) => (
                                    <div key={key} className="flex gap-2 font-mono">
                                        <span className="text-blue-400 shrink-0">{key}:</span>
                                        <span className="text-[--foreground] break-all select-all">{value}</span>
                                    </div>
                                ))}
                            </div>
                        </div>
                    )}

                    {/* Response Headers */}
                    {hasResponseHeaders && (
                        <div className="mt-2">
                            <p className="text-[--muted] mb-1 font-medium">Response Headers:</p>
                            <div className="rounded border border-[--border] bg-[--background] p-2 space-y-0.5">
                                {Object.entries(log.responseHeaders!).map(([key, value]) => (
                                    <div key={key} className="flex gap-2 font-mono">
                                        <span className="text-green-400 shrink-0">{key}:</span>
                                        <span className="text-[--foreground] break-all select-all">{value}</span>
                                    </div>
                                ))}
                            </div>
                        </div>
                    )}

                    {!hasRequestHeaders && !hasResponseHeaders && (
                        <p className="text-[--muted] text-xs mt-2 italic">
                            No headers captured for this request. Headers are only logged for extension-based providers.
                        </p>
                    )}
                </div>
            )}
        </div>
    )
}
