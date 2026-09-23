// CodeMirror and its language/merge packages are large; this module is only
// loaded when a code modal is opened (see extension-code.tsx).
import { javascript } from "@codemirror/lang-javascript"
import { StreamLanguage } from "@codemirror/language"
import { go } from "@codemirror/legacy-modes/mode/go"
import { unifiedMergeView } from "@codemirror/merge"
import { vscodeDark } from "@uiw/codemirror-theme-vscode"
import CodeMirror, { EditorView } from "@uiw/react-codemirror"
import React, { useMemo } from "react"

function getCodeMirrorLanguageExtensions(language?: string) {
    const normalized = language?.toLowerCase()
    if (normalized === "go") return [StreamLanguage.define(go)]

    return [javascript({ typescript: normalized === "typescript" })]
}

function normalizeDiffCode(code: string) {
    return code.replace(/^\uFEFF/, "").replace(/\r\n?/g, "\n")
}

export function ExtensionCodeEditor({
    code,
    setCode,
    language,
    readOnly,
}: { code: string, language: string, setCode: any, readOnly?: boolean }) {

    return (
        <div className="overflow-hidden rounded-md">
            <CodeMirror
                value={code}
                height="75vh"
                theme={vscodeDark}
                extensions={getCodeMirrorLanguageExtensions(language)}
                onChange={setCode}
                readOnly={readOnly}
            />
        </div>
    )
}

interface Props {
    oldCode: string;
    currentCode: string;
    language: string;
}

export const UnifiedDiff = ({ oldCode, currentCode, language }: Props) => {
    const normalizedOldCode = useMemo(() => normalizeDiffCode(oldCode), [oldCode])
    const normalizedCurrentCode = useMemo(() => normalizeDiffCode(currentCode), [currentCode])
    const extensions = useMemo(() => [
        ...getCodeMirrorLanguageExtensions(language),
        unifiedMergeView({
            original: normalizedOldCode,
            highlightChanges: true,
            gutter: true,
            mergeControls: false,
            allowInlineDiffs: true,
        }),
    ], [normalizedOldCode, language])

    const hideDiffStyles = EditorView.theme({
        ".cm-changedText": {
            background: "rgba(100, 160, 128, .1) !important",
        },
        ".cm-changedLine": {
            background: "rgba(100, 160, 128, .06) !important",
        },
    })

    return (
        <div className="overflow-hidden rounded-md">
            <CodeMirror
                value={normalizedCurrentCode}
                height="75vh"
                theme={vscodeDark}
                extensions={[hideDiffStyles, ...extensions]}
                readOnly
            />
        </div>
    )
}
