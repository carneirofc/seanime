/**
 * Stored credentials (media-player and torrent-client passwords, API keys, the
 * Nakama passwords) are withheld from every settings payload the server sends and
 * replaced with this placeholder. Keep it in sync with models.RedactedSecret in
 * internal/database/models/redact.go.
 *
 * Settings forms round-trip the placeholder unchanged, which the server reads as
 * "leave this credential as it is". Anything that consumes a credential as a value
 * rather than echoing it back must check with isRedactedSecret first.
 */
export const REDACTED_SECRET = "__seanime_redacted__"

export function isRedactedSecret(value: string | null | undefined): boolean {
    return value === REDACTED_SECRET
}
