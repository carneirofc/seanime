import * as CryptoJS from "crypto-js"

interface TokenClaims {
    endpoint: string
    iat: number // issued at (unix timestamp)
    exp: number // expires at (unix timestamp)
}

class HMACAuth {
    private secret: string
    private ttl: number

    constructor(secret: string, ttl: number) {
        this.secret = secret
        this.ttl = ttl
    }

    async generateToken(endpoint: string): Promise<string> {
        const now = Math.floor(Date.now() / 1000)
        const claims: TokenClaims = {
            endpoint,
            iat: now,
            exp: now + this.ttl,
        }

        const claimsJSON = JSON.stringify(claims)

        // Encode claims as base64
        const claimsB64 = btoa(claimsJSON)
            .replace(/\+/g, "-")
            .replace(/\//g, "_")
            .replace(/=/g, "")

        // Generate HMAC signature
        const signature = await this.generateHMACSignature(claimsB64)

        // Return token in format: claims.signature
        return `${claimsB64}.${signature}`
    }

    generateQueryParam(endpoint: string, symbol?: string): Promise<string> {
        return this.generateToken(endpoint).then(token => {
            const sym = symbol || "?"
            return `${sym}token=${encodeURIComponent(token)}`
        })
    }

    private async generateHMACSignature(data: string): Promise<string> {
        const signature = CryptoJS.HmacSHA256(data, this.secret)

        const base64 = CryptoJS.enc.Base64.stringify(signature)
        return base64
            .replace(/\+/g, "-")
            .replace(/\//g, "_")
            .replace(/=/g, "")
    }
}

/**
 * How long a client-minted query token claims to be valid for. The server enforces
 * the same bound on validation (core.MediaTokenTTL), so claiming longer here would
 * only produce tokens that stop working early — keep the two in sync.
 */
const MEDIA_TOKEN_TTL_SECONDS = 6 * 60 * 60

// HMAC auth instance using the stored server credential (for media URLs in password
// mode). Callers already establish that a credential exists before minting.
export function createServerPasswordHMACAuth(password: string): HMACAuth {
    return new HMACAuth(password, MEDIA_TOKEN_TTL_SECONDS)
}
