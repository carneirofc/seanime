import { LuffyError } from "@/components/shared/luffy-error"
import { LoadingOverlay } from "@/components/ui/loading-spinner"
import React from "react"

export default function Page() {

    return (
        <LoadingOverlay showSpinner={false}>
            <LuffyError title="Something went wrong" />
        </LoadingOverlay>
    )

}
