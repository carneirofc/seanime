import { useGetLibraryDiskUsage } from "@/api/hooks/library.hooks"
import { SettingsCard } from "@/app/(main)/settings/_components/settings-card"
import { SettingsSubmitButton } from "@/app/(main)/settings/_components/settings-submit-button"
import { DataSettings } from "@/app/(main)/settings/_containers/data-settings"
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion"
import { Field } from "@/components/ui/form"
import { ProgressBar } from "@/components/ui/progress-bar"
import { Separator } from "@/components/ui/separator"
import { javascript } from "@codemirror/lang-javascript"
import { vscodeDark } from "@uiw/codemirror-theme-vscode"
import CodeMirror from "@uiw/react-codemirror"
import React from "react"
import { useFormContext, useWatch } from "react-hook-form"
import { FcFolder } from "react-icons/fc"

type LibrarySettingsProps = {
    isPending: boolean
}

export function AnimeLibrarySettings(props: LibrarySettingsProps) {

    const { isPending } = props

    const useLegacyMatching = useWatch({ name: "scannerUseLegacyMatching" })


    return (
        <div className="space-y-4">

            <DiskUsageCard />

            <SettingsCard>
                <Field.DirectorySelector
                    name="libraryPath"
                    label="Library directory"
                    leftIcon={<FcFolder />}
                    help="Path of the directory where your media files ared located. (Keep the casing consistent)"
                    shouldExist
                />

                <Field.MultiDirectorySelector
                    name="libraryPaths"
                    label="Additional library directories"
                    leftIcon={<FcFolder />}
                    help="Include additional directory paths if your library is spread across multiple locations."
                    shouldExist
                />
            </SettingsCard>

            <SettingsCard>

                <Field.Switch
                    side="right"
                    name="autoScan"
                    label="Automatically refresh library"
                    moreHelp={<p>
                        When adding batches, not all files are guaranteed to be picked up.
                    </p>}
                />

                <Field.Switch
                    side="right"
                    name="refreshLibraryOnStart"
                    label="Refresh library on startup"
                />
            </SettingsCard>

            {/*<SettingsCard title="Advanced">*/}

            <Accordion
                type="single"
                collapsible
                className="border rounded-md"
                triggerClass="dark:bg-(--paper)"
                contentClass="pt-2! dark:bg-(--paper)"
                defaultValue={(useLegacyMatching) ? "more" : undefined}
            >
                <AccordionItem value="more">
                    <AccordionTrigger className="bg-gray-900 rounded-md" data-settings-anime-library="advanced-accordion-trigger">
                        Advanced
                    </AccordionTrigger>
                    <AccordionContent className="space-y-4">
                        {!useLegacyMatching && <div className="space-y-4">
                            <div>
                                <p className="font-semibold text-lg mb-2">Scanner Configuration</p>
                                <p className="text-sm text-(--muted) mb-4">
                                    Configure advanced scanner rules in JSON format. This allows you to define custom matching and hydration rules for
                                    your library.
                                </p>
                            </div>
                            <ScannerConfigEditor />
                        </div>}

                        <>
                            <Field.Switch
                                name="scannerUseLegacyMatching"
                                label="Use legacy matching algorithm"
                                help="Enable to use the legacy matching algorithms. (Versions 3.4 and below)"
                                moreHelp="The legacy matching algorithm uses simpler methods which may be less accurate."
                            />
                        </>

                        {useLegacyMatching && <div className="flex flex-col md:flex-row gap-3">
                            <Field.Select
                                options={[
                                    { value: "-", label: "Levenshtein + Sorensen-Dice (Default)" },
                                    { value: "sorensen-dice", label: "Sorensen-Dice" },
                                    { value: "jaccard", label: "Jaccard" },
                                ]}
                                name="scannerMatchingAlgorithm"
                                label="Matching algorithm"
                                help="Choose the algorithm used to match files to AniList entries."
                            />
                            <Field.Number
                                name="scannerMatchingThreshold"
                                label="Matching threshold"
                                placeholder="0.5"
                                help="The minimum score required for a file to be matched to an AniList entry. Default is 0.5."
                                formatOptions={{
                                    minimumFractionDigits: 1,
                                    maximumFractionDigits: 1,
                                }}
                                max={1.0}
                                step={0.1}
                            />
                        </div>}

                        <Separator />

                        <DataSettings />
                    </AccordionContent>
                </AccordionItem>
            </Accordion>

            {/*</SettingsCard>*/}

            <SettingsSubmitButton isPending={isPending} />

        </div>
    )
}

function DiskUsageCard() {
    const { data, isLoading } = useGetLibraryDiskUsage()

    if (isLoading || !data?.paths?.length) return null

    return (
        <SettingsCard>
            <p className="font-medium text-sm mb-3">Disk usage</p>
            <div className="space-y-4">
                {data.paths.map((info) => (
                    <div key={info.path} className="space-y-1.5">
                        <div className="flex items-center justify-between text-sm">
                            <span className="text-(--muted) truncate max-w-[60%]" title={info.path}>{info.path}</span>
                            <span className="text-(--muted) shrink-0">
                                {info.usedGB.toFixed(1)} GB / {info.totalGB.toFixed(1)} GB
                                &nbsp;·&nbsp;
                                <span className={info.usedPct > 90 ? "text-red-400" : info.usedPct > 75 ? "text-yellow-400" : "text-green-400"}>
                                    {info.usedPct.toFixed(1)}% used
                                </span>
                            </span>
                        </div>
                        <ProgressBar
                            value={info.usedPct}
                            size="sm"
                            indicatorClass={info.usedPct > 90 ? "bg-red-500" : info.usedPct > 75 ? "bg-yellow-500" : "bg-green-500"}
                        />
                        <p className="text-xs text-(--muted)">{info.freeGB.toFixed(1)} GB free ({info.freePct.toFixed(1)}%)</p>
                    </div>
                ))}
            </div>
        </SettingsCard>
    )
}

function ScannerConfigEditor() {
    const { setValue } = useFormContext()
    const scannerConfig = useWatch({ name: "scannerConfig" })

    const [value, setLocalValue] = React.useState(scannerConfig || "")

    React.useEffect(() => {
        setLocalValue(scannerConfig || "")
    }, [scannerConfig])

    const handleChange = React.useCallback((val: string) => {
        setLocalValue(val)
        setValue("scannerConfig", val, { shouldDirty: true })
    }, [setValue])

    return (
        <div className="overflow-hidden rounded-md border">
            <CodeMirror
                value={value}
                height="400px"
                theme={vscodeDark}
                extensions={[javascript()]}
                onChange={handleChange}
                basicSetup={{
                    lineNumbers: true,
                    foldGutter: true,
                    bracketMatching: true,
                    syntaxHighlighting: true,
                    highlightActiveLine: true,
                }}
                placeholder={`{
  "matching": {
    "rules": []
  },
  "hydration": {
    "rules": []
  },
  "logs": {
    "verbose": false
  }
}`}
            />
        </div>
    )
}

