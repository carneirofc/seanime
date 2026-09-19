package codegen

import (
	"fmt"
	"strings"
)

// This file converts the TypeScript type expressions the generator already
// computes (GoStructField.TypescriptType) into zod schema expressions.
//
// Working from the rendered TypeScript rather than from the Go AST a second time
// is deliberate: it guarantees schemas.ts describes exactly what types.ts
// declares, which is what makes the generated compile-time assertions between
// them meaningful.

// tsTypeToZod converts a TypeScript type expression to a zod schema expression.
//
// The grammar is small and closed — the generator only ever emits primitives,
// Array<>, Record<,>, anonymous object literals, and references to other
// generated types:
//
//	string                        => z.string()
//	Array<Models_User>            => z.array(Models_UserSchema)
//	Record<string, number>        => z.record(z.string(), z.number())
//	{ a: string; b: number; }     => z.looseObject({ "a": z.string(), "b": z.number() })
//	Models_User                   => Models_UserSchema
//
// An unrecognized expression falls back to z.unknown() rather than failing the
// run: an unvalidated field is a gap, but a generator that refuses to run over an
// unfamiliar type would block a backend change outright.
func tsTypeToZod(tsType string) string {
	return tsTypeToZodRef(tsType, "")
}

// tsTypeToZodRef is tsTypeToZod with a module qualifier applied to references to
// other generated schemas, e.g. qualifier "s" renders Models_User as
// s.Models_UserSchema. Used by endpoint.schemas.ts, which imports schemas.ts as a
// namespace rather than listing several hundred named imports.
func tsTypeToZodRef(tsType string, qualifier string) string {
	// types.ts and endpoint.types.ts substitute this at emit time (Go's
	// json.RawMessage arrives as an opaque object), so mirror it here or
	// "RawMessage" would be mistaken for a generated type name.
	t := strings.TrimSpace(strings.ReplaceAll(tsType, "RawMessage", "Record<string, any>"))
	if t == "" {
		return zodUnknown
	}

	if schema, ok := zodScalars[t]; ok {
		return schema
	}

	if inner, ok := genericArg(t, "Array"); ok {
		return fmt.Sprintf("z.array(%s)", tsTypeToZodRef(inner, qualifier))
	}

	if args, ok := genericArgs(t, "Record"); ok && len(args) == 2 {
		return fmt.Sprintf("z.record(%s, %s)", zodRecordKey(args[0]), tsTypeToZodRef(args[1], qualifier))
	}

	if strings.HasPrefix(t, "{") && strings.HasSuffix(t, "}") {
		return inlineObjectToZod(t, qualifier)
	}

	if isGeneratedTypeName(t) {
		return qualify(qualifier, t+zodSchemaSuffix)
	}

	return zodUnknown
}

// zodRecordKey maps a Record's key type.
//
// Go map keys other than string are still JSON object keys, which are always
// strings on the wire: `map[int]T` marshals to {"1": ...}. So a numeric key type
// has to coerce, or every real payload would be rejected.
func zodRecordKey(keyType string) string {
	switch strings.TrimSpace(keyType) {
	case "number":
		return "z.coerce.number()"
	case "string":
		return "z.string()"
	default:
		return "z.string()"
	}
}

// inlineObjectToZod converts an anonymous object literal, e.g.
// "{ image_url: string; small_image_url: string; }".
func inlineObjectToZod(t string, qualifier string) string {
	body := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(t, "{"), "}"))
	if body == "" {
		return "z.looseObject({})"
	}

	var fields []string
	for _, member := range splitTopLevel(body, ';') {
		member = strings.TrimSpace(member)
		if member == "" {
			continue
		}
		name, valueType, found := strings.Cut(member, ":")
		if !found {
			continue
		}
		name = strings.TrimSpace(name)
		optional := strings.HasSuffix(name, "?")
		name = strings.TrimSuffix(name, "?")

		schema := tsTypeToZodRef(valueType, qualifier)
		if optional {
			schema = schema + zodNullishSuffix
		}
		fields = append(fields, fmt.Sprintf("%s: %s", quoteJSKey(name), schema))
	}

	if len(fields) == 0 {
		return "z.looseObject({})"
	}
	return "z.looseObject({ " + strings.Join(fields, ", ") + " })"
}

// genericArg returns the single type argument of a one-parameter generic, e.g.
// genericArg("Array<string>", "Array") => "string".
func genericArg(t, name string) (string, bool) {
	args, ok := genericArgs(t, name)
	if !ok || len(args) != 1 {
		return "", false
	}
	return args[0], true
}

// genericArgs returns the comma-separated type arguments of a generic.
func genericArgs(t, name string) ([]string, bool) {
	prefix := name + "<"
	if !strings.HasPrefix(t, prefix) || !strings.HasSuffix(t, ">") {
		return nil, false
	}
	inner := t[len(prefix) : len(t)-1]
	return splitTopLevel(inner, ','), true
}

// splitTopLevel splits on sep, ignoring separators nested inside <> or {}.
func splitTopLevel(s string, sep rune) []string {
	parts := make([]string, 0, 2)
	depth := 0
	current := strings.Builder{}

	for _, r := range s {
		switch r {
		case '<', '{', '(':
			depth++
		case '>', '}', ')':
			depth--
		case sep:
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(current.String()))
				current.Reset()
				continue
			}
		}
		current.WriteRune(r)
	}
	if last := strings.TrimSpace(current.String()); last != "" {
		parts = append(parts, last)
	}
	return parts
}

// isGeneratedTypeName reports whether t looks like a reference to another
// generated type, i.e. a bare identifier starting with an upper-case letter.
func isGeneratedTypeName(t string) bool {
	if t == "" {
		return false
	}
	if c := t[0]; c < 'A' || c > 'Z' {
		return false
	}
	for _, r := range t {
		isWord := r == '_' ||
			(r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9')
		if !isWord {
			return false
		}
	}
	return true
}

// quoteJSKey quotes an object key unless it is a safe bare identifier. Go json
// tags allow characters a bare JS key cannot carry, e.g. "content-type".
func quoteJSKey(name string) string {
	if name != "" && isSafeJSIdentifier(name) {
		return name
	}
	return `"` + strings.ReplaceAll(name, `"`, `\"`) + `"`
}

// jsStringLiteral renders s as a quoted JavaScript string. Unlike quoteJSKey this
// always quotes: it is for value position, where a bare identifier would be read as
// a variable reference.
func jsStringLiteral(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(s) + `"`
}

// referencedSchemaNames returns the schema identifiers a rendered zod expression
// refers to, e.g. "z.array(Models_UserSchema).nullish()" => ["Models_UserSchema"].
//
// Emission order is computed from these rather than from the Go type each field
// points at, because two Go packages can share a type-name prefix -- "habari" and
// "vendor_habari" both render as "Habari_" -- so distinct Go types can collapse onto
// one schema identifier. The rendered expression is what the generated file actually
// contains, so it is the only sound basis for ordering.
func referencedSchemaNames(expr string) []string {
	var names []string
	start := -1

	for i := 0; i <= len(expr); i++ {
		isWord := i < len(expr) && (expr[i] == '_' ||
			(expr[i] >= 'a' && expr[i] <= 'z') ||
			(expr[i] >= 'A' && expr[i] <= 'Z') ||
			(expr[i] >= '0' && expr[i] <= '9'))

		if isWord {
			if start == -1 {
				start = i
			}
			continue
		}

		if start != -1 {
			word := expr[start:i]
			if strings.HasSuffix(word, zodSchemaSuffix) && word != zodSchemaSuffix {
				names = append(names, word)
			}
			start = -1
		}
	}

	return names
}

func isSafeJSIdentifier(name string) bool {
	for i, r := range name {
		switch {
		case r == '_' || r == '$':
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// qualify prefixes a schema reference with a module namespace, if there is one.
func qualify(qualifier, name string) string {
	if qualifier == "" {
		return name
	}
	return qualifier + "." + name
}

// fieldSchema renders the schema for one struct field, applying nullish when the
// field is optional.
func fieldSchema(field *GoStructField) string {
	return withNullish(tsTypeToZod(field.TypescriptType), field.Required)
}

// paramSchema renders the schema for one route param or request body field.
func paramSchema(param *RouteHandlerParam, qualifier string) string {
	return withNullish(tsTypeToZodRef(param.TypescriptType, qualifier), param.Required)
}

// withNullish applies .nullish() unless the field is required.
func withNullish(schema string, required bool) string {
	if required {
		return schema
	}
	return schema + zodNullishSuffix
}
