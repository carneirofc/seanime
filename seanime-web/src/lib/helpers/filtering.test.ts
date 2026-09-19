import type { AL_AnimeCollection_MediaListCollection_Lists_Entries } from "@/api/generated/types"
import { describe, expect, it } from "vitest"
import { DEFAULT_ANIME_COLLECTION_PARAMS, filterListEntries } from "./filtering"

type Entry = AL_AnimeCollection_MediaListCollection_Lists_Entries

function entry(id: number, title: string): Entry {
    return {
        id,
        media: {
            id,
            title: { userPreferred: title },
        },
    } as Entry
}

const ENTRIES: Entry[] = [entry(1, "Alpha"), entry(2, "Beta"), entry(3, "Gamma")]

function filterByTags(tags: string[] | null, mediaTagMap?: Record<number, string[]> | null) {
    return filterListEntries(
        "anime",
        ENTRIES,
        { ...DEFAULT_ANIME_COLLECTION_PARAMS, tags },
        true,
        false,
        mediaTagMap,
    ).map(n => n.media!.id)
}

describe("filterListEntries tag filtering", () => {
    const FULL_MAP = {
        1: ["Isekai", "Comedy"],
        2: ["Isekai"],
        3: ["Drama"],
    }

    it("keeps only entries carrying the tag", () => {
        expect(filterByTags(["Isekai"], FULL_MAP).sort()).toEqual([1, 2])
    })

    it("requires every selected tag, not any of them", () => {
        expect(filterByTags(["Isekai", "Comedy"], FULL_MAP)).toEqual([1])
    })

    it("does not filter when no tag is selected", () => {
        expect(filterByTags(null, FULL_MAP).sort()).toEqual([1, 2, 3])
        expect(filterByTags([], FULL_MAP).sort()).toEqual([1, 2, 3])
    })

    // The server now builds the tag map incrementally, so it can legitimately be missing
    // an entry that was only just added to the collection. A media absent from the map
    // must be treated as having no tags — excluded from a tag filter, never crashing and
    // never silently matching.
    it("excludes media the tag map does not know about", () => {
        const partialMap = { 1: ["Isekai"] }
        expect(filterByTags(["Isekai"], partialMap)).toEqual([1])
    })

    it("excludes media recorded as having no tags", () => {
        const mapWithEmpty = { 1: ["Isekai"], 2: [] }
        expect(filterByTags(["Isekai"], mapWithEmpty)).toEqual([1])
    })

    // Before the map has loaded at all the filter is inert rather than empty, so the
    // list does not flash empty on first paint.
    it("skips tag filtering entirely when the map is absent", () => {
        expect(filterByTags(["Isekai"], null).sort()).toEqual([1, 2, 3])
        expect(filterByTags(["Isekai"], undefined).sort()).toEqual([1, 2, 3])
    })
})
