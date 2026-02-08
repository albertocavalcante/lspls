// SPDX-License-Identifier: MIT
//
// Copyright 2026 Alberto Cavalcante. All rights reserved.
// Use of this source code is governed by a MIT-style license
// that can be found in the LICENSE file.

// Package proto generates Protocol Buffer definitions from the LSP specification.
package proto

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/albertocavalcante/lspls/model"
)

// Config holds configuration for proto generation.
type Config struct {
	// PackageName is the proto package name (e.g., "lsp").
	PackageName string

	// GoPackage is the go_package option value.
	GoPackage string

	// Types to include (empty means all).
	Types []string

	// ResolveDeps includes transitively referenced types.
	ResolveDeps bool

	// IncludeProposed generates types marked as proposed.
	IncludeProposed bool

	// Source metadata for header comments.
	Source     string
	Ref        string
	CommitHash string
	LSPVersion string
}

// Codegen generates proto3 definitions from the LSP model.
type Codegen struct {
	model      *model.Model
	config     Config
	knownTypes map[string]bool // Types that will be generated (used to skip unknown refs)
}

// New creates a new proto Codegen.
func New(m *model.Model, cfg Config) *Codegen {
	return &Codegen{
		model:      m,
		config:     cfg,
		knownTypes: make(map[string]bool),
	}
}

// Output contains the generated proto content.
type Output struct {
	Proto []byte
}

// Generate produces the proto3 definitions.
func (g *Codegen) Generate() (*Output, error) {
	var b strings.Builder

	// Build set of known types that will be generated
	// This is used to skip fields that reference types we won't generate
	g.buildKnownTypes()

	// Build type alias lookup table
	g.buildTypeAliasMap()

	// Header
	b.WriteString(g.generateHeader())
	b.WriteString("\n")

	// Package declaration
	b.WriteString(fmt.Sprintf("package %s;\n\n", g.config.PackageName))

	// Go package option
	if g.config.GoPackage != "" {
		b.WriteString(fmt.Sprintf("option go_package = %q;\n\n", g.config.GoPackage))
	}

	// Import google.protobuf types for LSPAny, LSPObject
	b.WriteString("// Import well-known types for dynamic values\n")
	b.WriteString("import \"google/protobuf/any.proto\";\n")
	b.WriteString("import \"google/protobuf/struct.proto\";\n\n")

	// Generate type alias comments/definitions
	b.WriteString("// Type Aliases\n")
	b.WriteString("// The following type aliases from LSP are mapped to proto3 types:\n")
	for _, alias := range g.model.TypeAliases {
		if !g.config.IncludeProposed && alias.Proposed {
			continue
		}
		// Add comment explaining the mapping
		protoType := g.resolveTypeAlias(alias.Name)
		b.WriteString(fmt.Sprintf("// %s -> %s\n", alias.Name, protoType))
	}
	b.WriteString("\n")

	// Generate enums first (dependencies)
	for _, enum := range g.model.Enumerations {
		if !g.config.IncludeProposed && enum.Proposed {
			continue
		}
		b.WriteString(g.generateEnum(enum))
		b.WriteString("\n")
	}

	// Generate messages
	for _, structure := range g.model.Structures {
		if !g.config.IncludeProposed && structure.Proposed {
			continue
		}
		b.WriteString(g.generateMessage(structure))
		b.WriteString("\n")
	}

	return &Output{Proto: []byte(b.String())}, nil
}

// buildKnownTypes populates the knownTypes set with all types that will be generated.
// This is used to skip fields that reference proposed types when --proposed is false.
func (g *Codegen) buildKnownTypes() {
	// Add all structures that will be generated
	for _, s := range g.model.Structures {
		if !g.config.IncludeProposed && s.Proposed {
			continue
		}
		g.knownTypes[s.Name] = true
	}

	// Add all enums that will be generated
	for _, e := range g.model.Enumerations {
		if !g.config.IncludeProposed && e.Proposed {
			continue
		}
		g.knownTypes[e.Name] = true
	}

	// Add all type aliases (these are mapped to proto types, so always "known")
	for _, a := range g.model.TypeAliases {
		if !g.config.IncludeProposed && a.Proposed {
			continue
		}
		g.knownTypes[a.Name] = true
	}

	// Add well-known proto types
	g.knownTypes["google.protobuf.Any"] = true
	g.knownTypes["google.protobuf.Value"] = true
	g.knownTypes["google.protobuf.Struct"] = true
	g.knownTypes["google.protobuf.ListValue"] = true
}

// typeAliasMap maps LSP type alias names to their proto3 equivalents.
var typeAliasMap = map[string]string{
	// Simple type aliases -> string
	"DocumentUri":                 "string",
	"URI":                         "string",
	"ChangeAnnotationIdentifier":  "string",
	"Pattern":                     "string",
	"GlobPattern":                 "string",
	"RegularExpressionEngineKind": "string",

	// Progress token can be string or integer - use string for simplicity
	"ProgressToken": "string",

	// Integer-based types
	"DocumentSelector": "string", // Complex type, simplified

	// Dynamic/any types -> google.protobuf.Value
	"LSPAny":    "google.protobuf.Value",
	"LSPObject": "google.protobuf.Struct",
	"LSPArray":  "google.protobuf.ListValue",

	// Other complex type aliases that are really messages - keep as-is
	// These will be generated as separate messages
}

// buildTypeAliasMap processes model.TypeAliases and adds them to the map.
func (g *Codegen) buildTypeAliasMap() {
	for _, alias := range g.model.TypeAliases {
		// Skip if already in static map
		if _, exists := typeAliasMap[alias.Name]; exists {
			continue
		}

		if alias.Type == nil {
			continue
		}

		switch alias.Type.Kind {
		case "base":
			// Simple type alias (e.g., DocumentUri = string)
			if protoType, err := convertBaseType(alias.Type.Name); err == nil {
				typeAliasMap[alias.Name] = protoType
			}

		case "reference":
			// Reference to another type (use as-is, will resolve to message)
			typeAliasMap[alias.Name] = toProtoMessageName(alias.Type.Name)

		case "or":
			// Union type - extract first non-null concrete type
			for _, item := range alias.Type.Items {
				if item == nil {
					continue
				}
				if item.Kind == "base" && item.Name == "null" {
					continue // Skip null
				}
				switch item.Kind {
				case "base":
					if protoType, err := convertBaseType(item.Name); err == nil {
						typeAliasMap[alias.Name] = protoType
					}
				case "reference":
					typeAliasMap[alias.Name] = toProtoMessageName(item.Name)
				}
				break // Use first non-null type
			}

		case "array":
			// Array type alias - map to repeated
			if alias.Type.Element != nil && alias.Type.Element.Kind == "reference" {
				typeAliasMap[alias.Name] = "repeated " + toProtoMessageName(alias.Type.Element.Name)
			}
		}
	}
}

// resolveTypeAlias converts a type alias name to its proto3 equivalent.
func (g *Codegen) resolveTypeAlias(name string) string {
	if protoType, exists := typeAliasMap[name]; exists {
		return protoType
	}
	// Unknown alias - might be a message type
	return toProtoMessageName(name)
}

// isKnownType checks if a type name is known (will be generated or is a scalar/well-known type).
func (g *Codegen) isKnownType(name string) bool {
	// Scalar types are always known
	switch name {
	case "string", "int32", "int64", "uint32", "uint64", "bool", "float", "double", "bytes":
		return true
	}

	// Well-known proto types
	if strings.HasPrefix(name, "google.protobuf.") {
		return true
	}

	// Check knownTypes set
	return g.knownTypes[name]
}

func (g *Codegen) generateHeader() string {
	var b strings.Builder
	b.WriteString("// Code generated by lspls. DO NOT EDIT.\n")
	if g.config.Source != "" {
		b.WriteString(fmt.Sprintf("// Source: %s\n", g.config.Source))
	}
	if g.config.Ref != "" {
		b.WriteString(fmt.Sprintf("// Ref: %s\n", g.config.Ref))
	}
	if g.config.CommitHash != "" {
		b.WriteString(fmt.Sprintf("// Commit: %s\n", g.config.CommitHash))
	}
	if g.config.LSPVersion != "" {
		b.WriteString(fmt.Sprintf("// LSP Version: %s\n", g.config.LSPVersion))
	}
	b.WriteString("\nsyntax = \"proto3\";\n")
	return b.String()
}

func (g *Codegen) generateMessage(s *model.Structure) string {
	var b strings.Builder

	// Documentation
	if s.Documentation != "" {
		for _, line := range strings.Split(s.Documentation, "\n") {
			b.WriteString(fmt.Sprintf("// %s\n", line))
		}
	}

	b.WriteString(fmt.Sprintf("message %s {\n", toProtoMessageName(s.Name)))

	fieldNum := 1
	for _, prop := range s.Properties {
		protoType, err := g.convertType(prop.Type)
		if err != nil {
			// Skip fields we can't convert
			b.WriteString(fmt.Sprintf("  // %s: skipped (%s)\n", prop.Name, err))
			continue
		}

		fieldName := toProtoFieldName(prop.Name)

		// Add field documentation (all lines)
		if prop.Documentation != "" {
			for _, line := range strings.Split(prop.Documentation, "\n") {
				b.WriteString(fmt.Sprintf("  // %s\n", line))
			}
		}

		// In proto3:
		// - repeated fields are inherently optional (can be empty)
		// - map fields are inherently optional
		// - scalar fields can use 'optional' keyword
		isRepeated := strings.HasPrefix(protoType, "repeated ")
		isMap := strings.HasPrefix(protoType, "map<")

		if prop.Optional && !isRepeated && !isMap {
			b.WriteString(fmt.Sprintf("  optional %s %s = %d;\n", protoType, fieldName, fieldNum))
		} else {
			b.WriteString(fmt.Sprintf("  %s %s = %d;\n", protoType, fieldName, fieldNum))
		}
		fieldNum++
	}

	b.WriteString("}\n")
	return b.String()
}

func (g *Codegen) generateEnum(e *model.Enumeration) string {
	var b strings.Builder

	// Documentation
	if e.Documentation != "" {
		for _, line := range strings.Split(e.Documentation, "\n") {
			b.WriteString(fmt.Sprintf("// %s\n", line))
		}
	}

	enumName := toProtoMessageName(e.Name)
	b.WriteString(fmt.Sprintf("enum %s {\n", enumName))

	prefix := toEnumPrefix(e.Name)

	// Check if any defined value is already 0
	hasZeroValue := false
	for _, v := range e.Values {
		switch val := v.Value.(type) {
		case float64:
			if int(val) == 0 {
				hasZeroValue = true
			}
		case int:
			if val == 0 {
				hasZeroValue = true
			}
		}
	}

	// Proto3 requires first value to be 0
	// Only add UNSPECIFIED if no existing value is 0
	if !hasZeroValue {
		b.WriteString(fmt.Sprintf("  %s_UNSPECIFIED = 0;\n", prefix))
	}

	// Track next sequential value for string enums
	nextSeqValue := 1

	for _, v := range e.Values {
		valueName := toEnumValueName(prefix, v.Name)

		// Get the numeric value
		var numValue int
		switch val := v.Value.(type) {
		case float64:
			numValue = int(val)
		case int:
			numValue = val
		case string:
			// String enums - assign sequential numbers
			numValue = nextSeqValue
			nextSeqValue++
		default:
			// Unknown type - skip
			continue
		}

		if v.Documentation != "" {
			b.WriteString(fmt.Sprintf("  // %s\n", strings.Split(v.Documentation, "\n")[0]))
		}
		b.WriteString(fmt.Sprintf("  %s = %d;\n", valueName, numValue))
	}

	b.WriteString("}\n")
	return b.String()
}

func (g *Codegen) convertType(t *model.Type) (string, error) {
	if t == nil {
		return "", fmt.Errorf("nil type")
	}

	switch t.Kind {
	case "base":
		return convertBaseType(t.Name)

	case "reference":
		// Check if this is a type alias that maps to a proto type
		protoType := g.resolveTypeAlias(t.Name)

		// Check if the resolved type is known (either a scalar, well-known type, or generated type)
		if !g.isKnownType(protoType) && !g.isKnownType(t.Name) {
			return "", fmt.Errorf("references proposed type %q (use --proposed to include)", t.Name)
		}

		return protoType, nil

	case "array":
		elemType, err := g.convertType(t.Element)
		if err != nil {
			return "", err
		}
		return "repeated " + elemType, nil

	case "map":
		// Proto3 map keys must be scalar types (string, int32, etc.)
		// Reference types (type aliases) are typically strings in LSP
		var keyTypeStr string
		switch t.Key.Kind {
		case "base":
			var err error
			keyTypeStr, err = convertBaseType(t.Key.Name)
			if err != nil {
				return "", err
			}
		case "reference":
			// Type aliases in LSP are typically strings (e.g., DocumentUri, ChangeAnnotationIdentifier)
			keyTypeStr = "string"
		default:
			return "", fmt.Errorf("unsupported map key type: %s", t.Key.Kind)
		}

		valType, ok := t.Value.(*model.Type)
		if !ok {
			return "", fmt.Errorf("map value is not a type")
		}

		// Proto3 doesn't support map<K, repeated V>
		// If value is an array, skip this field (would need wrapper message)
		if valType.Kind == "array" {
			return "", fmt.Errorf("map with array values not supported (would need wrapper message)")
		}

		valTypeStr, err := g.convertType(valType)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("map<%s, %s>", keyTypeStr, valTypeStr), nil

	case "or":
		// Union types - for now, use google.protobuf.Any or first non-null type
		for _, item := range t.Items {
			if item.Kind != "base" || item.Name != "null" {
				return g.convertType(item)
			}
		}
		return "", fmt.Errorf("union with only null types")

	case "literal":
		// Inline struct - would need nested message
		return "", fmt.Errorf("literal types require nested messages")

	case "stringLiteral":
		return "string", nil

	case "tuple":
		return "", fmt.Errorf("tuples not directly supported in proto3")

	default:
		return "", fmt.Errorf("unknown type kind: %s", t.Kind)
	}
}

// convertBaseType converts an LSP base type to a proto3 type.
func convertBaseType(name string) (string, error) {
	switch name {
	case "string", "DocumentUri", "URI":
		return "string", nil
	case "integer":
		return "int32", nil
	case "uinteger":
		return "uint32", nil
	case "decimal":
		return "double", nil
	case "boolean":
		return "bool", nil
	case "null":
		return "", fmt.Errorf("null type cannot be represented in proto3")
	default:
		return "", fmt.Errorf("unknown base type: %s", name)
	}
}

// toProtoMessageName converts an LSP type name to a proto message name.
func toProtoMessageName(name string) string {
	// Strip leading $ if present
	name = strings.TrimPrefix(name, "$")
	if name == "" {
		return name
	}
	// Ensure first letter is uppercase
	return string(unicode.ToUpper(rune(name[0]))) + name[1:]
}

// toProtoFieldName converts an LSP field name to a proto field name (snake_case).
func toProtoFieldName(name string) string {
	// Check if entire name is uppercase (like URI, ID)
	allUpper := true
	for _, r := range name {
		if !unicode.IsUpper(r) && unicode.IsLetter(r) {
			allUpper = false
			break
		}
	}
	if allUpper {
		return strings.ToLower(name)
	}

	var result strings.Builder
	for i, r := range name {
		if unicode.IsUpper(r) {
			if i > 0 {
				result.WriteRune('_')
			}
			result.WriteRune(unicode.ToLower(r))
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// toEnumPrefix converts an enum name to a SCREAMING_SNAKE_CASE prefix.
func toEnumPrefix(name string) string {
	var result strings.Builder
	for i, r := range name {
		if unicode.IsUpper(r) && i > 0 {
			result.WriteRune('_')
		}
		result.WriteRune(unicode.ToUpper(r))
	}
	return result.String()
}

// toEnumValueName creates a proto enum value name.
func toEnumValueName(prefix, name string) string {
	valuePart := toEnumPrefix(name)
	return prefix + "_" + valuePart
}
