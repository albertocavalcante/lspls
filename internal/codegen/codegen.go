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
	"cmp"
	"fmt"
	"go/format"
	"slices"
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

	// ResolveDeps automatically includes types referenced by filtered types.
	// When true, if you filter for "Range", types like "Position" that Range
	// references will also be included. Default: true.
	ResolveDeps bool

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
		ResolveDeps:     true,
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

	// orTypes tracks generated Or_* union types to avoid duplicates.
	// Key is the type name (e.g., "Or_TextEdit_AnnotatedTextEdit"), value is the type definition.
	orTypes *orderedMap[orTypeInfo]

	// proposedTypes caches whether a type is proposed for O(1) lookup.
	proposedTypes map[string]bool

	// serverMethods holds methods for the Server interface (clientToServer and both).
	serverMethods *orderedMap[methodInfo]

	// clientMethods holds methods for the Client interface (serverToClient and both).
	clientMethods *orderedMap[methodInfo]

	// methodConsts holds method name constants (e.g., MethodTextDocumentHover = "textDocument/hover").
	methodConsts *orderedMap[string]
}

// orTypeInfo holds information about a generated Or_* type.
type orTypeInfo struct {
	name      string   // Type name (e.g., "Or_TextEdit_AnnotatedTextEdit")
	itemNames []string // Sorted Go type names of union members
}

// methodInfo holds information about an LSP method for interface generation.
type methodInfo struct {
	name           string // Go method name (e.g., "TextDocumentHover")
	method         string // LSP method string (e.g., "textDocument/hover")
	paramsType     string // Go params type (e.g., "*HoverParams"), empty if no params
	resultType     string // Go result type (e.g., "*Hover"), empty for notifications
	documentation  string // Method documentation
	isNotification bool   // true for notifications, false for requests
}

// New creates a new Generator.
func New(m *model.Model, cfg Config) *Generator {
	g := &Generator{
		model:         m,
		config:        cfg,
		types:         newOrderedMap[string](),
		consts:        newOrderedMap[string](),
		orTypes:       newOrderedMap[orTypeInfo](),
		proposedTypes: buildProposedCache(m),
		serverMethods: newOrderedMap[methodInfo](),
		clientMethods: newOrderedMap[methodInfo](),
		methodConsts:  newOrderedMap[string](),
	}

	if len(cfg.Types) > 0 {
		g.typeFilter = make(map[string]bool)
		for _, t := range cfg.Types {
			g.typeFilter[t] = true
		}
	}

	return g
}

// buildProposedCache builds a cache of proposed type names for O(1) lookup.
func buildProposedCache(m *model.Model) map[string]bool {
	cache := make(map[string]bool)
	for _, s := range m.Structures {
		cache[s.Name] = s.Proposed
	}
	for _, e := range m.Enumerations {
		cache[e.Name] = e.Proposed
	}
	for _, a := range m.TypeAliases {
		cache[a.Name] = a.Proposed
	}
	return cache
}

// Generate produces all output files.
func (g *Generator) Generate() (*Output, error) {
	// Resolve transitive dependencies if filtering
	g.resolveTransitiveDeps()

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

	// Process requests and notifications for interface generation.
	// Skip when filtering specific types since interfaces would reference
	// types not included in the filtered output.
	if g.typeFilter == nil && (g.config.GenerateServer || g.config.GenerateClient) {
		g.processRequests()
		g.processNotifications()
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

// isProposed returns true if the type with the given name is proposed.
func (g *Generator) isProposed(name string) bool {
	return g.proposedTypes[name]
}

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

func (g *Generator) generateProtocolFile() ([]byte, error) {
	var buf bytes.Buffer

	// Header
	buf.WriteString(g.fileHeader())
	buf.WriteString("package " + g.config.PackageName + "\n\n")

	// Determine which imports are needed
	hasOrTypes := len(g.orTypes.keys()) > 0
	hasInterfaces := len(g.serverMethods.keys()) > 0 || len(g.clientMethods.keys()) > 0

	// Generate imports
	if hasOrTypes || hasInterfaces {
		buf.WriteString("import (\n")
		if hasInterfaces {
			buf.WriteString("\t\"context\"\n")
		}
		buf.WriteString("\t\"encoding/json\"\n")
		if hasOrTypes {
			buf.WriteString("\t\"fmt\"\n")
		}
		buf.WriteString(")\n\n")
	} else {
		buf.WriteString("import \"encoding/json\"\n\n")
		buf.WriteString("var _ = json.RawMessage{} // suppress unused import\n\n")
	}

	// Types
	for _, name := range g.types.keys() {
		buf.WriteString(g.types.get(name))
	}

	// Or_* union types
	buf.WriteString(g.generateOrTypes())

	// Constants (enum values)
	if len(g.consts.keys()) > 0 {
		buf.WriteString("const (\n")
		for _, name := range g.consts.keys() {
			buf.WriteString("\t")
			buf.WriteString(g.consts.get(name))
		}
		buf.WriteString(")\n\n")
	}

	// Interfaces (method constants, Server, Client)
	buf.WriteString(g.generateInterfaces())

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

// methodToGoName converts an LSP method name to a Go method name.
// Examples:
//   - "textDocument/hover" → "TextDocumentHover"
//   - "$/cancelRequest" → "CancelRequest"
//   - "initialize" → "Initialize"
func methodToGoName(method string) string {
	// Strip $/ prefix
	method = strings.TrimPrefix(method, "$/")

	var result strings.Builder
	capitalizeNext := true

	for _, r := range method {
		if r == '/' {
			capitalizeNext = true
			continue
		}
		if capitalizeNext {
			result.WriteRune(unicode.ToUpper(r))
			capitalizeNext = false
		} else {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// processRequests processes all requests from the model and adds them to
// the appropriate interface (server, client, or both).
func (g *Generator) processRequests() {
	for _, req := range g.model.Requests {
		if req.Proposed && !g.config.IncludeProposed {
			continue
		}

		info := methodInfo{
			name:           methodToGoName(req.Method),
			method:         req.Method,
			documentation:  req.Documentation,
			isNotification: false,
		}

		// Set params type
		if req.Params != nil {
			info.paramsType = "*" + g.goType(req.Params, false)
		}

		// Set result type
		if req.Result != nil {
			resultType := g.goType(req.Result, false)
			// Add pointer prefix if not already a pointer or slice
			if !strings.HasPrefix(resultType, "*") && !strings.HasPrefix(resultType, "[]") {
				resultType = "*" + resultType
			}
			info.resultType = resultType
		}

		g.addMethodToInterfaces(info, req.Direction)
	}
}

// processNotifications processes all notifications from the model and adds them
// to the appropriate interface (server, client, or both).
func (g *Generator) processNotifications() {
	for _, notif := range g.model.Notifications {
		if notif.Proposed && !g.config.IncludeProposed {
			continue
		}

		info := methodInfo{
			name:           methodToGoName(notif.Method),
			method:         notif.Method,
			documentation:  notif.Documentation,
			isNotification: true,
		}

		// Set params type
		if notif.Params != nil {
			info.paramsType = "*" + g.goType(notif.Params, false)
		}

		g.addMethodToInterfaces(info, notif.Direction)
	}
}

// addMethodToInterfaces adds a method to the appropriate interface(s) based on direction
// and registers the method constant.
func (g *Generator) addMethodToInterfaces(info methodInfo, direction string) {
	// Add method constant
	constName := "Method" + info.name
	g.methodConsts.set(constName, fmt.Sprintf("%s = %q", constName, info.method))

	// Add to appropriate interface(s) based on direction
	switch direction {
	case "clientToServer":
		if g.config.GenerateServer {
			g.serverMethods.set(info.name, info)
		}
	case "serverToClient":
		if g.config.GenerateClient {
			g.clientMethods.set(info.name, info)
		}
	case "both":
		if g.config.GenerateServer {
			g.serverMethods.set(info.name, info)
		}
		if g.config.GenerateClient {
			g.clientMethods.set(info.name, info)
		}
	}
}

// generateMethodConstants generates the const block with LSP method name constants.
func (g *Generator) generateMethodConstants() string {
	keys := g.methodConsts.keys()
	if len(keys) == 0 {
		return ""
	}

	var buf bytes.Buffer
	buf.WriteString("// LSP method names.\n")
	buf.WriteString("const (\n")
	for _, key := range keys {
		fmt.Fprintf(&buf, "\t%s\n", g.methodConsts.get(key))
	}
	buf.WriteString(")\n\n")
	return buf.String()
}

// generateInterface generates a single interface with its methods.
func (g *Generator) generateInterface(name string, methods *orderedMap[methodInfo]) string {
	keys := methods.keys()
	if len(keys) == 0 {
		return ""
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "// %s defines the LSP %s interface.\n", name, strings.ToLower(name))
	fmt.Fprintf(&buf, "type %s interface {\n", name)

	for _, key := range keys {
		info := methods.get(key)

		// Add documentation comment
		if info.documentation != "" {
			for line := range strings.SplitSeq(info.documentation, "\n") {
				fmt.Fprintf(&buf, "\t// %s\n", line)
			}
		}

		// Generate method signature
		if info.isNotification {
			// Notifications: MethodName(context.Context, *ParamsType) error
			// or MethodName(context.Context) error
			if info.paramsType != "" {
				fmt.Fprintf(&buf, "\t%s(context.Context, %s) error\n", info.name, info.paramsType)
			} else {
				fmt.Fprintf(&buf, "\t%s(context.Context) error\n", info.name)
			}
		} else {
			// Requests: MethodName(context.Context, *ParamsType) (*ResultType, error)
			// or MethodName(context.Context) (*ResultType, error)
			if info.paramsType != "" {
				fmt.Fprintf(&buf, "\t%s(context.Context, %s) (%s, error)\n", info.name, info.paramsType, info.resultType)
			} else {
				fmt.Fprintf(&buf, "\t%s(context.Context) (%s, error)\n", info.name, info.resultType)
			}
		}
	}

	buf.WriteString("}\n\n")
	return buf.String()
}

// generateInterfaces generates all interface definitions (Server, Client, and method constants).
func (g *Generator) generateInterfaces() string {
	var buf bytes.Buffer

	// Generate method constants first
	buf.WriteString(g.generateMethodConstants())

	// Generate Server interface
	if g.config.GenerateServer {
		buf.WriteString(g.generateInterface("Server", g.serverMethods))
	}

	// Generate Client interface
	if g.config.GenerateClient {
		buf.WriteString(g.generateInterface("Client", g.clientMethods))
	}

	return buf.String()
}

// resolveTransitiveDeps expands the type filter to include all referenced types.
// This ensures that when filtering for a type like "Range", all types it depends
// on (like "Position") are also included in the generated output.
func (g *Generator) resolveTransitiveDeps() {
	if g.typeFilter == nil || !g.config.ResolveDeps {
		return
	}

	expanded := make(map[string]bool)
	for name := range g.typeFilter {
		g.collectDeps(name, expanded)
	}
	g.typeFilter = expanded
}

// collectDeps recursively collects all types referenced by typeName.
func (g *Generator) collectDeps(typeName string, visited map[string]bool) {
	if visited[typeName] {
		return // Already processed or cycle
	}
	visited[typeName] = true

	// Check structures
	for _, s := range g.model.Structures {
		if s.Name == typeName {
			for _, prop := range s.Properties {
				// Skip proposed properties when not including proposed types
				if prop.Proposed && !g.config.IncludeProposed {
					continue
				}
				g.collectTypeRefs(prop.Type, visited)
			}
			// Also check extends and mixins
			for _, ext := range s.Extends {
				g.collectTypeRefs(ext, visited)
			}
			for _, mix := range s.Mixins {
				g.collectTypeRefs(mix, visited)
			}
			return
		}
	}

	// Check type aliases
	for _, a := range g.model.TypeAliases {
		if a.Name == typeName {
			g.collectTypeRefs(a.Type, visited)
			return
		}
	}

	// Enums don't reference other types, nothing to do
}

// collectTypeRefs extracts type references from a Type and recursively
// collects their dependencies.
func (g *Generator) collectTypeRefs(t *model.Type, visited map[string]bool) {
	if t == nil {
		return
	}
	switch t.Kind {
	case "reference":
		g.collectDeps(t.Name, visited)
	case "array":
		g.collectTypeRefs(t.Element, visited)
	case "map":
		g.collectTypeRefs(t.Key, visited)
		if vt, ok := t.Value.(*model.Type); ok {
			g.collectTypeRefs(vt, visited)
		}
	case "or":
		for _, item := range t.Items {
			g.collectTypeRefs(item, visited)
		}
	case "and":
		for _, item := range t.Items {
			g.collectTypeRefs(item, visited)
		}
	case "tuple":
		for _, item := range t.Items {
			g.collectTypeRefs(item, visited)
		}
	case "literal":
		// Literal types have inline properties
		if lit, ok := t.Value.(model.Literal); ok {
			for _, prop := range lit.Properties {
				g.collectTypeRefs(prop.Type, visited)
			}
		}
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
	sorted := slices.Clone(m.order)
	slices.Sort(sorted)
	return sorted
}
