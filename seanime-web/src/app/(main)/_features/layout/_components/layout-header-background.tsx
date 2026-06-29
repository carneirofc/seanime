import { cn } from "@/components/ui/core/styling"
import { usePathname } from "@/lib/navigation"
import React from "react"

export function LayoutHeaderBackground() {

    const pathname = usePathname()

    return (
        <>
            {!pathname.startsWith("/entry") && <>
                <div
                    data-layout-header-background
                    className={cn(
                        "bg-black opacity-50 bg-contain bg-center bg-repeat z-[-2] w-full h-80 absolute bottom-0",
                    )}
                >
                </div>
                <div
                    data-layout-header-background-gradient
                    className="w-full absolute bottom-0 h-32 bg-linear-to-t from-(--background) to-transparent z-[-2]"
                />
            </>}
        </>
    )
}
