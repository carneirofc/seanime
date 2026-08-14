import { useServerMutation, useServerQuery } from "@/api/client/requests"
import {
    FetchExternalExtensionData_Variables,
    GetAllExtensions_Variables,
    GrantPluginPermissions_Variables,
    InstallExternalExtension_Variables,
    InstallExternalExtensionRepository_Variables,
    ReloadExternalExtension_Variables,
    ReloadExternalExtensionFromSource_Variables,
    RemoveExtensionGitToken_Variables,
    RunExtensionPlaygroundCode_Variables,
    SaveExtensionUserConfig_Variables,
    SetExtensionGitToken_Variables,
    SetExternalExtensionDisabled_Variables,
    SetPluginSettingsPinnedTrays_Variables,
    UninstallExternalExtension_Variables,
    UpdateExtensionCode_Variables,
} from "@/api/generated/endpoint.types"
import { API_ENDPOINTS } from "@/api/generated/endpoints"
import {
    Extension_Extension,
    ExtensionRepo_AllExtensions,
    ExtensionRepo_AnimeTorrentProviderExtensionItem,
    ExtensionRepo_CustomSourceExtensionItem,
    ExtensionRepo_ExtensionInstallResponse,
    ExtensionRepo_ExtensionUserConfig,
    ExtensionRepo_GitTokenInfo,
    ExtensionRepo_MangaProviderExtensionItem,
    ExtensionRepo_OnlinestreamProviderExtensionItem,
    ExtensionRepo_PluginEpisodeTabExtensionItem,
    ExtensionRepo_ReloadFromSourceResult,
    ExtensionRepo_RepositoryInstallResponse,
    ExtensionRepo_StoredPluginSettingsData,
    ExtensionRepo_UpdateData,
    Nullish,
    RunPlaygroundCodeResponse,
} from "@/api/generated/types"
import { useQueryClient } from "@tanstack/react-query"
import { atom } from "jotai"
import { useAtom } from "jotai/react"
import React from "react"
import { toast } from "sonner"

const pluginWithIssuesCountAtom = atom(0)

export function usePluginWithIssuesCount() {
    const [count] = useAtom(pluginWithIssuesCountAtom)
    return count
}

export function useGetAllExtensions(withUpdates: boolean) {
    const { data, ...rest } = useServerQuery<ExtensionRepo_AllExtensions, GetAllExtensions_Variables>({
        endpoint: API_ENDPOINTS.EXTENSIONS.GetAllExtensions.endpoint,
        method: API_ENDPOINTS.EXTENSIONS.GetAllExtensions.methods[0],
        queryKey: [API_ENDPOINTS.EXTENSIONS.GetAllExtensions.key, withUpdates],
        data: {
            withUpdates: withUpdates,
        },
        gcTime: 0,
        enabled: true,
    })

    const [, setCount] = useAtom(pluginWithIssuesCountAtom)
    React.useEffect(() => {
        setCount((data?.invalidExtensions ?? []).length + (data?.invalidUserConfigExtensions ?? []).length)
    }, [data])

    return { data, ...rest }
}

export function useFetchExternalExtensionData(id: Nullish<string>) {
    return useServerMutation<Extension_Extension, FetchExternalExtensionData_Variables>({
        endpoint: API_ENDPOINTS.EXTENSIONS.FetchExternalExtensionData.endpoint,
        method: API_ENDPOINTS.EXTENSIONS.FetchExternalExtensionData.methods[0],
        mutationKey: [API_ENDPOINTS.EXTENSIONS.FetchExternalExtensionData.key, id],
        onSuccess: async () => {

        },
    })
}

export function useInstallExternalExtension() {
    return useServerMutation<ExtensionRepo_ExtensionInstallResponse, InstallExternalExtension_Variables>({
        endpoint: API_ENDPOINTS.EXTENSIONS.InstallExternalExtension.endpoint,
        method: API_ENDPOINTS.EXTENSIONS.InstallExternalExtension.methods[0],
        mutationKey: [API_ENDPOINTS.EXTENSIONS.InstallExternalExtension.key],
        onSuccess: async () => {
            // DEVNOTE: No need to refetch, the websocket listener will do it
        },
    })
}

export function useInstallExternalExtensionRepository() {
    return useServerMutation<ExtensionRepo_RepositoryInstallResponse, InstallExternalExtensionRepository_Variables>({
        endpoint: API_ENDPOINTS.EXTENSIONS.InstallExternalExtensionRepository.endpoint,
        method: API_ENDPOINTS.EXTENSIONS.InstallExternalExtensionRepository.methods[0],
        mutationKey: [API_ENDPOINTS.EXTENSIONS.InstallExternalExtensionRepository.key],
        onSuccess: async () => {

        },
    })
}

export function useUninstallExternalExtension() {
    return useServerMutation<boolean, UninstallExternalExtension_Variables>({
        endpoint: API_ENDPOINTS.EXTENSIONS.UninstallExternalExtension.endpoint,
        method: API_ENDPOINTS.EXTENSIONS.UninstallExternalExtension.methods[0],
        mutationKey: [API_ENDPOINTS.EXTENSIONS.UninstallExternalExtension.key],
        onSuccess: async () => {
            // DEVNOTE: No need to refetch, the websocket listener will do it
            toast.success("Extension uninstalled successfully.")
        },
    })
}

export function useGetExtensionPayload(id: string) {
    return useServerQuery<string>({
        endpoint: API_ENDPOINTS.EXTENSIONS.GetExtensionPayload.endpoint.replace("{id}", id),
        method: API_ENDPOINTS.EXTENSIONS.GetExtensionPayload.methods[0],
        queryKey: [API_ENDPOINTS.EXTENSIONS.GetExtensionPayload.key, id],
        enabled: true,
    })
}

export function useUpdateExtensionCode() {
    return useServerMutation<boolean, UpdateExtensionCode_Variables>({
        endpoint: API_ENDPOINTS.EXTENSIONS.UpdateExtensionCode.endpoint,
        method: API_ENDPOINTS.EXTENSIONS.UpdateExtensionCode.methods[0],
        mutationKey: [API_ENDPOINTS.EXTENSIONS.UpdateExtensionCode.key],
        onSuccess: async () => {
            // DEVNOTE: No need to refetch, the websocket listener will do it
            toast.success("Extension updated successfully.")
        },
    })
}

export function useReloadExternalExtensions() {
    return useServerMutation<boolean>({
        endpoint: API_ENDPOINTS.EXTENSIONS.ReloadExternalExtensions.endpoint,
        method: API_ENDPOINTS.EXTENSIONS.ReloadExternalExtensions.methods[0],
        mutationKey: [API_ENDPOINTS.EXTENSIONS.ReloadExternalExtensions.key],
        onSuccess: async () => {

        },
    })
}

export function useReloadExternalExtensionFromSource() {
    return useServerMutation<ExtensionRepo_ExtensionInstallResponse, ReloadExternalExtensionFromSource_Variables>({
        endpoint: API_ENDPOINTS.EXTENSIONS.ReloadExternalExtensionFromSource.endpoint,
        method: API_ENDPOINTS.EXTENSIONS.ReloadExternalExtensionFromSource.methods[0],
        mutationKey: [API_ENDPOINTS.EXTENSIONS.ReloadExternalExtensionFromSource.key],
        onSuccess: async () => {
            // DEVNOTE: No need to refetch, the websocket listener will do it
        },
    })
}

export function useReloadAllExternalExtensionsFromSource() {
    return useServerMutation<ExtensionRepo_ReloadFromSourceResult>({
        endpoint: API_ENDPOINTS.EXTENSIONS.ReloadAllExternalExtensionsFromSource.endpoint,
        method: API_ENDPOINTS.EXTENSIONS.ReloadAllExternalExtensionsFromSource.methods[0],
        mutationKey: [API_ENDPOINTS.EXTENSIONS.ReloadAllExternalExtensionsFromSource.key],
        onSuccess: async () => {
            // DEVNOTE: No need to refetch, the websocket listener will do it
        },
    })
}

export function useListExtensionData() {
    return useServerQuery<Array<Extension_Extension>>({
        endpoint: API_ENDPOINTS.EXTENSIONS.ListExtensionData.endpoint,
        method: API_ENDPOINTS.EXTENSIONS.ListExtensionData.methods[0],
        queryKey: [API_ENDPOINTS.EXTENSIONS.ListExtensionData.key],
        enabled: true,
    })
}

export function useListMangaProviderExtensions() {
    return useServerQuery<Array<ExtensionRepo_MangaProviderExtensionItem>>({
        endpoint: API_ENDPOINTS.EXTENSIONS.ListMangaProviderExtensions.endpoint,
        method: API_ENDPOINTS.EXTENSIONS.ListMangaProviderExtensions.methods[0],
        queryKey: [API_ENDPOINTS.EXTENSIONS.ListMangaProviderExtensions.key],
        enabled: true,
    })
}

export function useListOnlinestreamProviderExtensions() {
    return useServerQuery<Array<ExtensionRepo_OnlinestreamProviderExtensionItem>>({
        endpoint: API_ENDPOINTS.EXTENSIONS.ListOnlinestreamProviderExtensions.endpoint,
        method: API_ENDPOINTS.EXTENSIONS.ListOnlinestreamProviderExtensions.methods[0],
        queryKey: [API_ENDPOINTS.EXTENSIONS.ListOnlinestreamProviderExtensions.key],
        enabled: true,
    })
}

export function useListCustomSourceExtensions() {
    return useServerQuery<Array<ExtensionRepo_CustomSourceExtensionItem>>({
        endpoint: API_ENDPOINTS.EXTENSIONS.ListCustomSourceExtensions.endpoint,
        method: API_ENDPOINTS.EXTENSIONS.ListCustomSourceExtensions.methods[0],
        queryKey: [API_ENDPOINTS.EXTENSIONS.ListCustomSourceExtensions.key],
        enabled: true,
    })
}

export function useAnimeListTorrentProviderExtensions() {
    return useServerQuery<Array<ExtensionRepo_AnimeTorrentProviderExtensionItem>>({
        endpoint: API_ENDPOINTS.EXTENSIONS.ListAnimeTorrentProviderExtensions.endpoint,
        method: API_ENDPOINTS.EXTENSIONS.ListAnimeTorrentProviderExtensions.methods[0],
        queryKey: [API_ENDPOINTS.EXTENSIONS.ListAnimeTorrentProviderExtensions.key],
        enabled: true,
    })
}

export function useListAnimeEntryEpisodeTabExtensions() {
    return useServerQuery<Array<ExtensionRepo_PluginEpisodeTabExtensionItem>>({
        endpoint: API_ENDPOINTS.EXTENSIONS.ListAnimeEntryEpisodeTabExtensions.endpoint,
        method: API_ENDPOINTS.EXTENSIONS.ListAnimeEntryEpisodeTabExtensions.methods[0],
        queryKey: [API_ENDPOINTS.EXTENSIONS.ListAnimeEntryEpisodeTabExtensions.key],
        enabled: true,
    })
}

export function useRunExtensionPlaygroundCode() {
    return useServerMutation<RunPlaygroundCodeResponse, RunExtensionPlaygroundCode_Variables>({
        endpoint: API_ENDPOINTS.EXTENSIONS.RunExtensionPlaygroundCode.endpoint,
        method: API_ENDPOINTS.EXTENSIONS.RunExtensionPlaygroundCode.methods[0],
        mutationKey: [API_ENDPOINTS.EXTENSIONS.RunExtensionPlaygroundCode.key],
        onSuccess: async () => {

        },
    })
}

export function useGetExtensionUserConfig(id: string) {
    const normalizedId = id.trim()
    return useServerQuery<ExtensionRepo_ExtensionUserConfig>({
        endpoint: API_ENDPOINTS.EXTENSIONS.GetExtensionUserConfig.endpoint.replace("{id}", normalizedId),
        method: API_ENDPOINTS.EXTENSIONS.GetExtensionUserConfig.methods[0],
        queryKey: [API_ENDPOINTS.EXTENSIONS.GetExtensionUserConfig.key, normalizedId],
        enabled: normalizedId.length > 0,
        gcTime: 0,
    })
}

export function useSaveExtensionUserConfig() {
    return useServerMutation<boolean, SaveExtensionUserConfig_Variables>({
        endpoint: API_ENDPOINTS.EXTENSIONS.SaveExtensionUserConfig.endpoint,
        method: API_ENDPOINTS.EXTENSIONS.SaveExtensionUserConfig.methods[0],
        mutationKey: [API_ENDPOINTS.EXTENSIONS.SaveExtensionUserConfig.key],
        onSuccess: async () => {
            // DEVNOTE: No need to refetch, the websocket listener will do it
            toast.success("Config saved successfully.")
        },
    })
}

export function useListDevelopmentModeExtensions() {
    return useServerQuery<Array<Extension_Extension>>({
        endpoint: API_ENDPOINTS.EXTENSIONS.ListDevelopmentModeExtensions.endpoint,
        method: API_ENDPOINTS.EXTENSIONS.ListDevelopmentModeExtensions.methods[0],
        queryKey: [API_ENDPOINTS.EXTENSIONS.ListDevelopmentModeExtensions.key],
        enabled: true,
    })
}

export function useReloadExternalExtension() {
    const queryClient = useQueryClient()
    return useServerMutation<boolean, ReloadExternalExtension_Variables>({
        endpoint: API_ENDPOINTS.EXTENSIONS.ReloadExternalExtension.endpoint,
        method: API_ENDPOINTS.EXTENSIONS.ReloadExternalExtension.methods[0],
        mutationKey: [API_ENDPOINTS.EXTENSIONS.ReloadExternalExtension.key],
        onSuccess: async () => {
            toast.success("Extension reloaded successfully.")
            queryClient.invalidateQueries({ queryKey: [API_ENDPOINTS.EXTENSIONS.ListDevelopmentModeExtensions.key] })
            queryClient.invalidateQueries({ queryKey: [API_ENDPOINTS.EXTENSIONS.GetPluginSettings.key] })
            // DEVNOTE: No need to refetch, the websocket listener will do it
        },
    })
}

export function useSetExternalExtensionDisabled() {
    return useServerMutation<boolean, SetExternalExtensionDisabled_Variables>({
        endpoint: API_ENDPOINTS.EXTENSIONS.SetExternalExtensionDisabled.endpoint,
        method: API_ENDPOINTS.EXTENSIONS.SetExternalExtensionDisabled.methods[0],
        mutationKey: [API_ENDPOINTS.EXTENSIONS.SetExternalExtensionDisabled.key],
        onSuccess: async (_data, variables) => {
            toast.success(variables.disabled ? "Extension disabled." : "Extension enabled.")
        },
    })
}

export function useGetPluginSettings() {
    return useServerQuery<ExtensionRepo_StoredPluginSettingsData>({
        endpoint: API_ENDPOINTS.EXTENSIONS.GetPluginSettings.endpoint,
        method: API_ENDPOINTS.EXTENSIONS.GetPluginSettings.methods[0],
        queryKey: [API_ENDPOINTS.EXTENSIONS.GetPluginSettings.key],
        enabled: true,
    })
}

export function useSetPluginSettingsPinnedTrays() {
    const queryClient = useQueryClient()
    return useServerMutation<boolean, SetPluginSettingsPinnedTrays_Variables>({
        endpoint: API_ENDPOINTS.EXTENSIONS.SetPluginSettingsPinnedTrays.endpoint,
        method: API_ENDPOINTS.EXTENSIONS.SetPluginSettingsPinnedTrays.methods[0],
        mutationKey: [API_ENDPOINTS.EXTENSIONS.SetPluginSettingsPinnedTrays.key],
        onSuccess: async () => {
            queryClient.invalidateQueries({ queryKey: [API_ENDPOINTS.EXTENSIONS.GetPluginSettings.key] })
        },
    })
}

export function useGrantPluginPermissions() {
    const queryClient = useQueryClient()
    return useServerMutation<boolean, GrantPluginPermissions_Variables>({
        endpoint: API_ENDPOINTS.EXTENSIONS.GrantPluginPermissions.endpoint,
        method: API_ENDPOINTS.EXTENSIONS.GrantPluginPermissions.methods[0],
        mutationKey: [API_ENDPOINTS.EXTENSIONS.GrantPluginPermissions.key],
        onSuccess: async (data) => {
            if (data) {
                toast.success("Plugin permissions granted successfully.")
                queryClient.invalidateQueries({ queryKey: [API_ENDPOINTS.EXTENSIONS.GetPluginSettings.key] })
                queryClient.invalidateQueries({ queryKey: [API_ENDPOINTS.EXTENSIONS.GetAllExtensions.key] })
            }
        },
    })
}

export function useGetMarketplaceExtensions(marketplaceUrl?: string) {
    const url = marketplaceUrl ? `?marketplace=${encodeURIComponent(marketplaceUrl)}` : ""
    return useServerQuery<Array<Extension_Extension>>({
        endpoint: `${API_ENDPOINTS.EXTENSIONS.GetMarketplaceExtensions.endpoint}${url}`,
        method: API_ENDPOINTS.EXTENSIONS.GetMarketplaceExtensions.methods[0],
        queryKey: [API_ENDPOINTS.EXTENSIONS.GetMarketplaceExtensions.key, marketplaceUrl],
        enabled: true,
    })
}

export function useGetExtensionUpdateData() {
    return useServerQuery<Array<ExtensionRepo_UpdateData>>({
        endpoint: API_ENDPOINTS.EXTENSIONS.GetExtensionUpdateData.endpoint,
        method: API_ENDPOINTS.EXTENSIONS.GetExtensionUpdateData.methods[0],
        queryKey: [API_ENDPOINTS.EXTENSIONS.GetExtensionUpdateData.key],
        enabled: true,
    })
}

export function useListExtensionGitTokens() {
    return useServerQuery<Array<ExtensionRepo_GitTokenInfo>>({
        endpoint: API_ENDPOINTS.EXTENSIONS.ListExtensionGitTokens.endpoint,
        method: API_ENDPOINTS.EXTENSIONS.ListExtensionGitTokens.methods[0],
        queryKey: [API_ENDPOINTS.EXTENSIONS.ListExtensionGitTokens.key],
        enabled: true,
    })
}

export function useSetExtensionGitToken() {
    const queryClient = useQueryClient()

    return useServerMutation<boolean, SetExtensionGitToken_Variables>({
        endpoint: API_ENDPOINTS.EXTENSIONS.SetExtensionGitToken.endpoint,
        method: API_ENDPOINTS.EXTENSIONS.SetExtensionGitToken.methods[0],
        mutationKey: [API_ENDPOINTS.EXTENSIONS.SetExtensionGitToken.key],
        onSuccess: async () => {
            toast.success("Token saved")
            await queryClient.invalidateQueries({ queryKey: [API_ENDPOINTS.EXTENSIONS.ListExtensionGitTokens.key] })
        },
    })
}

export function useRemoveExtensionGitToken() {
    const queryClient = useQueryClient()

    return useServerMutation<boolean, RemoveExtensionGitToken_Variables>({
        endpoint: API_ENDPOINTS.EXTENSIONS.RemoveExtensionGitToken.endpoint,
        method: API_ENDPOINTS.EXTENSIONS.RemoveExtensionGitToken.methods[0],
        mutationKey: [API_ENDPOINTS.EXTENSIONS.RemoveExtensionGitToken.key],
        onSuccess: async () => {
            toast.success("Token removed")
            await queryClient.invalidateQueries({ queryKey: [API_ENDPOINTS.EXTENSIONS.ListExtensionGitTokens.key] })
        },
    })
}
