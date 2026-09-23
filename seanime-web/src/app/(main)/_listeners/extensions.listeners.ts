import { API_ENDPOINTS } from "@/api/generated/endpoints"
import { ExtensionRepo_ExtensionFailingEvent, ExtensionRepo_UpdateData } from "@/api/generated/types"
import { useWebsocketMessageListener } from "@/app/(main)/_hooks/handle-websockets"
import { WSEvents } from "@/lib/server/ws-events"
import { useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"

/**
 * @description
 * - Re-fetches queries associated with extension data
 */
export function useExtensionListener() {

    const qc = useQueryClient()

    useWebsocketMessageListener<number>({
        type: WSEvents.EXTENSIONS_RELOADED,
        onMessage: () => {
            (async () => {
                await qc.invalidateQueries({ queryKey: [API_ENDPOINTS.EXTENSIONS.ListAnimeTorrentProviderExtensions.key] })
                await qc.invalidateQueries({ queryKey: [API_ENDPOINTS.EXTENSIONS.ListMangaProviderExtensions.key] })
                await qc.invalidateQueries({ queryKey: [API_ENDPOINTS.EXTENSIONS.ListOnlinestreamProviderExtensions.key] })
                await qc.invalidateQueries({ queryKey: [API_ENDPOINTS.EXTENSIONS.ListExtensionData.key] })
                await qc.invalidateQueries({ queryKey: [API_ENDPOINTS.EXTENSIONS.GetAllExtensions.key] })
                await qc.invalidateQueries({ queryKey: [API_ENDPOINTS.EXTENSIONS.GetExtensionUserConfig.key] })
                await qc.invalidateQueries({ queryKey: [API_ENDPOINTS.EXTENSIONS.GetExtensionUpdateData.key] })
                await qc.invalidateQueries({ queryKey: [API_ENDPOINTS.EXTENSIONS.ListDevelopmentModeExtensions.key] })
            })()
        },
    })

    useWebsocketMessageListener<number>({
        type: WSEvents.PLUGIN_UNLOADED,
        onMessage: () => {
            (async () => {
                await qc.invalidateQueries({ queryKey: [API_ENDPOINTS.EXTENSIONS.ListDevelopmentModeExtensions.key] })
            })()
        },
    })

    useWebsocketMessageListener<ExtensionRepo_ExtensionFailingEvent>({
        type: WSEvents.EXTENSION_FAILING,
        onMessage: async (data) => {
            const name = data.name || data.id
            if (data.autoDisabled) {
                toast.warning(`${name} was disabled because it keeps failing.`, { description: data.lastError })
            } else {
                toast.error(`${name} keeps failing.`, { description: data.lastError })
            }
            await qc.invalidateQueries({ queryKey: [API_ENDPOINTS.EXTENSIONS.GetAllExtensions.key] })
        },
    })

    useWebsocketMessageListener<ExtensionRepo_UpdateData[]>({
        type: WSEvents.EXTENSION_UPDATES_FOUND,
        onMessage: async (data) => {
            await qc.invalidateQueries({ queryKey: [API_ENDPOINTS.EXTENSIONS.GetExtensionUpdateData.key] })
            await qc.invalidateQueries({ queryKey: [API_ENDPOINTS.EXTENSIONS.GetAllExtensions.key] })
        },
    })

}
