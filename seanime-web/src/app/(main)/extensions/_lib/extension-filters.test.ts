import { describe, expect, it } from "vitest"
import { getExtensionLanguageLabel, groupExtensionsByType, matchesExtensionSearch, normalizeSearchTerm } from "./extension-filters"

describe("matchesExtensionSearch", () => {
    const ext = { id: "animepahe", name: "AnimePahe", description: "Online streaming source" }

    it("matches everything for an empty term", () => {
        expect(matchesExtensionSearch(ext, "")).toBe(true)
    })

    it("matches name, description and id case-insensitively", () => {
        expect(matchesExtensionSearch(ext, normalizeSearchTerm("  PAHE "))).toBe(true)
        expect(matchesExtensionSearch(ext, "streaming")).toBe(true)
        expect(matchesExtensionSearch(ext, "animepahe")).toBe(true)
        expect(matchesExtensionSearch(ext, "torrent")).toBe(false)
    })

    it("does not match a missing extension", () => {
        expect(matchesExtensionSearch(undefined, "x")).toBe(false)
    })
})

describe("groupExtensionsByType", () => {
    it("groups in one pass and keeps order", () => {
        const groups = groupExtensionsByType([
            { id: "a", type: "manga-provider" as const },
            { id: "b", type: "plugin" as const },
            { id: "c", type: "manga-provider" as const },
            { id: "d", type: "unknown" as any },
        ])
        expect(groups["manga-provider"].map(e => e.id)).toEqual(["a", "c"])
        expect(groups.plugin.map(e => e.id)).toEqual(["b"])
        expect(groups["onlinestream-provider"]).toEqual([])
        expect(groups.other.map(e => e.id)).toEqual(["d"])
    })
})

describe("getExtensionLanguageLabel", () => {
    it("falls back to the uppercased code, then Unknown", () => {
        expect(getExtensionLanguageLabel("zz")).toBe("ZZ")
        expect(getExtensionLanguageLabel(undefined)).toBe("Unknown")
        expect(getExtensionLanguageLabel("en")).not.toBe("EN")
    })
})
