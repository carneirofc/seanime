import { SeaImage } from "@/components/shared/sea-image"
import { cn } from "@/components/ui/core/styling"
import React from "react"

type ExtensionIconProps = {
    icon: string | undefined
    name: string | undefined
    className?: string
}

/**
 * Extension icon, or the first letter of its name when it has none.
 */
export function ExtensionIcon({ icon, name, className }: ExtensionIconProps) {
    return (
        <div className={cn("relative rounded-md size-12 flex-none overflow-hidden", icon ? "bg-gray-900" : "bg-gray-950", className)}>
            {icon ? (
                <SeaImage
                    src={icon}
                    alt="extension icon"
                    crossOrigin="anonymous"
                    fill
                    quality={100}
                    className="object-cover"
                    isExternal
                />
            ) : (
                <div className="w-full h-full flex items-center justify-center">
                    <p className="text-2xl font-bold">{(name?.[0] ?? "?").toUpperCase()}</p>
                </div>
            )}
        </div>
    )
}
