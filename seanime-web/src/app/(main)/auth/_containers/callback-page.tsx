import { useLogin } from "@/api/hooks/auth.hooks"
import { websocketConnectedAtom } from "@/app/websocket-provider"
import { LoadingOverlay } from "@/components/ui/loading-spinner"
import { useRouter } from "@/lib/navigation"
import { useAtomValue } from "jotai/react"
import React from "react"
import { toast } from "sonner"

type CallbackPageProps = {}

/**
 * @description
 * - Logs the user in using the AniList token present in the URL hash
 */
export function CallbackPage(props: CallbackPageProps) {
    const router = useRouter()
    const {} = props

    const websocketConnected = useAtomValue(websocketConnectedAtom)

    const { mutate: login } = useLogin()

    const called = React.useRef(false)

    React.useEffect(() => {
        if (typeof window !== "undefined" && websocketConnected) {
            /**
             * Get the AniList token from the URL hash
             */
            const _token = window?.location?.hash?.replace("#access_token=", "")?.replace(/&.*/, "")

            /**
             * Drop the token from the URL before doing anything with it.
             *
             * AniList hands it over in the fragment, and the manual-entry form puts it
             * there too. Left alone it stays in the address bar and in browser history
             * for good - surviving logout - and the issue recorder captures page URLs,
             * so it would ride along into a public bug report.
             */
            window.history.replaceState(null, "", window.location.pathname + window.location.search)

            if (!!_token && !called.current) {
                login({ token: _token })
                called.current = true
            } else {
                toast.error("Invalid token")
                router.push("/")
            }
        }
    }, [websocketConnected])

    return (
        <div>
            <LoadingOverlay className="fixed w-full h-full z-80">
                <h3 className="mt-2">Authenticating...</h3>
            </LoadingOverlay>
        </div>
    )
}
