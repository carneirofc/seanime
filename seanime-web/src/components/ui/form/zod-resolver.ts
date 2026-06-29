import { zodResolver } from "@hookform/resolvers/zod"
import { FieldValues, get } from "react-hook-form"
import * as z from "zod"

export { zodResolver }

export type Options = {
    min?: number
    max?: number
}

const getType = (field: z.ZodType) => {
    switch ((field as any)._def.type) {
        case "array":
            return "array"
        case "object":
            return "object"
        case "number":
            return "number"
        case "date":
            return "date"
        case "string":
        default:
            return "text"
    }
}

const getArrayOption = (field: any, check: "min_length" | "max_length") => {
    const checks = field?._def?.checks ?? []
    const found = checks.find((c: any) => c?._zod?.def?.check === check)
    return check === "min_length" ? found?._zod?.def?.minimum : found?._zod?.def?.maximum
}

/**
 * A helper function to render forms automatically based on a Zod schema
 *
 * @param schema The Yup schema
 * @returns {FieldProps[]}
 */
export const getFieldsFromSchema = (schema: z.ZodType): FieldValues[] => {
    const fields: FieldValues[] = []

    const def = (schema as any)._def
    let schemaFields: Record<string, any> = {}
    if (def.type === "array") {
        schemaFields = (schema as any).element?.shape ?? {}
    } else if (def.type === "object") {
        schemaFields = (schema as any).shape ?? {}
    } else {
        return fields
    }

    for (const name in schemaFields) {
        const field = schemaFields[name]

        const options: Options = {}
        if (field._def.type === "array") {
            options.min = getArrayOption(field, "min_length")
            options.max = getArrayOption(field, "max_length")
        }

        const meta = field.description && zodParseMeta(field.description)

        fields.push({
            name,
            label: meta?.label || field.description || name,
            type: meta?.type || getType(field),
            ...options,
        })
    }
    return fields
}


export const getNestedSchema = (schema: z.ZodType, path: string) => {
    return get((schema as any).shape, path)
}

export const zodFieldResolver = <T extends z.ZodType>(schema: T) => {
    return {
        getFields() {
            return getFieldsFromSchema(schema)
        },
        getNestedFields(name: string) {
            return getFieldsFromSchema(getNestedSchema(schema, name))
        },
    }
}

export interface ZodMeta {
    label: string
    type?: string
}

export const zodMeta = (meta: ZodMeta) => {
    return JSON.stringify(meta)
}

export const zodParseMeta = (meta: string) => {
    try {
        return JSON.parse(meta)
    }
    catch (e) {
        return meta
    }
}

/**
 * @link https://github.com/colinhacks/zod/discussions/1953#discussioncomment-4811588
 * @param schema
 */
export function getZodDefaults<Schema extends z.ZodObject<any>>(schema: Schema) {
    return Object.fromEntries(
        Object.entries(schema.shape).map(([key, value]) => {
            if (value instanceof z.ZodDefault) {
                const dv = (value as any)._def.defaultValue
                return [key, typeof dv === "function" ? dv() : dv]
            }
            return [key, undefined]
        }),
    )
}

/**
 * @param schema
 */
export function getZodDescriptions<Schema extends z.ZodObject<any>>(schema: Schema) {
    return Object.fromEntries(
        Object.entries(schema.shape).map(([key, value]) => {
            return [key, (value as any).description ?? undefined]
        }),
    )
}

/**
 * @example
 * const meta = useMemo(() => getZodParsedDescription<{ minValue: CalendarDate }>(schema, props.name), [])
 * @param schema
 * @param key
 */
export function getZodParsedDescription<T extends {
    [p: string]: any
}>(schema: z.ZodObject<any>, key: string): T | undefined {
    const obj = getZodDescriptions(schema)
    const parsedDescription: any = (typeof obj[key as keyof typeof obj] === "string" || obj[key as keyof typeof obj] instanceof String) ? JSON.parse(
        obj[key as keyof typeof obj]) : undefined
    if (parsedDescription.constructor == Object) {
        return parsedDescription as T
    }
    return undefined

}
