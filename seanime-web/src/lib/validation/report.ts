import { logger } from "@/lib/helpers/debug"
import type { ZodError } from "zod"

/**
 * Shared reporting for runtime validation failures.
 *
 * Everything in `src/lib/validation` guards data the app does not control - values persisted
 * by an older Seanime version, websocket frames, third-party plugin payloads. A schema
 * rejection means "ignore this value and carry on", never "break the screen": callers fall
 * back to a default or drop the frame, and the failure is only reported here.
 *
 * Reporting is dev-only and deduplicated by `scope`, because these failures are usually
 * either per-frame (a bad websocket event repeats many times a second) or per-render.
 */
const reported = new Set<string>()

export function reportValidationFailure(scope: string, error: ZodError | string): void {
    if (import.meta.env.MODE !== "development") return
    if (reported.has(scope)) return

    reported.add(scope)

    const detail = typeof error === "string" ? error : error.issues.map(i => `${i.path.join(".") || "<root>"}: ${i.message}`).join("; ")

    logger("Validation").warning(`Rejected untrusted value (${scope}): ${detail}`)
}

/** Test-only: clears the dedupe set so each test observes reporting independently. */
export function __resetValidationReporting(): void {
    reported.clear()
}
