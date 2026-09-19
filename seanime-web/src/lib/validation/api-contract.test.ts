import { probeResponseContract, resolveResponseSchema } from "@/lib/validation/api-contract"
import { __resetValidationReporting } from "@/lib/validation/report"
import { beforeEach, describe, expect, it, vi } from "vitest"

/**
 * These run against the REAL generated schemas rather than fixtures: the point of the
 * probe is to check the generated contract, so a test that mocked it would prove nothing.
 */
const schemas = await import("@/api/generated/endpoint.schemas")

beforeEach(() => {
    __resetValidationReporting()
    vi.unstubAllEnvs()
})

describe("resolveResponseSchema", () => {
    it("finds an endpoint with no path parameters by exact match", () => {
        const found = resolveResponseSchema(schemas, "GET", "/api/v1/status")
        expect(found?.key).toBe("STATUS-get-status")
    })

    it("does not match the same path under a different method", () => {
        expect(resolveResponseSchema(schemas, "DELETE", "/api/v1/status")).toBeUndefined()
    })

    it("returns undefined for an unknown path", () => {
        expect(resolveResponseSchema(schemas, "GET", "/api/v1/not-a-real-endpoint")).toBeUndefined()
    })

    it("ignores a query string, which is not part of the route", () => {
        expect(resolveResponseSchema(schemas, "GET", "/api/v1/status?x=1")?.key).toBe("STATUS-get-status")
        expect(resolveResponseSchema(schemas, "GET", "/api/v1/status#frag")?.key).toBe("STATUS-get-status")
    })

    it("matches an endpoint whose path parameters have been substituted", () => {
        // Hooks call .replace("{id}", ...) before the request, so the lookup only ever
        // sees a concrete path.
        const parameterized = schemas.patternEndpointSchemas[0]
        expect(parameterized).toBeDefined()

        const concrete = parameterized.pattern.source
            .replace(/^\^/, "").replace(/\$$/, "")
            .replace(/\\\//g, "/")
            .replace(/\[\^\/\]\+/g, "12345")

        expect(resolveResponseSchema(schemas, parameterized.method, concrete)?.key).toBe(parameterized.key)
    })

    it("does not let a path parameter span a slash", () => {
        // /a/{id} must not match /a/b/c, or one route would be mistaken for another.
        const withParam = schemas.patternEndpointSchemas.find(p => p.pattern.source.includes("[^\\/]+"))
        expect(withParam).toBeDefined()

        const tooManySegments = withParam!.pattern.source
            .replace(/^\^/, "").replace(/\$$/, "")
            .replace(/\\\//g, "/")
            .replace(/\[\^\/\]\+/g, "a/b")

        expect(withParam!.pattern.test(tooManySegments)).toBe(false)
    })
})

describe("probeResponseContract", () => {
    it("does nothing outside development", async () => {
        vi.stubEnv("MODE", "production")
        const warn = vi.spyOn(console, "log").mockImplementation(() => {})

        await probeResponseContract("GET", "/api/v1/status", { nonsense: true })

        expect(warn).not.toHaveBeenCalled()
        warn.mockRestore()
    })

    it("reports a response that does not match the schema", async () => {
        vi.stubEnv("MODE", "development")
        const warn = vi.spyOn(console, "log").mockImplementation(() => {})

        // `Status` requires several fields; an empty object cannot satisfy it.
        await probeResponseContract("GET", "/api/v1/status", {})

        expect(warn).toHaveBeenCalled()
        expect(String(warn.mock.calls[0]?.join(" "))).toContain("STATUS-get-status")
        warn.mockRestore()
    })

    it("reports each endpoint only once, since a bad response usually repeats", async () => {
        vi.stubEnv("MODE", "development")
        const warn = vi.spyOn(console, "log").mockImplementation(() => {})

        await probeResponseContract("GET", "/api/v1/status", {})
        await probeResponseContract("GET", "/api/v1/status", {})
        await probeResponseContract("GET", "/api/v1/status", {})

        expect(warn).toHaveBeenCalledTimes(1)
        warn.mockRestore()
    })

    it("stays silent for an endpoint it has no schema for", async () => {
        vi.stubEnv("MODE", "development")
        const warn = vi.spyOn(console, "log").mockImplementation(() => {})

        await probeResponseContract("GET", "/api/v1/not-a-real-endpoint", { anything: 1 })

        expect(warn).not.toHaveBeenCalled()
        warn.mockRestore()
    })

    it("stays silent when the request returned no data", async () => {
        vi.stubEnv("MODE", "development")
        const warn = vi.spyOn(console, "log").mockImplementation(() => {})

        await probeResponseContract("GET", "/api/v1/status", undefined)

        expect(warn).not.toHaveBeenCalled()
        warn.mockRestore()
    })

    it("never throws, whatever it is handed", async () => {
        vi.stubEnv("MODE", "development")
        vi.spyOn(console, "log").mockImplementation(() => {})

        // The probe observes the response; it must not be able to break the request.
        for (const value of [null, 0, "", [], NaN, Symbol("x"), () => {}, { a: { b: { c: 1 } } }]) {
            await expect(probeResponseContract("GET", "/api/v1/status", value)).resolves.toBeUndefined()
        }
        vi.restoreAllMocks()
    })
})

describe("the generated schemas themselves", () => {
    it("accept null wherever a field is optional", () => {
        // This is the whole reason optional fields are .nullish(): Go marshals a nil
        // pointer, slice or map as null, not as an absent key.
        const schema = schemas.staticEndpointSchemas["GET /api/v1/status"]!.schema as any
        const shape = schema.shape as Record<string, any>

        const optionalKeys = Object.keys(shape).filter(k => shape[k].safeParse(null).success)
        expect(optionalKeys.length).toBeGreaterThan(0)

        for (const key of optionalKeys) {
            expect(shape[key].safeParse(null).success).toBe(true)
            expect(shape[key].safeParse(undefined).success).toBe(true)
        }
    })

    it("preserve fields the generator does not know about", () => {
        // Objects are loose, so a newer server adding a field does not have it stripped
        // out from under the app.
        const schema = schemas.staticEndpointSchemas["GET /api/v1/status"]!.schema as any
        const minimal: Record<string, unknown> = {}
        for (const [key, field] of Object.entries(schema.shape as Record<string, any>)) {
            minimal[key] = field.safeParse(null).success ? null : sampleFor(field)
        }

        const result = schema.safeParse({ ...minimal, fieldFromANewerServer: 42 })
        expect(result.success).toBe(true)
        expect(result.data).toHaveProperty("fieldFromANewerServer", 42)
    })
})

/** Smallest value satisfying a non-nullable field, for building a minimal payload. */
function sampleFor(field: any): unknown {
    const type = field?._zod?.def?.type
    switch (type) {
        case "string": return ""
        case "number": return 0
        case "boolean": return false
        case "array": return []
        case "record": return {}
        case "enum": return Object.values(field._zod.def.entries ?? {})[0]
        case "object": {
            const out: Record<string, unknown> = {}
            for (const [k, v] of Object.entries(field._zod.def.shape ?? {})) out[k] = sampleFor(v)
            return out
        }
        default: return null
    }
}
