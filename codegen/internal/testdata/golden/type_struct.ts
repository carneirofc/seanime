/**
 * - Filepath: internal/database/models/user.go
 * - Filename: user.go
 * - Package: models
 * @description
 *  User is a person.
 *  Second line.
 */
export type Models_User = {
    id: number
    /**
     * The display name.
     */
    name: string
    nickname?: string
    meta?: Record<string, any>
}

