import { z } from "zod"

/**
 * Payload of the `CLIENT_IDENTITY` websocket frame.
 *
 * The server uses this to hand the client its id and a proof token, which are then sent
 * back on every request as `X-Seanime-Client-Id` / `X-Seanime-Client-Id-Proof`. The values
 * are only ever echoed back to the server, but they are still attacker-influenced input to
 * `setClientIdentity`, so they are typed rather than trusted.
 *
 * `proof` is optional: the server omits it for clients that do not need one, and the
 * identity is still usable without it.
 */
export const clientIdentityPayloadSchema = z.object({
    clientId: z.string().trim().min(1),
    proof: z.string().trim().default(""),
})

export type ClientIdentityPayload = z.infer<typeof clientIdentityPayloadSchema>
