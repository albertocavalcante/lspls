// SPDX-License-Identifier: MIT
//
// Copyright 2026 Alberto Cavalcante. All rights reserved.
// Use of this source code is governed by a MIT-style license
// that can be found in the LICENSE file.

// Package proto generates Protocol Buffer definitions from the LSP specification.
package proto

import (
	"fmt"
	"sort"
	"strings"

	"github.com/albertocavalcante/lspls/generator"
	"github.com/albertocavalcante/lspls/internal/lspbase"
	"github.com/albertocavalcante/lspls/model"
)

// Codegen generates proto3 definitions from the LSP model.
type Codegen struct {
	model           *model.Model
	config          Config
	resolver        *TypeResolver
	typeFilter      map[string]bool   // nil = all types
	pendingWrappers map[string]string // Helper messages generated on-the-fly (name -> definition)
}

// New creates a new proto Codegen.
func New(m *model.Model, cfg Config) *Codegen {
	c := &Codegen{
		model:           m,
		config:          cfg,
		resolver:        NewTypeResolver(m, cfg.IncludeProposed, cfg.TypeOverrides),
		pendingWrappers: make(map[string]string),
	}
	if len(cfg.Types) > 0 {
		c.typeFilter = make(map[string]bool)
		for _, t := range cfg.Types {
			c.typeFilter[t] = true
		}
	}
	return c
}

// Output contains the generated proto content.
type Output struct {
	Proto []byte
}

// shouldInclude returns whether a type should be included in generation output.
func (g *Codegen) shouldInclude(name string, proposed bool) bool {
	if proposed && !g.config.IncludeProposed {
		return false
	}
	if g.typeFilter != nil && !g.typeFilter[name] {
		return false
	}
	return true
}

// Generate produces the proto3 definitions.
func (g *Codegen) Generate() (*Output, error) {
	// Resolve transitive dependencies if filtering
	if g.typeFilter != nil && g.config.ResolveDeps {
		g.typeFilter = generator.ResolveDeps(g.model, g.typeFilter, g.config.IncludeProposed)
	}

	var b strings.Builder

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
		if !g.shouldInclude(alias.Name, alias.Proposed) {
			continue
		}
		// Add comment explaining the mapping
		protoType := g.resolver.Resolve(alias.Name)
		b.WriteString(fmt.Sprintf("// %s -> %s\n", alias.Name, protoType))
	}
	b.WriteString("\n")

	// Generate enums first (dependencies)
	for _, enum := range g.model.Enumerations {
		if !g.shouldInclude(enum.Name, enum.Proposed) {
			continue
		}
		b.WriteString(g.generateEnum(enum))
		b.WriteString("\n")
	}

	// Generate messages
	for _, structure := range g.model.Structures {
		if !g.shouldInclude(structure.Name, structure.Proposed) {
			continue
		}
		b.WriteString(g.generateMessage(structure))
		b.WriteString("\n")
	}

	// Generate union types (oneof)
	for _, alias := range g.model.TypeAliases {
		if !g.shouldInclude(alias.Name, alias.Proposed) {
			continue
		}
		if alias.Type != nil && alias.Type.Kind == "or" {
			// Skip simple mappings (predefined in config)
			if g.resolver.IsMapped(alias.Name) {
				continue
			}

			b.WriteString(g.generateUnion(alias))
			b.WriteString("\n")
		}
	}

	// Generate pending wrappers (from map<K, repeated V>)
	// Sort for determinism
	if len(g.pendingWrappers) > 0 {
		b.WriteString("// Helper messages for complex types (e.g. maps with array values)\n")
		keys := make([]string, 0, len(g.pendingWrappers))
		for k := range g.pendingWrappers {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b.WriteString(g.pendingWrappers[k])
			b.WriteString("\n")
		}
	}

	return &Output{Proto: []byte(b.String())}, nil
}

// generateUnion produces a oneof message for a union type.
func (g *Codegen) generateUnion(alias *model.TypeAlias) string {
	var b strings.Builder

	// Documentation
	if alias.Documentation != "" {
		for _, line := range strings.Split(alias.Documentation, "\n") {
			b.WriteString(fmt.Sprintf("// %s\n", line))
		}
	}

	msgName := toProtoMessageName(alias.Name)
	b.WriteString(fmt.Sprintf("message %s {\n", msgName))
	b.WriteString("  oneof value {\n")

	fieldNum := 1
	for _, item := range alias.Type.Items {
		if item == nil || (item.Kind == "base" && item.Name == "null") {
			continue
		}

		var line string
		var err error

		switch item.Kind {
		case "array":
			line, err = g.generateUnionArrayField(item, fieldNum)
		case "map":
			line, err = g.generateUnionMapField(item, fieldNum)
		default:
			line, err = g.generateUnionStandardField(item, fieldNum)
		}

		if err != nil {
			b.WriteString(fmt.Sprintf("    // skipped %v: %v\n", item, err))
		} else {
			b.WriteString(line)
			fieldNum++
		}
	}

	b.WriteString("  }\n")
	b.WriteString("}\n")
	return b.String()
}

func (g *Codegen) generateUnionArrayField(item *model.Type, fieldNum int) (string, error) {
	elem, err := g.convertType(item.Element)
	if err != nil {
		return "", err
	}

	// Wrapper naming: ArrayOf_Type
	clean := strings.ReplaceAll(elem, ".", "_")
	wrapper := "ArrayOf_" + toProtoMessageName(clean)

	// Register wrapper on-the-fly
	if _, exists := g.pendingWrappers[wrapper]; !exists {
		var wb strings.Builder
		wb.WriteString(fmt.Sprintf("message %s {\n", wrapper))
		wb.WriteString(fmt.Sprintf("  repeated %s items = 1;\n", elem))
		wb.WriteString("}\n")
		g.pendingWrappers[wrapper] = wb.String()
	}

	return fmt.Sprintf("    %s %s_list = %d;\n", wrapper, toProtoFieldName(clean), fieldNum), nil
}

func (g *Codegen) generateUnionMapField(item *model.Type, fieldNum int) (string, error) {
	key, err := convertBaseType(item.Key.Name)
	if err != nil {
		key = "string"
	}
	val, err := g.convertType(item.Value.(*model.Type))
	if err != nil {
		return "", err
	}

	cleanKey := strings.ReplaceAll(key, ".", "_")
	cleanVal := strings.ReplaceAll(val, ".", "_")
	wrapper := fmt.Sprintf("MapOf_%s_%s", toProtoMessageName(cleanKey), toProtoMessageName(cleanVal))

	// Register wrapper on-the-fly
	if _, exists := g.pendingWrappers[wrapper]; !exists {
		var wb strings.Builder
		wb.WriteString(fmt.Sprintf("message %s {\n", wrapper))
		wb.WriteString(fmt.Sprintf("  map<%s, %s> pairs = 1;\n", key, val))
		wb.WriteString("}\n")
		g.pendingWrappers[wrapper] = wb.String()
	}

	return fmt.Sprintf("    %s %s_map = %d;\n", wrapper, toProtoFieldName(cleanVal), fieldNum), nil
}

func (g *Codegen) generateUnionStandardField(item *model.Type, fieldNum int) (string, error) {
	protoType, err := g.convertType(item)
	if err != nil {
		return "", err
	}

	var fieldName string
	switch item.Kind {
	case "base":
		fieldName = item.Name + "_value"
	case "reference":
		fieldName = toProtoFieldName(item.Name)
	default:
		fieldName = "value"
	}

	return fmt.Sprintf("    %s %s = %d;\n", protoType, fieldName, fieldNum), nil
}

// generateHeader produces the proto file header.
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

// convertType converts an LSP type to a proto3 type string.
func (g *Codegen) convertType(t *model.Type) (string, error) {
	if t == nil {
		return "", fmt.Errorf("nil type")
	}

	switch t.Kind {
	case "base":
		return convertBaseType(t.Name)

	case "reference":
		// Check if this is a type alias that maps to a proto type
		protoType := g.resolver.Resolve(t.Name)

		// Check if the resolved type is known
		if !g.resolver.IsKnown(protoType) && !g.resolver.IsKnown(t.Name) {
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
		var keyTypeStr string
		switch t.Key.Kind {
		case "base":
			var err error
			keyTypeStr, err = convertBaseType(t.Key.Name)
			if err != nil {
				return "", err
			}
		case "reference":
			// Type aliases in LSP are typically strings (e.g., DocumentUri)
			keyTypeStr = "string"
		default:
			return "", fmt.Errorf("unsupported map key type: %s", t.Key.Kind)
		}

		valType, ok := t.Value.(*model.Type)
		if !ok {
			return "", fmt.Errorf("map value is not a type")
		}

		// Proto3 doesn't support map<K, repeated V>
		if valType.Kind == "array" {
			// Generate wrapper for the array value
			elemType, err := g.convertType(valType.Element)
			if err != nil {
				return "", err
			}

			// Naming convention: MapArray_{ElementType}
			cleanType := strings.ReplaceAll(elemType, ".", "_")
			wrapperName := "MapArray_" + toProtoMessageName(cleanType)

			// Define wrapper message if not already present
			if _, exists := g.pendingWrappers[wrapperName]; !exists {
				var wb strings.Builder
				wb.WriteString(fmt.Sprintf("message %s {\n", wrapperName))
				wb.WriteString(fmt.Sprintf("  repeated %s items = 1;\n", elemType))
				wb.WriteString("}\n")
				g.pendingWrappers[wrapperName] = wb.String()
			}

			return fmt.Sprintf("map<%s, %s>", keyTypeStr, wrapperName), nil
		}

		valTypeStr, err := g.convertType(valType)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("map<%s, %s>", keyTypeStr, valTypeStr), nil

	case "or":
		// Union types: Try to find a compatible mapping
		// 1. If any member is a reference, use it (assumes optional/oneof compatibility handled elsewhere or acceptable)
		// 2. If all are scalars, use string or Value
		// TODO: Implement proper OneOf support in future
		for _, item := range t.Items {
			if item == nil {
				continue
			}
			if item.Kind == "base" && item.Name == "null" {
				continue
			}

			// Try to convert first valid option
			res, err := g.convertType(item)
			if err == nil {
				return res, nil
			}
		}
		return "", fmt.Errorf("could not resolve union type to a single proto type")

	case "and":
		// Intersection types - use first part
		if len(t.Items) > 0 {
			return g.convertType(t.Items[0])
		}
		return "", fmt.Errorf("empty intersection type")

	case "stringLiteral":
		return "string", nil

	case "integerLiteral":
		return "int32", nil

	case "booleanLiteral":
		return "bool", nil

	case "tuple":
		// Tuple becomes ListValue for now
		return "google.protobuf.ListValue", nil

	default:
		return "", fmt.Errorf("unsupported type kind: %s", t.Kind)
	}
}

// convertBaseType converts an LSP base type to a proto3 type.
func convertBaseType(name string) (string, error) {
	switch name {
	case lspbase.TypeString, lspbase.TypeDocumentURI, lspbase.TypeURI:
		return "string", nil
	case lspbase.TypeInteger:
		return "int32", nil
	case lspbase.TypeUinteger:
		return "uint32", nil
	case lspbase.TypeDecimal:
		return "double", nil
	case lspbase.TypeBoolean:
		return "bool", nil
	case lspbase.TypeNull:
		return "", fmt.Errorf("null type cannot be represented in proto3")
	default:
		return "", fmt.Errorf("unknown base type: %s", name)
	}
}

// toProtoMessageName converts an LSP type name to a proto message name.
func toProtoMessageName(name string) string {
	name = strings.TrimPrefix(name, "$")
	return lspbase.Capitalize(name)
}

// toProtoFieldName converts an LSP field name to a proto field name (snake_case).
func toProtoFieldName(name string) string {
	return lspbase.CamelToSnake(name)
}

// toEnumPrefix converts an enum name to a SCREAMING_SNAKE_CASE prefix.
func toEnumPrefix(name string) string {
	return lspbase.CamelToScreamingSnake(name)
}

// toEnumValueName creates a proto enum value name.
func toEnumValueName(prefix, name string) string {
	valuePart := toEnumPrefix(name)
	return prefix + "_" + valuePart
}
