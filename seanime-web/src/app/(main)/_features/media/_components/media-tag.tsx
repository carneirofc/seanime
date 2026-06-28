import { Button, ButtonAnatomy } from "@/components/ui/button";
import { cn } from "@/components/ui/core/styling";
import { Tooltip } from "@/components/ui/tooltip";
import React from "react";

export type AL_MediaTag = {
    name: string
    description?: string | null
    isAdult?: boolean | null
    category?: string | null
    rank?: number | null
    isMediaSpoiler?: boolean | null
    isGeneralSpoiler?: boolean | null
}

export type MediaTagState = "include" | "exclude" | "neutral";
export type MediaTagProps = {
    size?: "sm" | "md" | "lg";
    mediaTag: AL_MediaTag;
    onClick?: () => void;
    state: MediaTagState;
    isMissing?: boolean;
};

export function MediaTag({ size, mediaTag, onClick, state, isMissing }: MediaTagProps) {
    const intent = React.useMemo(() => {
        switch (state) {
            case "include":
                return !isMissing ? "success" : "success-subtle"
            case "exclude":
                return !isMissing ? "alert" : "alert-subtle"
            case "neutral":
                return !isMissing ? "gray" : "gray-subtle"
        }
    }, [state])

    return <Tooltip
        className="max-w-[300px] text-xs"
        trigger={
            <Button
                onClick={() => { onClick?.() }}
                className={
                    cn(ButtonAnatomy.root({
                        size: "xs",
                        intent: intent,
                    }),
                        mediaTag.isAdult ? "border-pink-400 dark:border-pink-500" : "")
                }
            >
                {mediaTag.name}
            </Button >
        }
    >
        {mediaTag.description ?? "No description available."}
    </Tooltip >

}