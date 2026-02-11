// SPDX-License-Identifier: MIT AND BSD-3-Clause

package golang

import (
	"bytes"
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/albertocavalcante/lspls/internal/lspbase"
	"github.com/albertocavalcante/lspls/model"
)

func (g *Generator) generateStructure(s *model.Structure) {
	var buf bytes.Buffer

	// Doc comment
	if s.Documentation != "" {
		writeDocComment(&buf, s.Documentation)
	}
	// Add @since only if not already in documentation (check for version pattern)
	if s.Since != "" && !strings.Contains(s.Documentation, "@since "+s.Since) {
		fmt.Fprintf(&buf, "//\n// @since %s\n", s.Since)
	}

	// Type declaration
	fmt.Fprintf(&buf, "type %s struct {\n", exportName(s.Name))

	// Embedded types (extends)
	for _, ext := range s.Extends {
		if ext.Kind == "reference" {
			fmt.Fprintf(&buf, "\t%s\n", exportName(ext.Name))
		}
	}

	// Mixins
	for _, mix := range s.Mixins {
		if mix.Kind == "reference" {
			fmt.Fprintf(&buf, "\t%s\n", exportName(mix.Name))
		}
	}

	// Properties
	for _, p := range s.Properties {
		// Skip proposed properties when not including proposed types
		if p.Proposed && !g.config.IncludeProposed {
			continue
		}
		g.generateProperty(&buf, &p)
	}

	buf.WriteString("}\n\n")
	g.types.set(s.Name, buf.String())
}

func (g *Generator) generateProperty(buf *bytes.Buffer, p *model.Property) {
	// Doc comment for property
	if p.Documentation != "" {
		for line := range strings.SplitSeq(p.Documentation, "\n") {
			fmt.Fprintf(buf, "\t// %s\n", line)
		}
	}

	// Field declaration
	goName := exportName(p.Name)
	goType := g.goType(p.Type, p.Optional)

	jsonTag := p.Name
	if p.Optional {
		jsonTag += ",omitempty"
	}

	fmt.Fprintf(buf, "\t%s %s `json:\"%s\"`\n", goName, goType, jsonTag)
}

func (g *Generator) generateEnumeration(e *model.Enumeration) {
	// Generate type
	var typeBuf bytes.Buffer
	if e.Documentation != "" {
		writeDocComment(&typeBuf, e.Documentation)
	}
	// Add @since only if not already in documentation (check for version pattern)
	if e.Since != "" && !strings.Contains(e.Documentation, "@since "+e.Since) {
		fmt.Fprintf(&typeBuf, "//\n// @since %s\n", e.Since)
	}

	baseType := g.goBaseType(e.Type)
	fmt.Fprintf(&typeBuf, "type %s %s\n\n", exportName(e.Name), baseType)
	g.types.set(e.Name, typeBuf.String())

	// Generate constants
	for _, v := range e.Values {
		var constBuf bytes.Buffer
		if v.Documentation != "" {
			writeDocComment(&constBuf, v.Documentation)
		}

		constName := exportName(e.Name) + exportName(v.Name)
		constValue := formatConstValue(v.Value, baseType)
		fmt.Fprintf(&constBuf, "%s %s = %s\n", constName, exportName(e.Name), constValue)

		g.consts.set(constName, constBuf.String())
	}
}

func (g *Generator) generateTypeAlias(a *model.TypeAlias) {
	var buf bytes.Buffer

	if a.Documentation != "" {
		writeDocComment(&buf, a.Documentation)
	}
	// Add @since only if not already in documentation (check for version pattern)
	if a.Since != "" && !strings.Contains(a.Documentation, "@since "+a.Since) {
		fmt.Fprintf(&buf, "//\n// @since %s\n", a.Since)
	}
	if a.Deprecated != "" {
		fmt.Fprintf(&buf, "//\n// Deprecated: %s\n", a.Deprecated)
	}

	goType := g.goType(a.Type, false)
	fmt.Fprintf(&buf, "type %s = %s\n\n", exportName(a.Name), goType)

	g.types.set(a.Name, buf.String())
}

// goType converts an LSP type to its Go equivalent.
func (g *Generator) goType(t *model.Type, _ bool) string {
	if t == nil {
		return "any"
	}

	// Handle optional types (T | null)
	if t.IsOptional() {
		inner := t.NonNullType()
		return "*" + g.goType(inner, false)
	}

	switch t.Kind {
	case "base":
		return g.goBaseType(t)

	case "reference":
		return exportName(t.Name)

	case "array":
		return "[]" + g.goType(t.Element, false)

	case "map":
		keyType := g.goType(t.Key, false)
		valType := "any"
		if vt, ok := t.Value.(*model.Type); ok {
			valType = g.goType(vt, false)
		}
		return fmt.Sprintf("map[%s]%s", keyType, valType)

	case "literal":
		// Anonymous struct - for now, use any
		// TODO: Generate named type
		return "any"

	case "stringLiteral":
		return "string"

	case "or":
		// Union type - generate Or_* type with JSON marshaling
		return g.getOrType(t)

	case "and":
		// Intersection - use embedded structs
		return "any"

	case "tuple":
		// Tuple - use slice for now
		return "[]any"

	default:
		return "any"
	}
}

func (g *Generator) goBaseType(t *model.Type) string {
	if t == nil {
		return "any"
	}
	switch t.Name {
	case lspbase.TypeString, lspbase.TypeURI, lspbase.TypeDocumentURI:
		return "string"
	case lspbase.TypeInteger:
		return "int32"
	case lspbase.TypeUinteger:
		return "uint32"
	case lspbase.TypeDecimal:
		return "float64"
	case lspbase.TypeBoolean:
		return "bool"
	case lspbase.TypeNull, lspbase.TypeLSPAny:
		return "any"
	default:
		return "any"
	}
}

// typeNameForIdent returns a Go-identifier-safe name for a type.
// This is used when building Or_* type names where []Location or map[K]V
// would be invalid in an identifier.
func (g *Generator) typeNameForIdent(t *model.Type) string {
	if t == nil {
		return "any"
	}

	switch t.Kind {
	case "base":
		// Base types are already safe identifiers (int32, string, etc.)
		return g.goBaseType(t)
	case "reference":
		return exportName(t.Name)
	case "array":
		return "Arr" + g.typeNameForIdent(t.Element)
	case "map":
		keyName := g.typeNameForIdent(t.Key)
		valName := "any"
		if vt, ok := t.Value.(*model.Type); ok {
			valName = g.typeNameForIdent(vt)
		}
		return "Map" + keyName + valName
	case "literal":
		return "Literal"
	case "stringLiteral":
		return "string"
	case "or":
		// Nested unions are rare, but handle them
		return "Union"
	case "and":
		return "Intersection"
	case "tuple":
		return "Tuple"
	default:
		return "any"
	}
}

// getOrType returns the Go type name for an "or" union type, registering it
// for generation if not already done. Returns "any" for empty or single-item unions.
func (g *Generator) getOrType(t *model.Type) string {
	if t.Kind != "or" || len(t.Items) == 0 {
		return "any"
	}

	// Filter out null items (already handled by IsOptional) and
	// proposed types when IncludeProposed is false
	var nonNullItems []*model.Type
	for _, item := range t.Items {
		if item.Kind == "base" && item.Name == "null" {
			continue
		}
		// Skip proposed reference types when not including proposed features
		if !g.config.IncludeProposed && item.Kind == "reference" && g.isProposed(item.Name) {
			continue
		}
		nonNullItems = append(nonNullItems, item)
	}

	// If only one non-null item, just use that type directly
	if len(nonNullItems) == 1 {
		return g.goType(nonNullItems[0], false)
	}

	// If no items left, return any
	if len(nonNullItems) == 0 {
		return "any"
	}

	// Build pairs of (identName, goType) for each item so we can sort together
	type namePair struct {
		identName string
		goType    string
	}
	var pairs []namePair
	for _, item := range nonNullItems {
		pairs = append(pairs, namePair{
			identName: g.typeNameForIdent(item),
			goType:    g.goType(item, false),
		})
	}

	// Sort by identifier-safe name (for deterministic Or_* type names)
	slices.SortFunc(pairs, func(a, b namePair) int {
		return cmp.Compare(a.identName, b.identName)
	})

	// Extract sorted names
	var identNames []string
	var itemNames []string
	for _, p := range pairs {
		identNames = append(identNames, p.identName)
		itemNames = append(itemNames, p.goType)
	}

	// Generate the type name: Or_Type1_Type2_... (using identifier-safe names)
	typeName := "Or_" + strings.Join(identNames, "_")

	// Check if we've already registered this type
	if _, exists := g.orTypes.m[typeName]; !exists {
		g.orTypes.set(typeName, orTypeInfo{
			name:      typeName,
			itemNames: itemNames,
		})
	}

	return typeName
}

// generateOrTypes generates all registered Or_* union types and their JSON methods.
func (g *Generator) generateOrTypes() string {
	var buf bytes.Buffer

	for _, name := range g.orTypes.keys() {
		info := g.orTypes.get(name)
		g.generateOrType(&buf, info)
	}

	return buf.String()
}

// generateOrType generates a single Or_* union type with its MarshalJSON and UnmarshalJSON methods.
func (g *Generator) generateOrType(buf *bytes.Buffer, info orTypeInfo) {
	// Type comment listing the union members
	fmt.Fprintf(buf, "// %s is a union type for: %s\n", info.name, strings.Join(info.itemNames, " | "))
	fmt.Fprintf(buf, "type %s struct {\n", info.name)
	fmt.Fprintf(buf, "\tValue any `json:\"value\"`\n")
	buf.WriteString("}\n\n")

	// MarshalJSON method
	fmt.Fprintf(buf, "func (t %s) MarshalJSON() ([]byte, error) {\n", info.name)
	buf.WriteString("\tswitch x := t.Value.(type) {\n")
	for _, name := range info.itemNames {
		fmt.Fprintf(buf, "\tcase %s:\n", name)
		buf.WriteString("\t\treturn json.Marshal(x)\n")
	}
	buf.WriteString("\tcase nil:\n")
	buf.WriteString("\t\treturn []byte(\"null\"), nil\n")
	buf.WriteString("\t}\n")
	fmt.Fprintf(buf, "\treturn nil, fmt.Errorf(\"type %%T not one of %v\", t.Value)\n", info.itemNames)
	buf.WriteString("}\n\n")

	// UnmarshalJSON method
	fmt.Fprintf(buf, "func (t *%s) UnmarshalJSON(x []byte) error {\n", info.name)
	buf.WriteString("\tif string(x) == \"null\" {\n")
	buf.WriteString("\t\tt.Value = nil\n")
	buf.WriteString("\t\treturn nil\n")
	buf.WriteString("\t}\n")
	for i, name := range info.itemNames {
		fmt.Fprintf(buf, "\tvar h%d %s\n", i, name)
		fmt.Fprintf(buf, "\tif err := json.Unmarshal(x, &h%d); err == nil {\n", i)
		fmt.Fprintf(buf, "\t\tt.Value = h%d\n", i)
		buf.WriteString("\t\treturn nil\n")
		buf.WriteString("\t}\n")
	}
	fmt.Fprintf(buf, "\treturn fmt.Errorf(\"unmarshal failed to match one of %v\")\n", info.itemNames)
	buf.WriteString("}\n\n")
}

func exportName(name string) string {
	return lspbase.ExportName(name)
}

func writeDocComment(buf *bytes.Buffer, doc string) {
	for line := range strings.SplitSeq(doc, "\n") {
		fmt.Fprintf(buf, "// %s\n", line)
	}
}

func formatConstValue(v any, baseType string) string {
	switch val := v.(type) {
	case string:
		return fmt.Sprintf("%q", val)
	case float64:
		if baseType == "string" {
			return fmt.Sprintf("%q", fmt.Sprintf("%v", val))
		}
		return fmt.Sprintf("%d", int64(val))
	default:
		return fmt.Sprintf("%v", v)
	}
}
