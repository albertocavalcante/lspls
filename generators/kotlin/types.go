// SPDX-License-Identifier: MIT
//
// Copyright 2026 Alberto Cavalcante. All rights reserved.
// Use of this source code is governed by a MIT-style license
// that can be found in the LICENSE file.

package kotlin

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/albertocavalcante/lspls/internal/lspbase"
	"github.com/albertocavalcante/lspls/model"
)

// kotlinType converts an LSP type to its Kotlin equivalent.
// When nullable is true the outermost type gets a trailing "?".
func (g *Codegen) kotlinType(t *model.Type, nullable bool) string {
	if t == nil {
		return "Any"
	}

	// T | null  →  inner?
	if t.IsOptional() {
		inner := t.NonNullType()
		return g.kotlinType(inner, false) + "?"
	}

	base := g.kotlinTypeInner(t)
	if nullable {
		return base + "?"
	}
	return base
}

// kotlinTypeInner resolves the non-nullable Kotlin type string.
func (g *Codegen) kotlinTypeInner(t *model.Type) string {
	switch t.Kind {
	case "base":
		return kotlinBaseType(t)

	case "reference":
		// Check predefined mapping first (e.g. DocumentUri → String)
		if mapped, ok := DefaultMappings[t.Name]; ok {
			return mapped
		}
		return typeName(t.Name)

	case "array":
		return "List<" + g.kotlinType(t.Element, false) + ">"

	case "map":
		keyType := g.kotlinType(t.Key, false)
		valType := "Any"
		if vt, ok := t.Value.(*model.Type); ok {
			valType = g.kotlinType(vt, false)
		}
		return fmt.Sprintf("Map<%s, %s>", keyType, valType)

	case "literal":
		return "Any"

	case "stringLiteral":
		return "String"

	case "or":
		return g.getOrType(t)

	case "and":
		return "Any"

	case "tuple":
		return "List<Any>"

	default:
		return "Any"
	}
}

// kotlinBaseType maps an LSP base type name to a Kotlin type.
func kotlinBaseType(t *model.Type) string {
	switch t.Name {
	case lspbase.TypeString, lspbase.TypeURI, lspbase.TypeDocumentUri, lspbase.TypeRegExp:
		return "String"
	case lspbase.TypeInteger:
		return "Int"
	case lspbase.TypeUinteger:
		return "UInt"
	case lspbase.TypeDecimal:
		return "Double"
	case lspbase.TypeBoolean:
		return "Boolean"
	case lspbase.TypeNull:
		return "Nothing?"
	case lspbase.TypeLSPAny:
		return "Any?"
	case lspbase.TypeLSPObject:
		return "Map<String, Any?>"
	case lspbase.TypeLSPArray:
		return "List<Any?>"
	default:
		return "Any"
	}
}

// typeNameForIdent returns an identifier-safe name for an LSP type,
// used when building sealed class names (e.g. Or_TextEdit_Location).
func (g *Codegen) typeNameForIdent(t *model.Type) string {
	if t == nil {
		return "Any"
	}
	switch t.Kind {
	case "base":
		return kotlinBaseType(t)
	case "reference":
		return typeName(t.Name)
	case "array":
		return "Arr" + g.typeNameForIdent(t.Element)
	case "map":
		keyName := g.typeNameForIdent(t.Key)
		valName := "Any"
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
		return "Any"
	}
}

// sealedVariantInfo describes one branch of a sealed class.
type sealedVariantInfo struct {
	identName  string // identifier-safe name (for the value class name)
	kotlinType string // full Kotlin type
}

// getOrType returns the Kotlin type name for an "or" union type, registering
// a sealed class for generation if not already done.
func (g *Codegen) getOrType(t *model.Type) string {
	if t.Kind != "or" || len(t.Items) == 0 {
		return "Any"
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
		return "Any"
	}
	if len(nonNullItems) == 1 {
		return g.kotlinType(nonNullItems[0], false)
	}

	// Build pairs for deterministic naming
	var pairs []sealedVariantInfo
	for _, item := range nonNullItems {
		pairs = append(pairs, sealedVariantInfo{
			identName:  g.typeNameForIdent(item),
			kotlinType: g.kotlinType(item, false),
		})
	}

	slices.SortFunc(pairs, func(a, b sealedVariantInfo) int {
		return cmp.Compare(a.identName, b.identName)
	})

	var identNames []string
	for _, p := range pairs {
		identNames = append(identNames, p.identName)
	}

	sealedName := "Or_" + strings.Join(identNames, "_")

	if _, exists := g.sealedTypes.m[sealedName]; !exists {
		g.sealedTypes.set(sealedName, sealedTypeInfo{
			name:     sealedName,
			variants: pairs,
		})
	}

	return sealedName
}

// typeName converts an LSP type name to a valid Kotlin class name.
func typeName(name string) string {
	return lspbase.ExportName(name)
}

// fieldName converts an LSP property name to a Kotlin property name (camelCase).
func fieldName(name string) string {
	return lspbase.StripMeta(name)
}

// enumConstName converts an enum value name to a Kotlin enum constant (SCREAMING_SNAKE).
func enumConstName(name string) string {
	return lspbase.CamelToScreamingSnake(name)
}
