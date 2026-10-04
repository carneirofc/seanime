import React from "react"
import { BsThreeDotsVertical } from "react-icons/bs"
import * as ResizablePrimitive from "react-resizable-panels"
import { cn } from "../ui/core/styling"

// Wrapper over react-resizable-panels v4 that keeps the v2-era props callers use:
// `direction` (v4: `orientation`), `autoSaveId` (v4: the useDefaultLayout hook), and
// numeric sizes as percentages (v4 reads bare numbers as pixels).

type PanelGroupProps = Omit<React.ComponentProps<typeof ResizablePrimitive.Group>, "orientation"> & {
    direction?: "horizontal" | "vertical"
    // Persists the layout in localStorage under this id.
    autoSaveId?: string
}

function PersistedGroup({ autoSaveId, ...props }: React.ComponentProps<typeof ResizablePrimitive.Group> & { autoSaveId: string }) {
    const { defaultLayout, onLayoutChanged } = ResizablePrimitive.useDefaultLayout({
        id: autoSaveId,
        storage: localStorage,
    })
    return <ResizablePrimitive.Group defaultLayout={defaultLayout} onLayoutChanged={onLayoutChanged} {...props} />
}

export const ResizablePanelGroup = ({
    className,
    direction = "horizontal",
    autoSaveId,
    ...props
}: PanelGroupProps) => {
    const groupProps = {
        orientation: direction,
        className: cn("flex h-full w-full", direction === "vertical" && "flex-col", className),
        ...props,
    }
    if (autoSaveId) {
        return <PersistedGroup autoSaveId={autoSaveId} {...groupProps} />
    }
    return <ResizablePrimitive.Group {...groupProps} />
}

type PanelSize = number | string | undefined

// Bare numbers keep their v2 meaning: a percentage of the group.
function asPercent(size: PanelSize): PanelSize {
    return typeof size === "number" ? `${size}%` : size
}

export const ResizablePanel = ({ defaultSize, minSize, maxSize, collapsedSize, ...props }: React.ComponentProps<typeof ResizablePrimitive.Panel>) => (
    <ResizablePrimitive.Panel
        defaultSize={asPercent(defaultSize)}
        minSize={asPercent(minSize)}
        maxSize={asPercent(maxSize)}
        collapsedSize={asPercent(collapsedSize)}
        {...props}
    />
)

// A separator in a vertical group is horizontal: aria-orientation="horizontal".
export const ResizableHandle = ({
    withHandle,
    className,
    ...props
}: React.ComponentProps<typeof ResizablePrimitive.Separator> & {
    withHandle?: boolean
}) => (
    <ResizablePrimitive.Separator
        className={cn(
            "relative flex w-px items-center justify-center bg-(--border) after:absolute after:inset-y-0 after:left-1/2 after:w-1 after:-translate-x-1/2 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring focus-visible:ring-offset-1 aria-[orientation=horizontal]:h-px aria-[orientation=horizontal]:w-full aria-[orientation=horizontal]:after:left-0 aria-[orientation=horizontal]:after:h-1 aria-[orientation=horizontal]:after:w-full aria-[orientation=horizontal]:after:-translate-y-1/2 aria-[orientation=horizontal]:after:translate-x-0 [&[aria-orientation=horizontal]>div]:rotate-90",
            className,
        )}
        {...props}
    >
        {withHandle && (
            <div className="z-10 flex h-4 w-3 items-center justify-center rounded-sm border bg-(--border)">
                <BsThreeDotsVertical className="h-2.5 w-2.5" />
            </div>
        )}
    </ResizablePrimitive.Separator>
)
