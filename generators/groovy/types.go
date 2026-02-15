// SPDX-License-Identifier: MIT
//
// Copyright 2026 Alberto Cavalcante. All rights reserved.
// Use of this source code is governed by a MIT-style license
// that can be found in the LICENSE file.

package groovy

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/albertocavalcante/lspls/internal/lspbase"
	"github.com/albertocavalcante/lspls/model"
)

// groovyType converts an LSP type to its Groovy equivalent.
// When nullable is true the type is treated as nullable (no suffix in Groovy,
// since all object types are inherently nullable).
func (g *Codegen) groovyType(t *model.Type, nullable bool) string {
	if t == nil {
		return "Object"
	}

	// T | null  →  inner (Groovy objects are inherently nullable)
	if t.IsOptional() {
		inner := t.NonNullType()
		return g.groovyType(inner, false)
	}

	return g.groovyTypeInner(t)
}

// groovyTypeInner resolves the Groovy type string.
func (g *Codegen) groovyTypeInner(t *model.Type) string {
	switch t.Kind {
	case "base":
		return groovyBaseType(t)

	case "reference":
		// Check predefined mapping first (e.g. DocumentUri -> String)
		if mapped, ok := DefaultMappings[t.Name]; ok {
			return mapped
		}
		return typeName(t.Name)

	case "array":
		return "List<" + g.groovyType(t.Element, false) + ">"

	case "map":
		keyType := g.groovyType(t.Key, false)
		valType := "Object"
		if vt, ok := t.Value.(*model.Type); ok {
			valType = g.groovyType(vt, false)
		}
		return fmt.Sprintf("Map<%s, %s>", keyType, valType)

	case "literal":
		return "Object"

	case "stringLiteral":
		return "String"

	case "or":
		return g.getOrType(t)

	case "and":
		return "Object"

	case "tuple":
		return "List<Object>"

	default:
		return "Object"
	}
}

// groovyBaseType maps an LSP base type name to a Groovy type.
func groovyBaseType(t *model.Type) string {
	switch t.Name {
	case lspbase.TypeString, lspbase.TypeURI, lspbase.TypeDocumentURI, lspbase.TypeRegExp:
		return "String"
	case lspbase.TypeInteger:
		return "int"
	case lspbase.TypeUinteger:
		return "int"
	case lspbase.TypeDecimal:
		return "double"
	case lspbase.TypeBoolean:
		return "boolean"
	case lspbase.TypeNull:
		return "Void"
	case lspbase.TypeLSPAny:
		return "Object"
	case lspbase.TypeLSPObject:
		return "Map<String, Object>"
	case lspbase.TypeLSPArray:
		return "List<Object>"
	default:
		return "Object"
	}
}

// typeNameForIdent returns an identifier-safe name for an LSP type,
// used when building union class names (e.g. Or_Integer_String).
func (g *Codegen) typeNameForIdent(t *model.Type) string {
	if t == nil {
		return "Object"
	}
	switch t.Kind {
	case "base":
		return groovyIdentBaseType(t)
	case "reference":
		return typeName(t.Name)
	case "array":
		return "Arr" + g.typeNameForIdent(t.Element)
	case "map":
		keyName := g.typeNameForIdent(t.Key)
		valName := "Object"
		if vt, ok := t.Value.(*model.Type); ok {
			valName = g.typeNameForIdent(vt)
		}
		return "Map" + keyName + valName
	case "literal":
		return "Literal"
	case "stringLiteral":
		return "String"
	case "or":
		return "Union"
	case "and":
		return "Intersection"
	case "tuple":
		return "Tuple"
	default:
		return "Object"
	}
}

// groovyIdentBaseType returns an identifier-friendly name for a base type.
func groovyIdentBaseType(t *model.Type) string {
	switch t.Name {
	case lspbase.TypeString, lspbase.TypeURI, lspbase.TypeDocumentURI, lspbase.TypeRegExp:
		return "String"
	case lspbase.TypeInteger:
		return "Integer"
	case lspbase.TypeUinteger:
		return "Integer"
	case lspbase.TypeDecimal:
		return "Double"
	case lspbase.TypeBoolean:
		return "Boolean"
	case lspbase.TypeNull:
		return "Void"
	case lspbase.TypeLSPAny:
		return "Object"
	case lspbase.TypeLSPObject:
		return "MapStringObject"
	case lspbase.TypeLSPArray:
		return "ListObject"
	default:
		return "Object"
	}
}

// unionVariantInfo describes one branch of a union wrapper class.
type unionVariantInfo struct {
	identName  string // identifier-safe name (for discrimination)
	groovyType string // full Groovy type
}

// getOrType returns the Groovy type name for an "or" union type, registering
// a wrapper class for generation if not already done.
func (g *Codegen) getOrType(t *model.Type) string {
	if t.Kind != "or" || len(t.Items) == 0 {
		return "Object"
	}

	// Filter out null items and proposed types
	var nonNullItems []*model.Type
	for _, item := range t.Items {
		if item.Kind == "base" && item.Name == "null" {
			continue
		}
		if !g.config.IncludeProposed && item.Kind == "reference" && g.isProposed(item.Name) {
			continue
		}
		nonNullItems = append(nonNullItems, item)
	}

	if len(nonNullItems) == 0 {
		return "Object"
	}
	if len(nonNullItems) == 1 {
		return g.groovyType(nonNullItems[0], false)
	}

	// Build pairs for deterministic naming
	var pairs []unionVariantInfo
	for _, item := range nonNullItems {
		pairs = append(pairs, unionVariantInfo{
			identName:  g.typeNameForIdent(item),
			groovyType: g.groovyType(item, false),
		})
	}

	slices.SortFunc(pairs, func(a, b unionVariantInfo) int {
		return cmp.Compare(a.identName, b.identName)
	})

	// Deduplicate variants that map to the same Groovy type
	// (e.g. integer and uinteger both become int/Integer).
	pairs = slices.CompactFunc(pairs, func(a, b unionVariantInfo) bool {
		return a.identName == b.identName
	})

	if len(pairs) == 1 {
		return pairs[0].groovyType
	}

	var identNames []string
	for _, p := range pairs {
		identNames = append(identNames, p.identName)
	}

	unionName := "Or_" + strings.Join(identNames, "_")

	if _, exists := g.unionTypes.m[unionName]; !exists {
		g.unionTypes.set(unionName, unionTypeInfo{
			name:     unionName,
			variants: pairs,
		})
	}

	return unionName
}

// typeName converts an LSP type name to a valid Groovy class name.
func typeName(name string) string {
	return lspbase.ExportName(name)
}

// fieldName converts an LSP property name to a Groovy property name (camelCase).
func fieldName(name string) string {
	return lspbase.StripMeta(name)
}

// enumConstName converts an enum value name to a Groovy enum constant (SCREAMING_SNAKE).
func enumConstName(name string) string {
	return lspbase.CamelToScreamingSnake(name)
}

// isPrimitiveGroovyType reports whether a Groovy type is a primitive/boxed type.
func isPrimitiveGroovyType(t string) bool {
	switch t {
	case "String", "int", "Integer", "double", "Double", "boolean", "Boolean":
		return true
	}
	return false
}

// boxPrimitive converts a primitive type to its boxed equivalent so it can
// hold null. Non-primitive types are returned unchanged.
func boxPrimitive(t string) string {
	switch t {
	case "int":
		return "Integer"
	case "double":
		return "Double"
	case "boolean":
		return "Boolean"
	default:
		return t
	}
}
