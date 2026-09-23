import { Extension_Extension, Extension_InvalidExtension, ExtensionRepo_ExtensionHealth, ExtensionRepo_UpdateData } from "@/api/generated/types"
import {
    useFetchExternalExtensionData,
    useInstallExternalExtension,
    useReloadExternalExtension,
    useReloadExternalExtensionFromSource,
    useSetExternalExtensionDisabled,
    useUninstallExternalExtension,
} from "@/api/hooks/extensions.hooks"
import { ExtensionDetails } from "@/app/(main)/extensions/_components/extension-details"
import { ExtensionIcon } from "@/app/(main)/extensions/_components/extension-icon"
import { ExtensionCodeModal } from "@/app/(main)/extensions/_containers/extension-code"
import { ExtensionUserConfigModal } from "@/app/(main)/extensions/_containers/extension-user-config"
import { getExtensionLanguageLabel } from "@/app/(main)/extensions/_lib/extension-filters"
import { ConfirmationDialog, useConfirmationDialog } from "@/components/shared/confirmation-dialog"
import { AppLayoutStack } from "@/components/ui/app-layout"
import { Badge } from "@/components/ui/badge"
import { Button, IconButton } from "@/components/ui/button"
import { cn } from "@/components/ui/core/styling"
import { LoadingOverlay } from "@/components/ui/loading-spinner"
import { Modal } from "@/components/ui/modal"
import { Popover } from "@/components/ui/popover"
import { Tooltip } from "@/components/ui/tooltip"
import { useRouter } from "@/lib/navigation"
import React from "react"
import { GrUpdate } from "react-icons/gr"
import { LuBook, LuCode, LuEllipsisVertical, LuPower, LuRefreshCcw, LuSearch, LuSettings2 } from "react-icons/lu"
import { RiDeleteBinLine } from "react-icons/ri"
import { TbCloudDownload } from "react-icons/tb"
import { toast } from "sonner"

type ExtensionCardProps = {
    extension: Extension_Extension
    updateData?: ExtensionRepo_UpdateData | undefined
    isInstalled: boolean
    userConfigError?: Extension_InvalidExtension | undefined
    allowReload?: boolean
    isUnsafe?: boolean
    isDisabled?: boolean
    health?: ExtensionRepo_ExtensionHealth | undefined
}

export function ExtensionCard(props: ExtensionCardProps) {

    const {
        extension,
        updateData,
        isInstalled,
        userConfigError,
        allowReload,
        isUnsafe = false,
        isDisabled = false,
        health,
        ...rest
    } = props

    const router = useRouter()
    const isFailing = !isDisabled && !!health?.failing

    const isBuiltin = extension.manifestURI === "builtin"

    return (
        <div
            className={cn(
                "group/extension-card border border-[rgb(255_255_255/5%)] relative overflow-hidden",
                "bg-gray-900 rounded-xl p-3",
                !!updateData && "border-(--green)",
                userConfigError && "border-(--orange)",
                isFailing && "border-red-800",
                isDisabled && "opacity-70 border-gray-700",
            )}
        >
            <div className="absolute z-0 right-0 top-0 h-full w-full max-w-[150px] bg-linear-to-l to-gray-950"></div>

            <div className="absolute top-3 right-3 z-2">
                <div className=" flex flex-row gap-1 z-2 flex-wrap justify-end">

                    {!!extension.userConfig && (
                        <>
                            <ExtensionUserConfigModal extension={extension} userConfigError={userConfigError}>
                                <div>
                                    <Tooltip
                                        side="top"
                                        trigger={<IconButton
                                            size="sm"
                                            intent={userConfigError ? "warning" : "gray-basic"}
                                            icon={<LuSettings2 />}
                                            className={cn(
                                                userConfigError && "animate-bounce",
                                            )}
                                        />}
                                    >Preferences</Tooltip>
                                </div>
                            </ExtensionUserConfigModal>
                        </>
                    )}


                    <ExtensionSettings
                        extension={extension}
                        isInstalled={isInstalled}
                        updateData={updateData}
                        allowReload={allowReload}
                        isDisabled={isDisabled}
                    >
                        <div>
                            <Tooltip
                                trigger={<IconButton
                                    size="sm"
                                    intent={!updateData ? "gray-basic" : "gray-subtle"}
                                    icon={<LuEllipsisVertical />}
                                />}
                            >Info</Tooltip>
                        </div>
                    </ExtensionSettings>
                </div>
                <div className="flex flex-row gap-1 z-2 flex-wrap justify-end">

                    {!!extension.readme && (
                        <div onClick={() => window.open(extension.readme, "_blank")}>
                            <Tooltip
                                side="left"
                                trigger={<IconButton
                                    size="sm"
                                    intent="gray-basic"
                                    icon={<LuBook />}
                                />}
                            >
                                Documentation
                            </Tooltip>
                        </div>
                    )}
                    {!isBuiltin && (
                        <ExtensionCodeModal extension={extension}>
                            <div>
                                <Tooltip
                                    trigger={<IconButton
                                        size="sm"
                                        intent="gray-basic"
                                        icon={<LuCode />}
                                    />}
                                    side="right"
                                >Code</Tooltip>
                            </div>
                        </ExtensionCodeModal>
                    )}
                </div>
            </div>

            <div className="z-1 relative flex flex-col h-full">
                <div className="flex gap-3 pr-16">
                    <ExtensionIcon icon={extension.icon} name={extension.name} />

                    <div>
                        <p
                            className={cn(
                                "font-semibold line-clamp-1 flex items-center gap-1",
                                extension.type === "custom-source" && "cursor-pointer hover:underline hover:underline-offset-2 hover:decoration-2 hover:decoration-solid hover:decoration-gray-500",
                            )}
                            onClick={() => {
                                if (extension.type === "custom-source") {
                                    router.push(`/custom-sources?provider=${extension.id}`)
                                }
                            }}
                        >
                            {extension.name}
                            {extension.type === "custom-source" && <LuSearch className="ml-1 text-lg inline-block" />}
                        </p>
                        <Popover
                            className="text-sm cursor-pointer" trigger={<p className="opacity-30 mt-1 text-xs line-clamp-1 tracking-wide">
                            {extension.description}
                        </p>}
                        >
                            {extension.description}
                        </Popover>
                    </div>
                </div>

                <div className="flex gap-2 flex-wrap pt-4 flex-1 items-end">
                    {isBuiltin && <Badge className="rounded-md tracking-wide border-transparent px-0 italic opacity-50" intent="unstyled">
                        Built-in
                    </Badge>}
                    {isDisabled && <Badge className="rounded-md tracking-wide border-transparent bg-transparent opacity-50 px-0" intent="warning">
                        Disabled
                    </Badge>}
                    {isFailing && <Tooltip
                        side="top"
                        className="max-w-md break-words"
                        trigger={<Badge className="rounded-md tracking-wide cursor-help" intent="alert">
                            Failing
                        </Badge>}
                    >
                        {health?.consecutiveFailures} failed calls in a row. Last error: {health?.lastError}
                    </Tooltip>}
                    {!!extension.version && !updateData && <Badge className="rounded-md tracking-wide" intent={!!updateData ? "success" : "unstyled"}>
                        {extension.version}
                    </Badge>}
                    {!!extension.version && updateData && <ExtensionCodeModal extension={extension} diff={updateData?.payload ?? ""} readOnly>
                        <Badge className="cursor-pointer rounded-md tracking-wide" intent={!!updateData ? "success" : "unstyled"}>
                            {extension.version}{updateData.version !== extension.version
                            ? " → " + updateData.version
                            : " → new code"}
                        </Badge>
                    </ExtensionCodeModal>}
                    {!isBuiltin && <Badge className="rounded-md" intent="unstyled">
                        {extension.author}
                    </Badge>}
                    {extension.lang?.toUpperCase() !== "MULTI" && <Badge className="border-transparent rounded-md px-0!" intent="unstyled">
                        {getExtensionLanguageLabel(extension.lang)}
                    </Badge>}
                </div>

            </div>
        </div>
    )
}

type ExtensionSettingsProps = {
    extension: Extension_Extension
    children?: React.ReactElement
    isInstalled: boolean
    updateData?: ExtensionRepo_UpdateData | undefined
    allowReload?: boolean
    isDisabled?: boolean
}

/**
 * Details modal with update / disable / uninstall actions.
 * The actions live in ExtensionSettingsContent so their mutation hooks only
 * mount while the modal is open, not once per card.
 */
export function ExtensionSettings(props: ExtensionSettingsProps) {
    const { children, ...rest } = props
    return (
        <Modal
            trigger={children}
            contentClass="max-w-3xl"
        >
            <ExtensionSettingsContent {...rest} />
        </Modal>
    )
}

function ExtensionSettingsContent(props: Omit<ExtensionSettingsProps, "children">) {

    const {
        extension,
        isInstalled,
        updateData,
        allowReload,
        isDisabled = false,
    } = props

    const isBuiltin = extension.manifestURI === "builtin"

    const { mutate: uninstall, isPending: isUninstalling } = useUninstallExternalExtension()

    const { mutate: fetchExtensionData, data: fetchedExtensionData, isPending: isFetchingData, reset } = useFetchExternalExtensionData(extension.id)

    const { mutate: reloadExternalExtension, isPending: isReloadingExtension } = useReloadExternalExtension()

    const { mutate: reloadFromSource, isPending: isReloadingFromSource } = useReloadExternalExtensionFromSource()

    const { mutate: setExternalExtensionDisabled, isPending: isTogglingDisabled } = useSetExternalExtensionDisabled()


    const confirmUninstall = useConfirmationDialog({
        title: `Remove ${extension.name}`,
        description: "This action cannot be undone.",
        onConfirm: () => {
            uninstall({
                id: extension.id,
            })
        },
    })

    const {
        mutate: installExtension,
        data: installResponse,
        isPending: isInstalling,
    } = useInstallExternalExtension()

    React.useEffect(() => {
        if (installResponse) {
            toast.success(installResponse.message)
            reset()
        }
    }, [installResponse])

    const checkingForUpdatesRef = React.useRef(false)

    function handleCheckUpdate() {
        fetchExtensionData({
            manifestUri: extension.manifestURI,
        })
        checkingForUpdatesRef.current = true
    }

    React.useEffect(() => {

        if (fetchedExtensionData && checkingForUpdatesRef.current) {
            checkingForUpdatesRef.current = false

            if (fetchedExtensionData.version !== extension.version) {
                toast.success("Update available")
            } else {
                toast.info("The extension is up to date")
            }
        }
    }, [fetchedExtensionData])

    return (
        <>
            {(isUninstalling || isTogglingDisabled) && <LoadingOverlay />}

            <ExtensionDetails extension={extension} />

            {!isBuiltin && (
                <>

                    {isInstalled && (
                        <div className="flex gap-2">
                            <>
                                {!!extension.manifestURI && <Button
                                    intent="gray-outline"
                                    leftIcon={<GrUpdate className="text-lg" />}
                                    disabled={!extension.manifestURI}
                                    onClick={handleCheckUpdate}
                                    loading={isFetchingData}
                                >
                                    Check for updates
                                </Button>}

                                {!!extension.manifestURI && <Button
                                    intent="gray-outline"
                                    leftIcon={<LuRefreshCcw className="text-lg" />}
                                    disabled={!extension.manifestURI}
                                    loading={isReloadingFromSource}
                                    onClick={() => {
                                        if (!extension.id) return toast.error("Extension has no ID")
                                        reloadFromSource({ id: extension.id }, {
                                            onSuccess: (data) => {
                                                toast.success(data?.message || "Reloaded from source")
                                            },
                                        })
                                    }}
                                >
                                    Reload from source
                                </Button>}

                                <Button
                                    intent={isDisabled ? "success-subtle" : "warning-subtle"}
                                    leftIcon={<LuPower className="text-lg" />}
                                    loading={isTogglingDisabled}
                                    onClick={() => {
                                        if (!extension.id) return toast.error("Extension has no ID")
                                        setExternalExtensionDisabled({ id: extension.id, disabled: !isDisabled })
                                    }}
                                >
                                    {isDisabled ? "Enable" : "Disable"}
                                </Button>

                                <Button
                                    intent="alert-subtle"
                                    leftIcon={<RiDeleteBinLine className="text-xl" />}
                                    onClick={confirmUninstall.open}
                                >
                                    Uninstall
                                </Button>

                                <div className="flex flex-1"></div>

                                {(allowReload && !isBuiltin) && (
                                    <div>
                                        <Tooltip
                                            side="right" trigger={<IconButton
                                            size="sm"
                                            intent="gray-basic"
                                            icon={<LuRefreshCcw />}
                                            onClick={() => {
                                                if (!extension.id) return toast.error("Extension has no ID")
                                                reloadExternalExtension({ id: extension.id })
                                            }}
                                            disabled={isReloadingExtension}
                                        />}
                                        >Reload</Tooltip>
                                    </div>
                                )}
                            </>
                        </div>
                    )}


                    {((!!fetchedExtensionData && fetchedExtensionData?.version !== extension.version) || !!updateData) && (
                        <AppLayoutStack>
                            <p className="">
                                Update available: <span className="font-bold text-white">
                                    {(updateData?.payloadChanged && updateData?.version === extension.version)
                                        ? "the code changed upstream"
                                        : (fetchedExtensionData?.version || updateData?.version)}
                                </span>
                            </p>
                            <div className="flex gap-2">
                                <ExtensionCodeModal extension={extension} diff={updateData?.payload ?? ""} readOnly>
                                    <Button
                                        size="md"
                                        intent="gray-subtle"
                                    >
                                        View updated code
                                    </Button>
                                </ExtensionCodeModal>
                                <Button
                                    intent="white"
                                    leftIcon={<TbCloudDownload className="text-lg" />}
                                    loading={isInstalling}
                                    onClick={() => {
                                        installExtension({
                                            manifestUri: fetchedExtensionData?.manifestURI || updateData?.manifestURI || "",
                                        })
                                    }}
                                >
                                    Install update
                                </Button>
                            </div>
                        </AppLayoutStack>
                    )}

                    <ConfirmationDialog {...confirmUninstall} />
                </>
            )}
        </>
    )
}
