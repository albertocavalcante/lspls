// SPDX-License-Identifier: MIT AND BSD-3-Clause
//
// Copyright 2026 Alberto Cavalcante. All rights reserved.
// Use of this source code is governed by a MIT-style license
// that can be found in the LICENSE file.
//
// Code generation logic inspired by golang.org/x/tools/gopls:
// https://github.com/golang/tools/blob/master/gopls/internal/protocol/generate/output.go
// Copyright 2022 The Go Authors. All rights reserved.
// See NOTICE file for the full license text.

// Package codegen generates Go source code from the LSP specification model.
package codegen

import (
	"bytes"
	"fmt"
	"go/format"
	"sort"
	"strings"
	"unicode"

	"github.com/albertocavalcante/lspls/internal/model"
)

// Config controls code generation behavior.
type Config struct {
	// PackageName is the Go package name for generated code.
	PackageName string

	// Types limits generation to specific type names.
	// If empty, all types are generated.
	Types []string

	// IncludeProposed includes proposed (unstable) features.
	IncludeProposed bool

	// GenerateClient generates the Client interface.
	GenerateClient bool

	// GenerateServer generates the Server interface.
	GenerateServer bool

	// GenerateJSON generates custom JSON marshaling code.
	GenerateJSON bool

	// Source describes where the spec came from (for header comment).
	Source string

	// Ref is the git reference used (for header comment).
	Ref string

	// CommitHash is the git commit (for header comment).
	CommitHash string

	// LSPVersion is the protocol version (for header comment).
	LSPVersion string
}

// DefaultConfig returns sensible defaults for code generation.
func DefaultConfig() Config {
	return Config{
		PackageName:     "protocol",
		IncludeProposed: false,
		GenerateClient:  true,
		GenerateServer:  true,
		GenerateJSON:    true,
	}
}

// Output contains the generated code files.
type Output struct {
	Protocol []byte // Type definitions and constants
	Client   []byte // Client interface and dispatcher
	Server   []byte // Server interface and dispatcher
	JSON     []byte // Custom JSON marshaling
}

// Generator produces Go code from an LSP model.
type Generator struct {
	model  *model.Model
	config Config

	// Generated code buffers
	types  *orderedMap[string]
	consts *orderedMap[string]

	// Type filter (nil = all types)
	typeFilter map[string]bool
}

// New creates a new Generator.
func New(m *model.Model, cfg Config) *Generator {
	g := &Generator{
		model:  m,
		config: cfg,
		types:  newOrderedMap[string](),
		consts: newOrderedMap[string](),
	}

	if len(cfg.Types) > 0 {
		g.typeFilter = make(map[string]bool)
		for _, t := range cfg.Types {
			g.typeFilter[t] = true
		}
	}

	return g
}

// Generate produces all output files.
func (g *Generator) Generate() (*Output, error) {
	// Process all structures
	for _, s := range g.model.Structures {
		if !g.shouldInclude(s.Name, s.Proposed) {
			continue
		}
		g.generateStructure(s)
	}

	// Process all enumerations
	for _, e := range g.model.Enumerations {
		if !g.shouldInclude(e.Name, e.Proposed) {
			continue
		}
		g.generateEnumeration(e)
	}

	// Process all type aliases
	for _, a := range g.model.TypeAliases {
		if !g.shouldInclude(a.Name, a.Proposed) {
			continue
		}
		g.generateTypeAlias(a)
	}

	out := &Output{}
	var err error

	// Generate protocol.go
	out.Protocol, err = g.generateProtocolFile()
	if err != nil {
		return nil, fmt.Errorf("generate protocol: %w", err)
	}

	return out, nil
}

func (g *Generator) shouldInclude(name string, proposed bool) bool {
	if proposed && !g.config.IncludeProposed {
		return false
	}
	if g.typeFilter != nil && !g.typeFilter[name] {
		return false
	}
	return true
}

func (g *Generator) generateStructure(s *model.Structure) {
	var buf bytes.Buffer

	// Doc comment
	if s.Documentation != "" {
		writeDocComment(&buf, s.Documentation)
	}
	if s.Since != "" {
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
		g.generateProperty(&buf, &p)
	}

	buf.WriteString("}\n\n")
	g.types.set(s.Name, buf.String())
}

func (g *Generator) generateProperty(buf *bytes.Buffer, p *model.Property) {
	// Doc comment for property
	if p.Documentation != "" {
		lines := strings.Split(p.Documentation, "\n")
		for _, line := range lines {
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
	if e.Since != "" {
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
	if a.Since != "" {
		fmt.Fprintf(&buf, "//\n// @since %s\n", a.Since)
	}
	if a.Deprecated != "" {
		fmt.Fprintf(&buf, "//\n// Deprecated: %s\n", a.Deprecated)
	}

	goType := g.goType(a.Type, false)
	fmt.Fprintf(&buf, "type %s = %s\n\n", exportName(a.Name), goType)

	g.types.set(a.Name, buf.String())
}

func (g *Generator) generateProtocolFile() ([]byte, error) {
	var buf bytes.Buffer

	// Header
	buf.WriteString(g.fileHeader())
	buf.WriteString("package " + g.config.PackageName + "\n\n")

	// Imports (minimal for now)
	buf.WriteString("import \"encoding/json\"\n\n")
	buf.WriteString("var _ = json.RawMessage{} // suppress unused import\n\n")

	// Types
	for _, name := range g.types.keys() {
		buf.WriteString(g.types.get(name))
	}

	// Constants
	if len(g.consts.keys()) > 0 {
		buf.WriteString("const (\n")
		for _, name := range g.consts.keys() {
			buf.WriteString("\t")
			buf.WriteString(g.consts.get(name))
		}
		buf.WriteString(")\n")
	}

	return format.Source(buf.Bytes())
}

func (g *Generator) fileHeader() string {
	var lines []string
	lines = append(lines, "// Code generated by lspls. DO NOT EDIT.")
	if g.config.Source != "" {
		lines = append(lines, fmt.Sprintf("// Source: %s", g.config.Source))
	}
	if g.config.Ref != "" {
		lines = append(lines, fmt.Sprintf("// Ref: %s", g.config.Ref))
	}
	if g.config.CommitHash != "" {
		lines = append(lines, fmt.Sprintf("// Commit: %s", g.config.CommitHash))
	}
	if g.config.LSPVersion != "" {
		lines = append(lines, fmt.Sprintf("// LSP Version: %s", g.config.LSPVersion))
	}
	lines = append(lines, "")
	return strings.Join(lines, "\n")
}

// goType converts an LSP type to its Go equivalent.
func (g *Generator) goType(t *model.Type, optional bool) string {
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
		// Union type - use any for now
		// TODO: Generate custom type with JSON marshaling
		return "any"

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
	case "string":
		return "string"
	case "integer":
		return "int32"
	case "uinteger":
		return "uint32"
	case "decimal":
		return "float64"
	case "boolean":
		return "bool"
	case "null":
		return "any"
	case "URI", "DocumentUri":
		return "string"
	case "LSPAny":
		return "any"
	default:
		return "any"
	}
}

// Helper functions

func exportName(name string) string {
	if name == "" {
		return ""
	}
	// Handle names starting with underscore (internal types)
	if name[0] == '_' {
		return "X" + name[1:]
	}
	// Capitalize first letter
	runes := []rune(name)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func writeDocComment(buf *bytes.Buffer, doc string) {
	lines := strings.Split(doc, "\n")
	for _, line := range lines {
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

// orderedMap maintains insertion order for deterministic output.
type orderedMap[T any] struct {
	m     map[string]T
	order []string
}

func newOrderedMap[T any]() *orderedMap[T] {
	return &orderedMap[T]{
		m: make(map[string]T),
	}
}

func (m *orderedMap[T]) set(key string, value T) {
	if _, exists := m.m[key]; !exists {
		m.order = append(m.order, key)
	}
	m.m[key] = value
}

func (m *orderedMap[T]) get(key string) T {
	return m.m[key]
}

func (m *orderedMap[T]) keys() []string {
	// Sort for deterministic output
	sorted := make([]string, len(m.order))
	copy(sorted, m.order)
	sort.Strings(sorted)
	return sorted
}
