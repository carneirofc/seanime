import { Extension_Extension, Extension_Type, ExtensionRepo_AllExtensions } from "@/api/generated/types"
import {
    getFailingExtensionIds,
    useGetAllExtensions,
    useInstallExternalExtension,
    useReloadAllExternalExtensionsFromSource,
    useSetExtensionAutoDisable,
    useSetExternalExtensionDisabled,
    useUninstallExternalExtension,
} from "@/api/hooks/extensions.hooks"
import { ExtensionIcon } from "@/app/(main)/extensions/_components/extension-icon"
import { AddExtensionModal } from "@/app/(main)/extensions/_containers/add-extension-modal"
import { ExtensionCard } from "@/app/(main)/extensions/_containers/extension-card"
import { GitTokensModal } from "@/app/(main)/extensions/_containers/git-tokens-modal"
import { InvalidExtensionCard, UnauthorizedExtensionPluginCard } from "@/app/(main)/extensions/_containers/invalid-extension-card"
import { groupExtensionsByType, matchesExtensionSearch, normalizeSearchTerm } from "@/app/(main)/extensions/_lib/extension-filters"
import { ConfirmationDialog, useConfirmationDialog } from "@/components/shared/confirmation-dialog"
import { LuffyError } from "@/components/shared/luffy-error"
import { SeaLink } from "@/components/shared/sea-link"
import { AppLayoutStack } from "@/components/ui/app-layout"
import { Button, IconButton } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { DropdownMenu, DropdownMenuItem } from "@/components/ui/dropdown-menu"
import { LoadingSpinner } from "@/components/ui/loading-spinner"
import { Switch } from "@/components/ui/switch"
import { TextInput } from "@/components/ui/text-input"
import { useRouter } from "@/lib/navigation"
import { formatDistanceToNow } from "date-fns"
import { atom, useSetAtom } from "jotai"
import orderBy from "lodash/orderBy"
import React from "react"
import { BiDotsVerticalRounded, BiSearch } from "react-icons/bi"
import { CgMediaPodcast } from "react-icons/cg"
import { GrInstallOption } from "react-icons/gr"
import { LuBlocks, LuDownload, LuPower } from "react-icons/lu"
import { MdDataSaverOn } from "react-icons/md"
import { PiBookFill } from "react-icons/pi"
import { RiDeleteBinLine, RiFolderDownloadFill } from "react-icons/ri"
import { TbRefresh, TbReload } from "react-icons/tb"
import { toast } from "sonner"

type ExtensionListProps = {
    children?: React.ReactNode
}

export const __extensions_currentPageAtom = atom<"installed" | "marketplace">("installed")

const TYPE_SECTIONS: { type: Extension_Type, title: string, icon: React.ReactNode }[] = [
    { type: "plugin", title: "Plugins", icon: <LuBlocks /> },
    { type: "custom-source", title: "Custom Sources", icon: <MdDataSaverOn /> },
    { type: "anime-torrent-provider", title: "Anime torrents", icon: <RiFolderDownloadFill /> },
    { type: "manga-provider", title: "Manga", icon: <PiBookFill /> },
    { type: "onlinestream-provider", title: "Online streaming", icon: <CgMediaPodcast /> },
]

const GRID_CLASS = "grid grid-cols-1 lg:grid-cols-3 2xl:grid-cols-4 gap-4"

export function ExtensionList(props: ExtensionListProps) {

    const router = useRouter()

    const [checkForUpdates, setCheckForUpdates] = React.useState(false)
    const [searchTerm, setSearchTerm] = React.useState("")
    // Filtering re-renders every card; keep typing responsive on large lists
    const deferredSearchTerm = React.useDeferredValue(searchTerm)
    const [gitTokensModalOpen, setGitTokensModalOpen] = React.useState(false)

    const { data: allExtensions, isPending: isLoading, refetch } = useGetAllExtensions(checkForUpdates)

    const setPage = useSetAtom(__extensions_currentPageAtom)

    const {
        mutate: installExtension,
        isPending: isInstalling,
    } = useInstallExternalExtension()

    const {
        mutate: reloadAllFromSource,
        isPending: isReloadingFromSource,
    } = useReloadAllExternalExtensionsFromSource()

    const { mutate: setAutoDisable, isPending: isSettingAutoDisable } = useSetExtensionAutoDisable()

    const term = normalizeSearchTerm(deferredSearchTerm)
    const view = buildListView(allExtensions, term)

    if (isLoading) return <LoadingSpinner />

    if (!allExtensions) return <LuffyError>
        Could not get extensions.
    </LuffyError>

    function renderCard(extension: Extension_Extension, isDisabled = false) {
        return (
            <ExtensionCard
                key={extension.id}
                extension={extension}
                updateData={view.updates.get(extension.id)}
                isInstalled={view.installedIds.has(extension.id)}
                userConfigError={view.userConfigErrors.get(extension.id)}
                isUnsafe={allExtensions?.unsafeExtensions?.[extension.id] ?? false}
                health={allExtensions?.health?.[extension.id]}
                isDisabled={isDisabled}
                allowReload={!isDisabled}
            />
        )
    }

    return (
        <AppLayoutStack className="gap-6">
            <div className="flex items-center gap-2 flex-wrap">
                <div>
                    <h2>
                        Extensions
                    </h2>
                    <p className="text-(--muted) text-sm">
                        Manage your plugins and content providers.
                    </p>
                </div>

                <div className="flex flex-1"></div>

                <div className="flex items-center gap-2 flex-wrap">
                    {!!allExtensions?.hasUpdate?.filter(update => !allExtensions.unsafeExtensions?.[update.extensionID]).length && (
                        <Button
                            className="rounded-full animate-pulse"
                            intent="success"
                            leftIcon={<LuDownload className="text-lg" />}
                            loading={isInstalling}
                            onClick={() => {
                                toast.info("Installing updates...")
                                allExtensions?.hasUpdate?.forEach(update => {
                                    if (allExtensions.unsafeExtensions?.[update.extensionID]) {
                                        toast.warning(`Skipped "${update.extensionID}" because it uses unsafe flags. Update it manually.`)
                                        return
                                    }
                                    installExtension({
                                        manifestUri: update.manifestURI,
                                    })
                                })
                            }}
                        >
                            Update all
                        </Button>
                    )}
                    <Button
                        className="rounded-full"
                        intent="gray-basic"
                        leftIcon={<TbReload className="text-lg" />}
                        disabled={isLoading}
                        onClick={() => {
                            setCheckForUpdates(true)
                        }}
                    >
                        Check for updates
                    </Button>
                    <Button
                        className="rounded-full"
                        intent="gray-basic"
                        leftIcon={<TbRefresh className="text-lg" />}
                        loading={isReloadingFromSource}
                        disabled={isLoading}
                        onClick={() => {
                            toast.info("Reloading extensions from source...")
                            reloadAllFromSource(undefined, {
                                onSuccess: (data) => {
                                    const reloaded = data?.reloaded?.length ?? 0
                                    const failed = Object.keys(data?.failed ?? {}).length
                                    if (failed > 0) {
                                        toast.warning(`Reloaded ${reloaded} extension(s), ${failed} failed.`)
                                    } else {
                                        toast.success(`Reloaded ${reloaded} extension(s) from source.`)
                                    }
                                    refetch()
                                },
                            })
                        }}
                    >
                        Reload from source
                    </Button>
                    <AddExtensionModal extensions={view.installedExtensions}>
                        <Button
                            className="rounded-full"
                            intent="white-subtle"
                            leftIcon={<GrInstallOption className="text-lg" />}
                        >
                            Add extensions
                        </Button>
                    </AddExtensionModal>

                    <DropdownMenu trigger={<IconButton icon={<BiDotsVerticalRounded />} intent="gray-basic" />}>

                        <DropdownMenuItem
                            onClick={() => {
                                router.push("/extensions/playground")
                            }}
                        >
                            <span>Playground</span>
                        </DropdownMenuItem>

                        <DropdownMenuItem
                            onClick={() => {
                                setPage("marketplace")
                            }}
                        >
                            <span>Marketplace</span>
                        </DropdownMenuItem>

                        <DropdownMenuItem
                            onClick={() => {
                                setGitTokensModalOpen(true)
                            }}
                        >
                            <span>Private repositories</span>
                        </DropdownMenuItem>
                    </DropdownMenu>

                    <GitTokensModal open={gitTokensModalOpen} onOpenChange={setGitTokensModalOpen} />
                </div>
            </div>

            <div className="flex items-center gap-4 flex-wrap">
                <TextInput
                    placeholder="Search installed extensions..."
                    value={searchTerm}
                    onValueChange={setSearchTerm}
                    className="pl-10"
                    fieldClass="flex-1 min-w-60"
                    leftIcon={<BiSearch />}
                />
                <div className="w-fit">
                    <Switch
                        side="right"
                        label="Auto-disable failing extensions"
                        help="Disable an extension after 5 failed calls in a row."
                        value={!!allExtensions.autoDisableFailing}
                        disabled={isSettingAutoDisable}
                        onValueChange={enabled => setAutoDisable({ enabled })}
                    />
                </div>
            </div>

            {!!term && !view.hasVisibleResults && (
                <Card className="p-8 text-center">
                    <p className="text-(--muted)">No extensions found matching your search.</p>
                </Card>
            )}

            {!!view.failing.length && <FailingExtensionsCard extensions={view.failing} allExtensions={allExtensions} />}

            {!!view.permissionsRequired.length && (
                <Card className="p-4 space-y-6">
                    <h3 className="flex gap-3 items-center">Permissions required</h3>

                    <div className={GRID_CLASS}>
                        {view.permissionsRequired.map(extension => (
                            <UnauthorizedExtensionPluginCard
                                key={extension.id}
                                extension={extension}
                                isInstalled={view.installedIds.has(extension.id)}
                                isUnsafe={allExtensions?.unsafeExtensions?.[extension.id] ?? false}
                            />
                        ))}
                    </div>
                </Card>
            )}

            {!!view.invalid.length && <InvalidExtensionsCard extensions={view.invalid} installedIds={view.installedIds} />}

            {!!view.disabled.length && (
                <Card className="p-4 space-y-6">
                    <h3 className="flex gap-3 items-center">Disabled</h3>
                    <div className={GRID_CLASS}>
                        {view.disabled.map(extension => renderCard(extension, true))}
                    </div>
                </Card>
            )}

            {TYPE_SECTIONS.map(section => !!view.enabledByType[section.type].length && (
                <Card key={section.type} className="p-4 space-y-6">
                    <div className="flex items-center gap-4">
                        <h3 className="flex gap-3 items-center">{section.icon} {section.title}</h3>
                        {section.type === "custom-source" && (
                            <SeaLink href="/custom-sources" className="text-sm underline underline-offset-2 text-(--muted) hover:text-(--foreground)">
                                Browse all sources
                            </SeaLink>
                        )}
                    </div>
                    <div className={GRID_CLASS}>
                        {view.enabledByType[section.type].map(extension => renderCard(extension))}
                    </div>
                </Card>
            ))}
        </AppLayoutStack>
    )
}

/**
 * Derives everything the list renders from the API response in a few passes,
 * with O(1) lookups for the per-card data.
 */
function buildListView(allExtensions: ExtensionRepo_AllExtensions | undefined, term: string) {
    const extensions = allExtensions?.extensions ?? []
    const disabledExtensions = allExtensions?.disabledExtensions ?? []
    const invalidExtensions = allExtensions?.invalidExtensions ?? []

    const installedIds = new Set<string>()
    for (const ext of [...extensions, ...disabledExtensions, ...invalidExtensions]) installedIds.add(ext.id)

    const updates = new Map((allExtensions?.hasUpdate ?? []).map(u => [u.extensionID, u]))
    const userConfigErrors = new Map((allExtensions?.invalidUserConfigExtensions ?? []).map(e => [e.id, e]))
    const failingIds = new Set(getFailingExtensionIds(allExtensions))

    const matches = (ext: Extension_Extension | undefined) => matchesExtensionSearch(ext, term)
    const enabled = orderBy(extensions.filter(matches), ["name", "manifestUri"])
    const disabled = orderBy(disabledExtensions.filter(matches), ["name", "manifestUri"])
    const failing = enabled.filter(ext => failingIds.has(ext.id))

    const visibleInvalid = invalidExtensions
        .filter(ext => matches(ext.extension))
        .sort((a, b) => a.id.localeCompare(b.id))
    const permissionsRequired = visibleInvalid.filter(ext => ext.code === "plugin_permissions_not_granted")
    const invalid = visibleInvalid.filter(ext => ext.code !== "plugin_permissions_not_granted")

    return {
        installedIds,
        updates,
        userConfigErrors,
        enabledByType: groupExtensionsByType(enabled),
        disabled,
        failing,
        permissionsRequired,
        invalid,
        installedExtensions: [...extensions, ...disabledExtensions],
        hasVisibleResults: enabled.length + disabled.length + visibleInvalid.length > 0,
    }
}

function FailingExtensionsCard({ extensions, allExtensions }: { extensions: Extension_Extension[], allExtensions: ExtensionRepo_AllExtensions }) {
    const { mutateAsync: setDisabled, isPending } = useSetExternalExtensionDisabled({ silent: true })

    async function disable(ids: string[]) {
        let disabled = 0
        for (const id of ids) {
            try {
                await setDisabled({ id, disabled: true })
                disabled++
            }
            catch {
                // The request layer already shows the error
            }
        }
        if (disabled) toast.success(disabled === 1 ? "Extension disabled." : `${disabled} extensions disabled.`)
    }

    return (
        <Card className="p-4 space-y-4 border-red-800">
            <div className="flex items-center gap-2 flex-wrap">
                <div>
                    <h3>Failing extensions</h3>
                    <p className="text-sm text-(--muted)">
                        These extensions loaded, but their recent calls keep failing. The source may be down or the extension may be outdated.
                    </p>
                </div>
                <div className="flex flex-1"></div>
                {extensions.length > 1 && (
                    <Button
                        intent="alert-subtle"
                        leftIcon={<LuPower className="text-lg" />}
                        loading={isPending}
                        onClick={() => disable(extensions.map(ext => ext.id))}
                    >
                        Disable all failing
                    </Button>
                )}
            </div>
            <div className="divide-y divide-(--border)">
                {extensions.map(ext => {
                    const health = allExtensions.health?.[ext.id]
                    return (
                        <div key={ext.id} className="flex items-center gap-3 py-3">
                            <ExtensionIcon icon={ext.icon} name={ext.name} className="size-10" />
                            <div className="flex-1 min-w-0">
                                <p className="font-semibold line-clamp-1">{ext.name}</p>
                                <p className="text-sm text-red-300 line-clamp-2 break-words" title={health?.lastError}>
                                    {health?.lastError || "Unknown error"}
                                </p>
                                <p className="text-xs text-(--muted)">
                                    {health?.consecutiveFailures} failed calls in a row
                                    {health?.lastErrorAt && <> · last {formatDistanceToNow(new Date(health.lastErrorAt), { addSuffix: true })}</>}
                                    {!!health?.calls && <> · {health.failures}/{health.calls} calls failed since load</>}
                                </p>
                            </div>
                            <Button
                                size="sm"
                                intent="warning-subtle"
                                leftIcon={<LuPower />}
                                disabled={isPending}
                                onClick={() => disable([ext.id])}
                            >
                                Disable
                            </Button>
                        </div>
                    )
                })}
            </div>
        </Card>
    )
}

function InvalidExtensionsCard({ extensions, installedIds }: {
    extensions: NonNullable<ExtensionRepo_AllExtensions["invalidExtensions"]>,
    installedIds: Set<string>
}) {
    const { mutateAsync: uninstall, isPending } = useUninstallExternalExtension({ silent: true })

    const removableIds = extensions
        .filter(ext => !!ext.extension?.id && ext.extension.manifestURI !== "builtin")
        .map(ext => ext.extension.id)

    const confirmRemoveAll = useConfirmationDialog({
        title: `Remove ${removableIds.length} invalid extension(s)`,
        description: "Their files and settings are deleted. This action cannot be undone.",
        onConfirm: async () => {
            let removed = 0
            for (const id of removableIds) {
                try {
                    await uninstall({ id })
                    removed++
                }
                catch {
                    // The request layer already shows the error
                }
            }
            if (removed) toast.success(`Removed ${removed} invalid extension(s).`)
        },
    })

    return (
        <Card className="p-4 space-y-6 border-red-800">
            <div className="flex items-center gap-2 flex-wrap">
                <h3 className="flex gap-3 items-center">Invalid extensions</h3>
                <div className="flex flex-1"></div>
                {removableIds.length > 1 && (
                    <Button
                        intent="alert-subtle"
                        leftIcon={<RiDeleteBinLine className="text-lg" />}
                        loading={isPending}
                        onClick={confirmRemoveAll.open}
                    >
                        Remove all invalid
                    </Button>
                )}
            </div>

            <div className={GRID_CLASS}>
                {extensions.map(extension => (
                    <InvalidExtensionCard
                        key={extension.id}
                        extension={extension}
                        isInstalled={installedIds.has(extension.id)}
                    />
                ))}
            </div>
            <ConfirmationDialog {...confirmRemoveAll} />
        </Card>
    )
}
