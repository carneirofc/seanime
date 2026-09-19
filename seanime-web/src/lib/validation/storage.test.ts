import { createStore } from "jotai"
import { beforeEach, describe, expect, it, vi } from "vitest"
import { z } from "zod"
import { atomWithValidatedStorage, readValidatedStorage } from "./storage"

function installStorageStub() {
    const backing = new Map<string, string>()
    const store = {
        getItem: (k: string) => backing.get(k) ?? null,
        setItem: (k: string, v: string) => void backing.set(k, v),
        removeItem: (k: string) => void backing.delete(k),
        clear: () => backing.clear(),
        key: (i: number) => [...backing.keys()][i] ?? null,
        get length() {
            return backing.size
        },
    } as unknown as Storage

    vi.stubGlobal("localStorage", store)
    vi.stubGlobal("sessionStorage", store)
    vi.stubGlobal("window", { localStorage: store, sessionStorage: store })
    return backing
}

const prefsSchema = z.object({
    volume: z.number(),
    mode: z.enum(["elapsed", "remaining"]),
})

type Prefs = z.infer<typeof prefsSchema>

const defaults: Prefs = { volume: 1, mode: "elapsed" }

let backing: Map<string, string>

beforeEach(() => {
    vi.unstubAllGlobals()
    backing = installStorageStub()
})

describe("atomWithValidatedStorage", () => {
    it("returns the stored value when it matches the schema", () => {
        backing.set("prefs", JSON.stringify({ volume: 0.5, mode: "remaining" }))

        const atom = atomWithValidatedStorage("prefs", prefsSchema, defaults, { getOnInit: true })

        expect(createStore().get(atom)).toEqual({ volume: 0.5, mode: "remaining" })
    })

    it("falls back to the default when the key is absent", () => {
        const atom = atomWithValidatedStorage("prefs", prefsSchema, defaults, { getOnInit: true })

        expect(createStore().get(atom)).toEqual(defaults)
    })

    it("falls back when a stored field has the wrong type", () => {
        // The case this wrapper exists for: a value persisted by an older version whose
        // shape has since changed. Plain atomWithStorage would hand `"loud"` to the player.
        backing.set("prefs", JSON.stringify({ volume: "loud", mode: "elapsed" }))

        const atom = atomWithValidatedStorage("prefs", prefsSchema, defaults, { getOnInit: true })

        expect(createStore().get(atom)).toEqual(defaults)
    })

    it("falls back when a stored enum value is no longer valid", () => {
        backing.set("prefs", JSON.stringify({ volume: 1, mode: "countdown" }))

        const atom = atomWithValidatedStorage("prefs", prefsSchema, defaults, { getOnInit: true })

        expect(createStore().get(atom)).toEqual(defaults)
    })

    it("falls back when the stored value is not an object at all", () => {
        backing.set("prefs", JSON.stringify("nonsense"))

        const atom = atomWithValidatedStorage("prefs", prefsSchema, defaults, { getOnInit: true })

        expect(createStore().get(atom)).toEqual(defaults)
    })

    it("still writes through to storage", () => {
        const atom = atomWithValidatedStorage("prefs", prefsSchema, defaults, { getOnInit: true })
        const store = createStore()

        store.set(atom, { volume: 0.2, mode: "remaining" })

        expect(JSON.parse(backing.get("prefs")!)).toEqual({ volume: 0.2, mode: "remaining" })
        expect(store.get(atom)).toEqual({ volume: 0.2, mode: "remaining" })
    })
})

describe("readValidatedStorage", () => {
    it("returns the parsed value when valid", () => {
        backing.set("k", JSON.stringify({ volume: 0.3, mode: "elapsed" }))

        expect(readValidatedStorage("k", prefsSchema, defaults)).toEqual({ volume: 0.3, mode: "elapsed" })
    })

    it("returns the fallback for a missing key", () => {
        expect(readValidatedStorage("k", prefsSchema, defaults)).toEqual(defaults)
    })

    it("returns the fallback for malformed JSON", () => {
        backing.set("k", "}{")

        expect(readValidatedStorage("k", prefsSchema, defaults)).toEqual(defaults)
    })

    it("returns the fallback for a shape mismatch", () => {
        backing.set("k", JSON.stringify({ volume: 1 }))

        expect(readValidatedStorage("k", prefsSchema, defaults)).toEqual(defaults)
    })

    it("returns the fallback when storage access throws", () => {
        // Private windows and some embedded webviews throw on storage access.
        vi.stubGlobal("window", {
            localStorage: {
                getItem: () => {
                    throw new Error("denied")
                },
            },
        })

        expect(readValidatedStorage("k", prefsSchema, defaults)).toEqual(defaults)
    })
})
