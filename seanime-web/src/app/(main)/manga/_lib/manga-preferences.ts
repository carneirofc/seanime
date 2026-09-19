import { ExtensionRepo_MangaProviderExtensionItem, Manga_MangaPreferences } from "@/api/generated/types"
import { z } from "zod"

export type MangaEntryFilters = {
    scanlators: string[]
    language: string
    // Fork addition: narrows chapters to a single upstream source when a provider aggregates
    // several. Client-side only — the server preference payload has no matching field.
    sourceProvider: string
}

const emptyMangaEntryFilters: MangaEntryFilters = { scanlators: [], language: "", sourceProvider: "" }

/**
 * Schema for one persisted filter entry.
 *
 * These are stored in localStorage, so they outlive the type that produced them.
 * `sourceProvider` is a fork addition: every user who upgrades into it has stored entries
 * without the field, which TypeScript still types as `string`. Defaulting each field keeps
 * those entries usable instead of letting `undefined` reach code that assumes a string.
 */
export const mangaEntryFiltersSchema = z.object({
    scanlators: z.array(z.string()).catch([]),
    language: z.string().catch(""),
    sourceProvider: z.string().catch(""),
})

/**
 * Schema for the whole `mediaId$providerId` -> filters map. An entry that is not an object
 * at all collapses to empty filters rather than discarding every other entry with it.
 */
export const mangaEntryFiltersRecordSchema = z.record(
    z.string(),
    mangaEntryFiltersSchema.catch(() => ({ ...emptyMangaEntryFilters })),
)

export function toMangaPreferences(providers: Record<string, string>, filters: Record<string, MangaEntryFilters>): Manga_MangaPreferences {
    const entries: NonNullable<Manga_MangaPreferences["entries"]> = {}

    for (const [mediaId, provider] of Object.entries(providers)) {
        entries[Number(mediaId)] = { provider, filters: {} }
    }

    for (const [key, filter] of Object.entries(filters)) {
        const separator = key.indexOf("$")
        if (separator <= 0) continue
        const mediaId = Number(key.slice(0, separator))
        const provider = key.slice(separator + 1)
        if (!Number.isInteger(mediaId) || mediaId <= 0 || !provider) continue

        const entry = entries[mediaId] ?? { provider: "", filters: {} }
        entry.filters ??= {}
        entry.filters[provider] = {
            scanlators: filter.scanlators ?? [],
            language: filter.language ?? "",
        }
        entries[mediaId] = entry
    }

    return { entries }
}

export function fromMangaPreferences(preferences: Manga_MangaPreferences) {
    const providers: Record<string, string> = {}
    const filters: Record<string, MangaEntryFilters> = {}

    for (const [mediaId, entry] of Object.entries(preferences.entries ?? {})) {
        if (entry.provider) {
            providers[mediaId] = entry.provider
        }
        for (const [provider, filter] of Object.entries(entry.filters ?? {})) {
            filters[`${mediaId}$${provider}`] = {
                scanlators: filter.scanlators ?? [],
                language: filter.language ?? "",
                sourceProvider: "",
            }
        }
    }

    return { providers, filters }
}

export function getActiveMangaFilters(
    storedFilters: Record<string, MangaEntryFilters>,
    selectedProviders: Record<string, string>,
    extensions: ExtensionRepo_MangaProviderExtensionItem[] | undefined,
) {
    const filters: Record<string, MangaEntryFilters> = {}

    for (const [key, value] of Object.entries(storedFilters)) {
        const [mediaId, providerId] = key.split("$")
        const selectedProvider = selectedProviders[mediaId]
        if (!selectedProvider || providerId !== selectedProvider) continue

        const extension = extensions?.find(extension => extension.id === selectedProvider)
        if (!extension?.settings?.supportsMultiScanlator && !extension?.settings?.supportsMultiLanguage) continue

        filters[mediaId] = {
            scanlators: value.scanlators ?? [],
            language: value.language ?? "",
            sourceProvider: value.sourceProvider ?? "",
        }
    }

    return filters
}
