import { describe, expect, it } from "vitest"
import { fromMangaPreferences, getActiveMangaFilters, mangaEntryFiltersRecordSchema, toMangaPreferences } from "./manga-preferences"

describe("manga preferences", () => {
    it("preserves filters for inactive providers", () => {
        const preferences = toMangaPreferences(
            { "1": "provider-a" },
            {
                "1$provider-a": { scanlators: ["Group A"], language: "en" },
                "1$provider-b": { scanlators: ["Group B"], language: "fr" },
            },
        )

        expect(preferences.entries?.[1].provider).toBe("provider-a")
        expect(preferences.entries?.[1].filters?.["provider-b"]).toEqual({
            scanlators: ["Group B"],
            language: "fr",
        })
    })

    it("hydrates the local provider and filter maps", () => {
        const unpacked = fromMangaPreferences({
            entries: {
                5: {
                    provider: "provider-b",
                    filters: {
                        "provider-b": { scanlators: ["Group"], language: "ja" },
                    },
                },
            },
        })

        expect(unpacked.providers).toEqual({ "5": "provider-b" })
        // sourceProvider is a client-side field with no server counterpart, so hydrating
        // from the server payload always starts it empty.
        expect(unpacked.filters).toEqual({
            "5$provider-b": { scanlators: ["Group"], language: "ja", sourceProvider: "" },
        })
    })

    it("uses filters from the selected provider only", () => {
        const filters = getActiveMangaFilters(
            {
                "1$provider-a": { scanlators: ["Active Group"], language: "en" },
                "1$provider-b": { scanlators: ["Inactive Group"], language: "fr" },
            },
            { "1": "provider-a" },
            [{
                id: "provider-a",
                name: "Provider A",
                lang: "en",
                settings: { supportsMultiScanlator: true, supportsMultiLanguage: true },
            }],
        )

        expect(filters).toEqual({
            "1": { scanlators: ["Active Group"], language: "en", sourceProvider: "" },
        })
    })
})

describe("stored manga filter validation", () => {
    it("backfills sourceProvider for filters stored before the field existed", () => {
        // sourceProvider is a fork addition to a type that is persisted in localStorage, so
        // every user upgrading into it has entries without the field while TypeScript still
        // types it as `string`.
        const parsed = mangaEntryFiltersRecordSchema.parse({
            "1$provider-a": { scanlators: ["Group"], language: "en" },
        })

        expect(parsed).toEqual({
            "1$provider-a": { scanlators: ["Group"], language: "en", sourceProvider: "" },
        })
    })

    it("replaces fields of the wrong type with their defaults", () => {
        const parsed = mangaEntryFiltersRecordSchema.parse({
            "1$provider-a": { scanlators: "not-an-array", language: 7, sourceProvider: null },
        })

        expect(parsed).toEqual({
            "1$provider-a": { scanlators: [], language: "", sourceProvider: "" },
        })
    })

    it("keeps good entries when one entry is unusable", () => {
        const parsed = mangaEntryFiltersRecordSchema.parse({
            "1$provider-a": { scanlators: ["Group"], language: "en", sourceProvider: "src" },
            "2$provider-b": "corrupt",
        })

        expect(parsed["1$provider-a"]).toEqual({ scanlators: ["Group"], language: "en", sourceProvider: "src" })
        expect(parsed["2$provider-b"]).toEqual({ scanlators: [], language: "", sourceProvider: "" })
    })
})
