import { reportValidationFailure } from "@/lib/validation/report"

/**
 * Checks server responses against the generated zod schemas.
 *
 * This is a *probe*, not a guard. The schemas in `@/api/generated/schemas` describe what
 * `codegen` believes each endpoint returns; this reports where that belief is wrong, and
 * changes nothing else. Specifically it never transforms the response and never throws:
 * `buildSeaQuery` returns exactly the data it would have returned without this module, so
 * a schema that is itself wrong cannot break a screen that works.
 *
 * That matters because the generator is known to be approximate in places - it renders
 * `[]byte` as `Array<string>`, skips `json:"-"` fields, and flattens embedded structs by
 * hand - so a rejection is at least as likely to mean "the schema is wrong" as "the server
 * is wrong". Until that stops being true, the only safe posture is to report and carry on.
 *
 * Development only. In a production build the dynamic import below is never reached, so the
 * schema module becomes a chunk that is never fetched and the ~140KB of schema code stays
 * out of the shipped bundle.
 */

/** Cached module, so the schemas are parsed once rather than per request. */
let schemaModule: typeof import("@/api/generated/endpoint.schemas") | null = null
let schemaModuleFailed = false

async function loadSchemas() {
    if (schemaModule || schemaModuleFailed) return schemaModule

    try {
        // The comparison is written inline, and the import() sits lexically inside it, so
        // the bundler can fold the branch away in a production build and drop the schema
        // chunk entirely rather than emitting an asset that is never fetched.
        if (import.meta.env.MODE === "development") {
            schemaModule = await import("@/api/generated/endpoint.schemas")
        } else {
            schemaModuleFailed = true
        }
    } catch {
        // A failed import means the probe is unavailable; there is nothing to report.
        schemaModuleFailed = true
    }

    return schemaModule
}

/**
 * Resolves the schema for a request whose path parameters have already been substituted.
 *
 * Exact match first - the overwhelming majority of endpoints are literal paths - then the
 * handful carrying `{placeholders}`, matched by pattern.
 */
export function resolveResponseSchema(
    schemas: typeof import("@/api/generated/endpoint.schemas"),
    method: string,
    endpoint: string,
): { key: string, schema: { safeParse: (value: unknown) => { success: boolean, error?: unknown } } } | undefined {
    const path = stripQuery(endpoint)

    const exact = schemas.staticEndpointSchemas[`${method} ${path}`]
    if (exact) return exact

    for (const candidate of schemas.patternEndpointSchemas) {
        if (candidate.method === method && candidate.pattern.test(path)) {
            return candidate
        }
    }

    return undefined
}

/** Query strings are not part of the route, and `endpoint` may carry one. */
function stripQuery(endpoint: string): string {
    const cut = endpoint.search(/[?#]/)
    return cut === -1 ? endpoint : endpoint.slice(0, cut)
}

/**
 * Validates `data` against the schema for this endpoint and reports any mismatch.
 *
 * Returns nothing and swallows every error by design - callers use it as
 * `void probeResponseContract(...)` and must not await it.
 */
export async function probeResponseContract(method: string, endpoint: string, data: unknown): Promise<void> {
    if (import.meta.env.MODE !== "development") return
    // A response that carried no data has nothing to check; the request either failed or
    // the endpoint returns nothing.
    if (data === undefined) return

    try {
        const schemas = await loadSchemas()
        if (!schemas) return

        const resolved = resolveResponseSchema(schemas, method, endpoint)
        if (!resolved) return

        const result = resolved.schema.safeParse(data)
        if (!result.success) {
            reportValidationFailure(`api:${resolved.key}`, result.error as never)
        }
    } catch {
        // The probe must never affect the request it is observing.
    }
}
