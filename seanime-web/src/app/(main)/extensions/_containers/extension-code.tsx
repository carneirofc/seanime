import { Extension_Extension } from "@/api/generated/types"
import { useGetExtensionPayload, useUpdateExtensionCode } from "@/api/hooks/extensions.hooks"
import { Button } from "@/components/ui/button"
import { LoadingSpinner } from "@/components/ui/loading-spinner"
import { Modal } from "@/components/ui/modal"
import React from "react"

const ExtensionCodeEditor = React.lazy(() => import("./extension-code-editor").then(m => ({ default: m.ExtensionCodeEditor })))
const UnifiedDiff = React.lazy(() => import("./extension-code-editor").then(m => ({ default: m.UnifiedDiff })))


type ExtensionCodeModalProps = {
    children?: React.ReactElement
    extension: Extension_Extension
    readOnly?: boolean
    diff?: string
}

export function ExtensionCodeModal(props: ExtensionCodeModalProps) {


    return (
        <Modal
            contentClass="max-w-5xl"
            trigger={props.children}
            title="Code"
            onInteractOutside={e => {
                if (!props.readOnly) e.preventDefault()
            }}
            // size="xl"
            // contentClass="space-y-4"
        >
            <Content {...props} />
        </Modal>
    )
}

function Content(props: ExtensionCodeModalProps) {
    const {
        extension,
        readOnly,
        diff,
    } = props

    const [code, setCode] = React.useState("")

    const { data: payload, isLoading } = useGetExtensionPayload(extension.id)

    React.useEffect(() => {
        if (payload) {
            setCode(payload)
        }
    }, [payload])

    const { mutate: updateCode, isPending } = useUpdateExtensionCode()

    React.useLayoutEffect(() => {
        setCode(extension.payload)
    }, [extension.payload])

    function handleSave() {
        if (isPending) {
            return
        }
        if (code === extension.payload) {
            return
        }
        if (code.length === 0) {
            return
        }
        updateCode({
            id: extension.id,
            payload: code,
        })
    }

    if (isLoading) {
        return <LoadingSpinner />
    }

    return (
        <>
            <div>
                <p>
                    {extension.name}
                </p>
                {!readOnly && !diff && <div className="text-sm text-(--muted)">
                    You can edit the code of the extension here.
                </div>}
            </div>
            {!readOnly && <div className="flex">
                <Button intent="white" loading={isPending} onClick={handleSave}>
                    Save
                </Button>
                <div className="flex flex-1"></div>
            </div>}
            <React.Suspense fallback={<LoadingSpinner />}>
                {!diff ? <ExtensionCodeEditor
                    code={code}
                    setCode={setCode}
                    language={extension.language}
                    readOnly={readOnly}
                /> : <UnifiedDiff oldCode={code} currentCode={diff} language={extension.language} />}
            </React.Suspense>
        </>
    )
}
