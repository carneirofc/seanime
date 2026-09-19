package codegen

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// selectedStructs is the set of types that reach the frontend.
//
// Both generated files describe exactly this set: types.ts declares a TypeScript
// type for each, and schemas.ts a zod schema. They MUST agree, because
// schemas.assert.ts asserts one against the other at compile time — so the
// selection lives here and is computed once, rather than being reimplemented by
// each generator.
type selectedStructs struct {
	// ByKey is keyed by "package.Name", e.g. "models.User".
	ByKey map[string]*GoStruct
	// ByPackage groups the same structs, each group sorted by FormattedName.
	ByPackage map[string][]*GoStruct
	// Packages lists the package names in emission order.
	Packages []string
}

// All returns every selected struct in package order, then FormattedName order —
// the same order the generated files declare them in.
func (s *selectedStructs) All() []*GoStruct {
	out := make([]*GoStruct, 0, len(s.ByKey))
	for _, pkg := range s.Packages {
		out = append(out, s.ByPackage[pkg]...)
	}
	return out
}

// selectReferencedStructs resolves which types the frontend needs, starting from
// the types the route handlers return and expanding through every field
// reference.
//
// goStructStrs comes from GenerateTypescriptEndpointsFile: the structs referenced
// by handler params and request bodies. A struct reached more than once is
// "shared", which only affects which bucket it starts in — both buckets are
// expanded and merged, so the distinction does not change the result. It is kept
// because removing it would change nothing and risk changing the output.
//
// Mutates the given structs: embedded struct fields are flattened into the
// embedding struct, which is why each caller loads its own copy.
func selectReferencedStructs(handlers []*RouteHandler, goStructs []*GoStruct, goStructStrs []string) (*selectedStructs, error) {
	// e.g. map["models.User"]*GoStruct
	goStructsMap := make(map[string]*GoStruct, len(goStructs))
	for _, goStruct := range goStructs {
		goStructsMap[goStruct.Package+"."+goStruct.Name] = goStruct
	}

	// Expand the structs with embedded structs
	for _, goStruct := range goStructs {
		for _, embeddedStructType := range goStruct.EmbeddedStructTypes {
			if embeddedStructType != "" {
				if usedStruct, ok := goStructsMap[embeddedStructType]; ok {
					for _, usedField := range usedStruct.Fields {
						goStruct.Fields = append(goStruct.Fields, usedField)
					}
				}
			}
		}
	}

	// Count how many times each struct is returned by a route.
	structStrMap := make(map[string]int)
	for _, str := range goStructStrs {
		structStrMap[str]++
	}
	for _, handler := range handlers {
		if handler.Api == nil {
			continue
		}
		switch handler.Api.ReturnTypescriptType {
		case "null", "string", "number", "boolean":
			continue
		}
		structStrMap[handler.Api.ReturnGoType]++
	}

	sharedStructs := make([]*GoStruct, 0)
	otherStructs := make([]*GoStruct, 0)

	for structStr, count := range structStrMap {
		// e.g. "models.User" — anything not package-qualified is not a struct.
		if len(strings.Split(structStr, ".")) != 2 {
			continue
		}
		goStruct, ok := goStructsMap[structStr]
		if !ok {
			continue
		}
		if count > 1 {
			sharedStructs = append(sharedStructs, goStruct)
		} else {
			otherStructs = append(otherStructs, goStruct)
		}
	}

	// Structs the routes never mention but the frontend still needs.
	for _, structName := range additionalStructNames {
		if goStruct, ok := goStructsMap[structName]; ok {
			otherStructs = append(otherStructs, goStruct)
		}
	}

	referenced, ok := getReferencedStructsRecursively(sharedStructs, otherStructs, goStructsMap)
	if !ok {
		return nil, fmt.Errorf("failed to resolve referenced structs")
	}

	// Group by package. Both the package list and each group are sorted, because
	// the maps above iterate in a random order and the generated files must be
	// byte-identical between runs.
	byPackage := make(map[string][]*GoStruct)
	for _, goStruct := range referenced {
		byPackage[goStruct.Package] = append(byPackage[goStruct.Package], goStruct)
	}

	packages := make([]string, 0, len(byPackage))
	for pkg := range byPackage {
		packages = append(packages, pkg)
	}
	slices.SortStableFunc(packages, func(i, j string) int {
		return cmp.Compare(i, j)
	})
	for _, pkg := range packages {
		slices.SortStableFunc(byPackage[pkg], func(i, j *GoStruct) int {
			return cmp.Compare(i.FormattedName, j.FormattedName)
		})
	}

	return &selectedStructs{ByKey: referenced, ByPackage: byPackage, Packages: packages}, nil
}

// formatPackageHeading renders a package name as the section heading used in the
// generated files, e.g. "torrent_client" => "TorrentClient".
func formatPackageHeading(pkg string) string {
	return strings.ReplaceAll(
		cases.Title(language.English, cases.Compact).String(strings.ReplaceAll(pkg, "_", " ")),
		" ", "",
	)
}
