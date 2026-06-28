import { __DEV_SERVER_PORT } from "@/lib/server/config"
import { __isDesktop__ } from "@/types/constants"

export function getServerBaseUrl(removeProtocol: boolean = false): string {
    const isDev = import.meta.env.MODE === "development"

    if (__isDesktop__) {
        let ret = isDev ? `http://127.0.0.1:${__DEV_SERVER_PORT}` : "http://127.0.0.1:43211"
        if (removeProtocol) {
            ret = ret.replace("http://", "").replace("https://", "")
        }
        return ret
    }

    let ret: string
    if (isDev) {
        const hostname = typeof window !== "undefined" ? window.location.hostname : "127.0.0.1"
        ret = `http://${hostname}:${__DEV_SERVER_PORT}`
    } else {
        ret = typeof window !== "undefined" ? `${window?.location?.protocol}//${window?.location?.host}` : ""
    }

    if (removeProtocol) {
        ret = ret.replace("http://", "").replace("https://", "")
    }
    return ret
}
