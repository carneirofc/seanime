import { describe, expect, it } from "vitest"

/**
 * Properties the whole generated schema set must hold.
 *
 * These are cheap to state and would be expensive to discover by hand: they run over every
 * generated schema, so a change to the generator that breaks one type is caught here rather
 * than as a stray console warning months later.
 */
const schemas = await import("@/api/generated/schemas")
const endpoints = await import("@/api/generated/endpoint.schemas")

type AnySchema = {
    safeParse: (value: unknown) => { success: boolean, data?: any }
    _zod?: { def?: { type?: string, shape?: Record<string, AnySchema>, entries?: Record<string, unknown> } }
}

function allSchemas(): Array<[string, AnySchema]> {
    return Object.entries(schemas)
        .filter(([name, value]) => name.endsWith("Schema") && typeof (value as AnySchema)?.safeParse === "function") as Array<[string, AnySchema]>
}

function objectSchemas(): Array<[string, AnySchema]> {
    return allSchemas().filter(([, schema]) => !!schema._zod?.def?.shape)
}

/**
 * Builds the payload a Go server actually sends: null in every position that accepts it -
 * which is what a nil pointer, slice or map marshals to - and a minimal value elsewhere.
 */
function goShapedPayload(schema: AnySchema): unknown {
    const def = schema._zod?.def
    switch (def?.type) {
        case "nullable":
        case "optional":
            return null
        case "string": return ""
        case "number": return 0
        case "boolean": return false
        case "array": return []
        case "record": return {}
        case "enum": return Object.values(def.entries ?? {})[0]
        case "object": {
            const out: Record<string, unknown> = {}
            for (const [key, field] of Object.entries(def.shape ?? {})) out[key] = goShapedPayload(field)
            return out
        }
        default: return null
    }
}

describe("generated schemas", () => {
    it("exports a schema for every generated type", () => {
        expect(allSchemas().length).toBeGreaterThan(300)
    })

    it("parses a Go-shaped payload for every object type", () => {
        // The single most important property in this file. Optional fields are .nullish()
        // rather than .optional() because Go sends null, not an absent key, for more than
        // half of all generated fields. If this regresses, virtually every screen would
        // start logging contract failures.
        const failures: string[] = []

        for (const [name, schema] of objectSchemas()) {
            if (!schema.safeParse(goShapedPayload(schema)).success) failures.push(name)
        }

        expect(failures).toEqual([])
    })

    it("preserves unknown keys rather than stripping them", () => {
        // Objects are loose, so a field a newer server adds - or one the generator failed
        // to capture - survives parsing instead of being silently dropped.
        const stripped: string[] = []

        for (const [name, schema] of objectSchemas().slice(0, 50)) {
            const payload = goShapedPayload(schema) as Record<string, unknown>
            const result = schema.safeParse({ ...payload, aFieldTheGeneratorDoesNotKnow: 1 })
            if (result.success && !("aFieldTheGeneratorDoesNotKnow" in result.data)) stripped.push(name)
        }

        expect(stripped).toEqual([])
    })

    it("exports a values array for every string enum", () => {
        const enumNames = allSchemas()
            .filter(([, schema]) => schema._zod?.def?.type === "enum")
            .map(([name]) => name.replace(/Schema$/, ""))

        expect(enumNames.length).toBeGreaterThan(0)

        for (const name of enumNames) {
            const values = (schemas as Record<string, unknown>)[`${name}Values`]
            expect(Array.isArray(values), `${name}Values should be an array`).toBe(true)
            expect((values as unknown[]).length).toBeGreaterThan(0)
        }
    })

    it("resolves the recursive file tree schema to an arbitrary depth", () => {
        const node = (depth: number): unknown => ({
            name: `n${depth}`, path: `/p${depth}`, normalizedPath: `/p${depth}`, kind: "file",
            children: depth === 0 ? null : [node(depth - 1)],
        })

        const result = schemas.LibraryExplorer_FileTreeNodeJSONSchema.safeParse(node(5))
        expect(result.success).toBe(true)
    })
})

describe("generated endpoint lookups", () => {
    it("maps every endpoint to a usable schema", () => {
        for (const [lookup, entry] of Object.entries(endpoints.staticEndpointSchemas)) {
            expect(typeof (entry.schema as AnySchema).safeParse, lookup).toBe("function")
            expect(entry.key, lookup).toBeTruthy()
        }
        for (const entry of endpoints.patternEndpointSchemas) {
            expect(typeof (entry.schema as AnySchema).safeParse).toBe("function")
            expect(entry.pattern).toBeInstanceOf(RegExp)
        }
    })

    it("keys static endpoints as `METHOD /path`", () => {
        for (const lookup of Object.keys(endpoints.staticEndpointSchemas)) {
            expect(lookup).toMatch(/^(GET|POST|PATCH|PUT|DELETE) \/api\//)
        }
    })

    it("has no static endpoint carrying an unsubstituted placeholder", () => {
        // A path with {id} in it can never be matched by an exact lookup, since the
        // placeholder is replaced before the request is made.
        for (const lookup of Object.keys(endpoints.staticEndpointSchemas)) {
            expect(lookup).not.toContain("{")
        }
    })

    it("anchors every pattern so it cannot match a longer path", () => {
        for (const entry of endpoints.patternEndpointSchemas) {
            expect(entry.pattern.source.startsWith("^")).toBe(true)
            expect(entry.pattern.source.endsWith("$")).toBe(true)
        }
    })
})
