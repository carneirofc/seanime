import Page from "@/app/(main)/watchlists/page"
import { createLazyFileRoute } from "@tanstack/react-router"

export const Route = createLazyFileRoute("/_main/watchlists/")({
    component: Page,
})
