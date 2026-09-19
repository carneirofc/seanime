import { reportValidationFailure } from "@/lib/validation/report"
import { atomWithStorage, createJSONStorage } from "jotai/utils"
import type { z } from "zod"

type AtomWithStorageOptions = {
    getOnInit?: boolean
}

/**
 * `atomWithStorage`, but the persisted value is validated before it is trusted.
 *
 * Jotai's `atomWithStorage` `JSON.parse`s whatever is in `localStorage` and hands it back
 * as-is; it only falls back to `initialValue` when the key is *absent*. So a value written
 * by an older Seanime version - a renamed field, a changed enum, an object where a string
 * is now expected - flows straight into application state and surfaces later as a confusing
 * render crash, far from the real cause.
 *
 * This wrapper re-parses through the schema on read and falls back to `initialValue` when
 * the stored shape no longer matches. That makes an upgrade self-healing rather than a
 * support request. Writes are unchanged.
 *
 * Use it for object- and array-valued preferences. A plain boolean/string/number atom gains
 * nothing from a schema and should keep using `atomWithStorage` directly.
 */
export function atomWithValidatedStorage<T>(
    key: string,
    schema: z.ZodType<T>,
    initialValue: T,
    options?: AtomWithStorageOptions,
) {
    const baseStorage = createJSONStorage<T>(() => localStorage)

    const storage = {
        ...baseStorage,
        getItem: (storedKey: string, fallback: T): T => {
            const raw = baseStorage.getItem(storedKey, fallback)

            // Absent key: jotai already returned the fallback, nothing to validate.
            if (raw === fallback) return fallback

            const result = schema.safeParse(raw)
            if (result.success) return result.data

            reportValidationFailure(`localStorage:${storedKey}`, result.error)
            return fallback
        },
    }

    return atomWithStorage<T>(key, initialValue, storage, options)
}

/**
 * Read and validate a `localStorage`/`sessionStorage` value outside of jotai.
 *
 * Returns `fallback` when the key is missing, the contents are not valid JSON, the shape
 * does not match, or storage is unavailable altogether (private windows and embedded
 * webviews can throw on access).
 */
export function readValidatedStorage<T>(
    key: string,
    schema: z.ZodType<T>,
    fallback: T,
    storage: "local" | "session" = "local",
): T {
    if (typeof window === "undefined") return fallback

    let raw: string | null
    try {
        raw = (storage === "session" ? window.sessionStorage : window.localStorage).getItem(key)
    }
    catch {
        return fallback
    }

    if (raw === null) return fallback

    let parsed: unknown
    try {
        parsed = JSON.parse(raw)
    }
    catch {
        reportValidationFailure(`${storage}Storage:${key}`, "value is not valid JSON")
        return fallback
    }

    const result = schema.safeParse(parsed)
    if (result.success) return result.data

    reportValidationFailure(`${storage}Storage:${key}`, result.error)
    return fallback
}
