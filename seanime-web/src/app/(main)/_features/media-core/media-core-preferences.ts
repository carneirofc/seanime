import { atomWithStorage } from "jotai/utils"
import { z } from "zod"

export interface MediaCorePreferences {
    version: number
    autoPlay: boolean
    autoNext: boolean
    volume: number
    muted: boolean
    playbackRate: number
    autoSkip: boolean
    skipPatterns: string
    showStats: boolean
    chapterMarkers: boolean
    timestampMode: "elapsed" | "remaining"
}

export const mediaCoreDefaultPreferences: MediaCorePreferences = {
    version: 2,
    autoPlay: true,
    autoNext: true,
    volume: 1.0,
    muted: false,
    playbackRate: 1.0,
    autoSkip: false,
    skipPatterns: "",
    showStats: false,
    chapterMarkers: true,
    timestampMode: "elapsed",
}

const PREFERENCES_KEY = "sea-media-core-preferences"

/**
 * Schema for a stored preferences blob.
 *
 * Every field falls back to its default individually rather than rejecting the whole
 * object, which preserves the previous merge-over-defaults behaviour: a preferences file
 * missing a field written by a newer version still loads. The difference is that a field
 * present with the *wrong type* - `volume: "loud"`, a `timestampMode` that is no longer a
 * valid option - now falls back too, instead of being spread through a `Partial` cast
 * straight into player state.
 *
 * `version` is deliberately NOT defaulted: an unrecognised version means "not a preferences
 * blob we understand", which sends the caller down the legacy-migration path below.
 */
const storedPreferencesSchema = z.object({
    version: z.union([z.literal(1), z.literal(2)]),
    autoPlay: z.boolean().catch(mediaCoreDefaultPreferences.autoPlay),
    autoNext: z.boolean().catch(mediaCoreDefaultPreferences.autoNext),
    volume: z.number().catch(mediaCoreDefaultPreferences.volume),
    muted: z.boolean().catch(mediaCoreDefaultPreferences.muted),
    playbackRate: z.number().catch(mediaCoreDefaultPreferences.playbackRate),
    autoSkip: z.boolean().catch(mediaCoreDefaultPreferences.autoSkip),
    skipPatterns: z.string().catch(mediaCoreDefaultPreferences.skipPatterns),
    showStats: z.boolean().catch(mediaCoreDefaultPreferences.showStats),
    chapterMarkers: z.boolean().catch(mediaCoreDefaultPreferences.chapterMarkers),
    timestampMode: z.enum(["elapsed", "remaining"]).catch(mediaCoreDefaultPreferences.timestampMode),
})

function parsePreferences(value: unknown): MediaCorePreferences | null {
    const result = storedPreferencesSchema.safeParse(value)
    if (!result.success) return null

    // Reading always normalises to the current version.
    return { ...result.data, version: 2 }
}

const customStorage = {
    getItem(key: string, initialValue: MediaCorePreferences): MediaCorePreferences {
        try {
            const raw = localStorage.getItem(key)
            if (raw) {
                const parsed: unknown = JSON.parse(raw)
                const preferences = parsePreferences(parsed)
                if (preferences) {
                    // Persist whenever validation changed anything - a version bump, a
                    // dropped field, or a value that failed its schema - so the repair
                    // happens once rather than on every read.
                    if (JSON.stringify(preferences) !== JSON.stringify(parsed)) {
                        localStorage.setItem(key, JSON.stringify(preferences))
                    }
                    return preferences
                }
            }
        } catch (e) {
            console.error("Failed to parse sea-media-core-preferences", e)
        }

        // Migration precedence: existing unified value -> VideoCore legacy value -> MpvCore legacy value -> default.
        const getLegacyValue = <T>(vcKey: string, mcKey: string, fallback: T): T => {
            try {
                const vcRaw = localStorage.getItem(vcKey)
                if (vcRaw !== null) return JSON.parse(vcRaw) as T
                const mcRaw = localStorage.getItem(mcKey)
                if (mcRaw !== null) return JSON.parse(mcRaw) as T
            } catch (e) {
                console.error(`Failed to migrate legacy keys ${vcKey} / ${mcKey}`, e)
            }
            return fallback
        }

        const migrated: MediaCorePreferences = {
            version: 2,
            autoPlay: getLegacyValue("sea-video-core-auto-play", "sea-mpv-core-auto-play", mediaCoreDefaultPreferences.autoPlay),
            autoNext: getLegacyValue("sea-video-core-auto-next", "sea-mpv-core-auto-next", mediaCoreDefaultPreferences.autoNext),
            volume: getLegacyValue("sea-video-core-volume", "sea-mpv-core-volume", mediaCoreDefaultPreferences.volume),
            muted: getLegacyValue("sea-video-core-muted", "sea-mpv-core-muted", mediaCoreDefaultPreferences.muted),
            playbackRate: getLegacyValue("sea-video-core-playback-rate", "sea-mpv-core-playback-rate", mediaCoreDefaultPreferences.playbackRate),
            autoSkip: getLegacyValue("sea-video-core-auto-skip-op-ed", "sea-mpv-core-auto-skip", mediaCoreDefaultPreferences.autoSkip),
            skipPatterns: mediaCoreDefaultPreferences.skipPatterns,
            showStats: getLegacyValue("sea-video-core-show-stats-for-nerds", "sea-mpv-core-show-stats", mediaCoreDefaultPreferences.showStats),
            chapterMarkers: getLegacyValue("sea-video-core-chapter-markers", "sea-mpv-core-chapter-markers", mediaCoreDefaultPreferences.chapterMarkers),
            timestampMode: getLegacyValue<"elapsed" | "remaining">("sea-video-core-timestamp-type", "dummy-nonexistent-key", mediaCoreDefaultPreferences.timestampMode),
        }

        try {
            localStorage.setItem(key, JSON.stringify(migrated))
        } catch (e) {
            console.error("Failed to write migrated sea-media-core-preferences", e)
        }

        return migrated
    },
    setItem(key: string, value: MediaCorePreferences) {
        localStorage.setItem(key, JSON.stringify(value))
    },
    removeItem(key: string) {
        localStorage.removeItem(key)
    },
    subscribe(key: string, callback: (value: MediaCorePreferences) => void, initialValue: MediaCorePreferences) {
        const handleStorage = (e: StorageEvent) => {
            if (e.key === key) {
                if (e.newValue === null) {
                    callback(initialValue)
                } else {
                    try {
                        callback(parsePreferences(JSON.parse(e.newValue)) ?? initialValue)
                    } catch {
                        callback(initialValue)
                    }
                }
            }
        }
        window.addEventListener("storage", handleStorage)
        return () => window.removeEventListener("storage", handleStorage)
    }
}

export const mediaCorePreferencesAtom = atomWithStorage<MediaCorePreferences>(
    PREFERENCES_KEY,
    mediaCoreDefaultPreferences,
    customStorage,
    { getOnInit: true }
)
