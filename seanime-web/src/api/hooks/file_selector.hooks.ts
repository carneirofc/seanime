import { useServerQuery } from "@/api/client/requests"
import { FileSelector_Variables } from "@/api/generated/endpoint.types"
import { API_ENDPOINTS } from "@/api/generated/endpoints"
import { FileSelectorResponse } from "@/api/generated/types"

export function useFileSelector(debouncedInput: string, extensions: string[], enabled: boolean = true) {
    return useServerQuery<FileSelectorResponse, FileSelector_Variables>({
        endpoint: API_ENDPOINTS.FILE_SELECTOR.FileSelector.endpoint,
        method: API_ENDPOINTS.FILE_SELECTOR.FileSelector.methods[0],
        queryKey: [API_ENDPOINTS.FILE_SELECTOR.FileSelector.key, debouncedInput, extensions],
        data: { input: debouncedInput, extensions: extensions },
        // An empty input is allowed: the server starts from the home directory.
        enabled: enabled,
        muteError: true,
    })
}
