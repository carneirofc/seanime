import { reportValidationFailure } from "@/lib/validation/report"
import { z } from "zod"

/**
 * Schemas for the websocket boundary.
 *
 * Only the *envelope* is validated - `type`, and `extensionId` for plugin frames. Payloads
 * stay `unknown` here and are narrowed by the listener that asked for that event type:
 * validating every payload against the generated types would mean generating ~1000 zod
 * schemas for a contract the server already owns, at real bundle cost, to guard types that
 * are mostly optional anyway.
 *
 * The envelope is where the cheap wins are. A frame missing `type` used to flow into
 * `parsed.type === type` comparisons and dispatch handlers with `undefined` payloads.
 */

export const seaWebsocketEnvelopeSchema = z.object({
    type: z.string().min(1),
    payload: z.unknown().optional(),
})

export type SeaWebsocketEnvelope = z.infer<typeof seaWebsocketEnvelopeSchema>

/**
 * Plugin frames arrive nested: the outer envelope has `type: "plugin"` and its payload is
 * itself an envelope carrying `extensionId`.
 *
 * `extensionId` is defaulted rather than required because the existing listeners treat an
 * empty id as "any plugin", and a frame without one is still dispatchable.
 */
export const seaWebsocketPluginEnvelopeSchema = z.object({
    type: z.string().min(1),
    extensionId: z.string().default(""),
    payload: z.unknown().optional(),
})

export type SeaWebsocketPluginEnvelope = z.infer<typeof seaWebsocketPluginEnvelopeSchema>

/**
 * Payload of a `plugin:batch-events` frame.
 *
 * These originate from user-installed third-party plugins, the weakest trust assumption in
 * the app. A malformed batch previously reached `batchPayload.events || []` through an
 * `as any` cast, so a non-array `events` (or entries without a `type`) would throw inside
 * the listener loop and take out every other listener on the same frame.
 *
 * Invalid entries are dropped individually so one bad event cannot discard a good batch.
 */
export const pluginBatchEventsPayloadSchema = z.object({
    events: z.array(
        z.object({
            type: z.string().min(1),
            extensionId: z.string().default(""),
            payload: z.unknown().optional(),
        }).catch(() => ({ type: "", extensionId: "", payload: undefined })),
    ).default([]),
})

export type PluginBatchEvent = {
    type: string
    extensionId: string
    // Optional: an event carrying no body is legitimate, and zod only supplies the key when
    // the sender did.
    payload?: unknown
}

/**
 * Parse a raw websocket frame into a validated envelope.
 *
 * Returns `null` - never throws - when the frame is not JSON or not a well-formed envelope,
 * so callers can simply drop it.
 */
export function parseWebsocketFrame(raw: unknown, scope: string): SeaWebsocketEnvelope | null {
    if (typeof raw !== "string") {
        reportValidationFailure(`websocket:${scope}`, "frame data was not a string")
        return null
    }

    let json: unknown
    try {
        json = JSON.parse(raw)
    }
    catch {
        reportValidationFailure(`websocket:${scope}`, "frame was not valid JSON")
        return null
    }

    const result = seaWebsocketEnvelopeSchema.safeParse(json)
    if (!result.success) {
        reportValidationFailure(`websocket:${scope}`, result.error)
        return null
    }

    return result.data
}

/** Narrow an outer `"plugin"` frame's payload to a plugin envelope, or `null` if malformed. */
export function parsePluginEnvelope(payload: unknown, scope: string): SeaWebsocketPluginEnvelope | null {
    const result = seaWebsocketPluginEnvelopeSchema.safeParse(payload)
    if (!result.success) {
        reportValidationFailure(`websocket:${scope}`, result.error)
        return null
    }

    return result.data
}

/**
 * Extract the events of a `plugin:batch-events` frame.
 *
 * Returns `[]` for anything malformed, and silently skips entries that failed to parse.
 */
export function parsePluginBatchEvents(payload: unknown, scope: string): PluginBatchEvent[] {
    const result = pluginBatchEventsPayloadSchema.safeParse(payload)
    if (!result.success) {
        reportValidationFailure(`websocket:${scope}`, result.error)
        return []
    }

    return result.data.events.filter(event => event.type !== "")
}
