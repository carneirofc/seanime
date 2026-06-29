import { cn } from "@/components/ui/core/styling"
import { ScrollArea, ScrollAreaProps } from "@/components/ui/scroll-area"
import React from "react"

export function ScrollAreaBox({ listClass, className, children, ...rest }: ScrollAreaProps & { listClass?: string }) {
    return <ScrollArea
        className={cn(
            "h-[calc(100dvh-25rem)] min-h-52 relative border rounded-(--radius)",
            className,
        )} {...rest}>
        <div
            className="z-5 absolute bottom-0 w-full h-8 bg-linear-to-t from-(--background) to-transparent"
        />
        <div
            className="z-5 absolute top-0 w-full h-8 bg-linear-to-b from-(--background) to-transparent"
        />
        <div className="space-y-2 p-6">
            {children}
        </div>
    </ScrollArea>
}
