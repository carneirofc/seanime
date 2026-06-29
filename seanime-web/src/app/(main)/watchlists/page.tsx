import { WatchlistsManager } from "@/app/(main)/watchlists/_containers/watchlists-manager"
import { PageWrapper } from "@/components/shared/page-wrapper"
import React from "react"

export default function Page() {
    return (
        <PageWrapper
            className="p-4 sm:p-8 pt-4 relative"
            data-watchlists-page
        >
            <WatchlistsManager />
        </PageWrapper>
    )
}
