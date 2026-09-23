import { Extension_Extension } from "@/api/generated/types"
import { useInstallExternalExtension } from "@/api/hooks/extensions.hooks"
import { ExtensionIcon } from "@/app/(main)/extensions/_components/extension-icon"
import { EXTENSION_TYPE, getExtensionLanguageLabel } from "@/app/(main)/extensions/_lib/extension-filters"
import { Badge } from "@/components/ui/badge"
import { IconButton } from "@/components/ui/button"
import { cn } from "@/components/ui/core/styling"
import { Popover } from "@/components/ui/popover"
import capitalize from "lodash/capitalize"
import React from "react"
import { LuCheck, LuDownload } from "react-icons/lu"
import { toast } from "sonner"

type MarketplaceExtensionCardProps = {
    extension: Extension_Extension
    isInstalled: boolean
    hideInstallButton?: boolean
    showType?: boolean
    className?: string
}

export function MarketplaceExtensionCard(props: MarketplaceExtensionCardProps) {

    const {
        extension,
        isInstalled,
        hideInstallButton,
        showType,
        className,
    } = props

    const { mutate: installExtension, isPending: isInstalling } = useInstallExternalExtension()

    return (
        <div
            className={cn(
                "group/extension-card border border-[rgb(255_255_255/5%)] relative overflow-hidden",
                "bg-gray-900 rounded-xl p-3",
                className,
            )}
        >
            {!hideInstallButton && <div className="absolute top-3 right-3 z-2">
                <div className=" flex flex-row gap-1 z-2 flex-wrap justify-end">
                    {!isInstalled ? <IconButton
                        size="sm"
                        intent="primary-subtle"
                        icon={<LuDownload />}
                        loading={isInstalling}
                        onClick={() => installExtension({ manifestUri: extension.manifestURI }, {
                            onSuccess: data => {
                                if (data) toast.success(data.message)
                            },
                        })}
                    /> : <IconButton
                        size="sm"
                        disabled
                        intent="success-subtle"
                        icon={<LuCheck />}
                    />
                    }
                </div>
            </div>}

            <div className="z-1 relative space-y-3">
                <div className="flex gap-3 pr-16">
                    <ExtensionIcon icon={extension.icon} name={extension.name} />

                    <div>
                        <p className="font-semibold line-clamp-1">
                            {extension.name}
                        </p>
                        <p className="text-xs line-clamp-1 tracking-wide">
                            {showType && <span className="opacity-70">{EXTENSION_TYPE[extension.type] ?? extension.type} - </span>}
                            <span className="opacity-30">{extension.id}</span>
                        </p>
                    </div>
                </div>

                {extension.description && (
                    <Popover
                        trigger={<p className="text-sm text-(--muted) line-clamp-2 cursor-pointer">
                            {extension.description}
                        </p>}
                    >
                        <p className="text-sm">
                            {extension.description}
                        </p>
                    </Popover>
                )}

                <div className="flex gap-2 flex-wrap">
                    {!!extension.version && <Badge className="rounded-md tracking-wide">
                        {extension.version}
                    </Badge>}
                    <Badge className="rounded-md" intent="unstyled">
                        {extension.author}
                    </Badge>
                    {extension.lang?.toUpperCase() !== "MULTI" && <Badge className="border-transparent rounded-md" intent="blue">
                        {getExtensionLanguageLabel(extension.lang)}
                    </Badge>}
                    <Badge className="border-transparent rounded-md text-(--muted) px-0" intent="unstyled">
                        {capitalize(extension.language)}
                    </Badge>
                </div>

            </div>
        </div>
    )
}
