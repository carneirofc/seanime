import { useServerStatus } from "@/app/(main)/_hooks/use-server-status"
import { PageWrapper } from "@/components/shared/page-wrapper"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import React from "react"
import { LuExternalLink } from "react-icons/lu"

export default function Page() {

    const status = useServerStatus()
    const settings = status?.settings

    const qbittorrentUrl = settings?.torrent
        ? `http://${settings.torrent.qbittorrentHost}:${String(settings.torrent.qbittorrentPort)}`
        : ""

    if (!settings) return null

    return (
        <PageWrapper className="p-4 sm:p-6 lg:p-8 space-y-6">
            <header className="flex items-center justify-between">
                <div>
                    <h2>qBittorrent</h2>
                    <p className="text-(--muted)">Access the embedded qBittorrent client Web UI.</p>
                </div>
            </header>

            <div className="flex items-center justify-center h-[calc(100vh-16rem)]">
                <Card className="max-w-md p-8 text-center space-y-6 border border-(--border) bg-gray-900/40 backdrop-blur-sm rounded-2xl shadow-xl">
                    <div className="space-y-2">
                        <h3 className="text-xl font-semibold tracking-tight text-white">Open in a new tab</h3>
                        <p className="text-sm text-(--muted)">
                            Due to browser security policies (COEP and Clickjacking protection), the embedded client cannot be loaded inside the
                            iframe here.
                        </p>
                    </div>
                    <a
                        href={qbittorrentUrl}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="inline-block w-full"
                    >
                        <Button
                            className="w-full"
                            intent="primary"
                            leftIcon={<LuExternalLink />}
                        >
                            Open qBittorrent Web UI
                        </Button>
                    </a>
                </Card>
            </div>
        </PageWrapper>
    )
}
