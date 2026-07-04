import { API_ENDPOINTS } from "@/api/generated/endpoints.ts"
import { isLoginModalOpenAtom } from "@/app/(main)/_atoms/server-status.atoms.ts"
import { useWebsocketMessageListener } from "@/app/(main)/_hooks/handle-websockets.ts"
import { logger } from "@/lib/helpers/debug.ts"
import { WSEvents } from "@/lib/server/ws-events.ts"
import { useQueryClient } from "@tanstack/react-query"
import { useAtom } from "jotai/react"
import { toast } from "sonner"

export function useAuthEventListeners() {
    const queryClient = useQueryClient()

    const [, setLoginModalOpen] = useAtom(isLoginModalOpenAtom)

    useWebsocketMessageListener<string>({
        type: WSEvents.SERVER_LOGGED_OUT_ANILIST, async onMessage(msg: string) {
            // The server forced an AniList logout (token reported invalid). Log it to the console so the cause
            // is visible after the fact — the toast is ephemeral. Check the server logs for the matching
            // "anilist cache: ... user not found" line to see the raw AniList error behind the logout.
            logger("AniList").warning("Server forced AniList logout:", msg)
            // refetch the status, user should be logged out
            await queryClient.invalidateQueries({ queryKey: [API_ENDPOINTS.STATUS.GetStatus.key] })
            toast.warning(msg)
            setTimeout(() => {
                // open the login modal
                setLoginModalOpen(true)
            }, 1000)
        },
    })

}
