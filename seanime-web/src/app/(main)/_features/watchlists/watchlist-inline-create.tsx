import { IconButton } from "@/components/ui/button"
import { TextInput } from "@/components/ui/text-input"
import React from "react"
import { LuPlus } from "react-icons/lu"

type InlineCreateProps = {
    placeholder: string
    onCreate: (name: string) => void
    icon?: React.ReactNode
    className?: string
}

/**
 * A compact input + button used to create a grouping or watchlist inline.
 * Submits on Enter or button click, then clears.
 */
export function InlineCreate(props: InlineCreateProps) {
    const [value, setValue] = React.useState("")

    const submit = () => {
        const name = value.trim()
        if (!name) return
        props.onCreate(name)
        setValue("")
    }

    return (
        <div className={props.className ? `flex items-center gap-2 ${props.className}` : "flex items-center gap-2"}>
            <TextInput
                value={value}
                onValueChange={setValue}
                placeholder={props.placeholder}
                size="sm"
                onKeyDown={(e: React.KeyboardEvent) => {
                    if (e.key === "Enter") {
                        e.preventDefault()
                        submit()
                    }
                }}
            />
            <IconButton
                icon={props.icon ?? <LuPlus />}
                intent="primary-subtle"
                size="sm"
                onClick={submit}
                disabled={!value.trim()}
            />
        </div>
    )
}
