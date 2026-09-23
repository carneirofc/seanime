import { Extension_Extension, Extension_Type } from "@/api/generated/types"
import { LANGUAGES_LIST } from "@/app/(main)/manga/_lib/language-map"

export const EXTENSION_TYPE: Record<Extension_Type, string> = {
    "plugin": "Plugin",
    "anime-torrent-provider": "Anime Torrent Provider",
    "manga-provider": "Manga Provider",
    "onlinestream-provider": "Online Streaming Provider",
    "custom-source": "Custom Source",
}

/**
 * Extension types in the order their groups are rendered.
 */
export const EXTENSION_TYPE_ORDER: Extension_Type[] = [
    "plugin",
    "custom-source",
    "anime-torrent-provider",
    "manga-provider",
    "onlinestream-provider",
]

export type ExtensionsByType<T> = Record<Extension_Type, T[]> & {
    /** Extensions whose type is not one of EXTENSION_TYPE_ORDER. */
    other: T[]
}

/**
 * Whether the extension's name, description or ID contains the search term.
 * `term` must already be trimmed and lowercased; an empty term matches everything.
 */
export function matchesExtensionSearch(extension: Pick<Extension_Extension, "name" | "description" | "id"> | undefined, term: string) {
    if (!term) return true
    if (!extension) return false
    return [extension.name, extension.description, extension.id].some(value => value?.toLowerCase().includes(term))
}

export function normalizeSearchTerm(term: string) {
    return term.trim().toLowerCase()
}

/**
 * Splits extensions by type in a single pass, keeping their input order.
 */
export function groupExtensionsByType<T extends { type: Extension_Type }>(extensions: T[]): ExtensionsByType<T> {
    const groups: ExtensionsByType<T> = {
        "plugin": [],
        "custom-source": [],
        "anime-torrent-provider": [],
        "manga-provider": [],
        "onlinestream-provider": [],
        other: [],
    }
    for (const ext of extensions) {
        (groups[ext.type] ?? groups.other).push(ext)
    }
    return groups
}

/**
 * Human-readable language of an extension, e.g. "日本語" for "ja".
 */
export function getExtensionLanguageLabel(lang: string | undefined) {
    return LANGUAGES_LIST[lang?.toLowerCase() ?? ""]?.nativeName || lang?.toUpperCase() || "Unknown"
}
