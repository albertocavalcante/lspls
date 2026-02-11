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

// Package golang generates Go source code from the LSP specification model.
package golang

import (
	"bytes"
	"fmt"
	"go/format"
	"strings"

	"github.com/albertocavalcante/lspls/generator"
	"github.com/albertocavalcante/lspls/internal/lspbase"
	"github.com/albertocavalcante/lspls/model"
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
	var items []lspbase.NamedProposal
	for _, s := range m.Structures {
		items = append(items, lspbase.NamedProposal{Name: s.Name, Proposed: s.Proposed})
	}
	for _, e := range m.Enumerations {
		items = append(items, lspbase.NamedProposal{Name: e.Name, Proposed: e.Proposed})
	}
	for _, a := range m.TypeAliases {
		items = append(items, lspbase.NamedProposal{Name: a.Name, Proposed: a.Proposed})
	}
	return lspbase.ProposedTypes(items...)
}

// Generate produces all output files.
func (g *Generator) Generate() (*Output, error) {
	// Resolve transitive dependencies if filtering
	if g.typeFilter != nil && g.config.ResolveDeps {
		g.typeFilter = generator.ResolveDeps(g.model, g.typeFilter, g.config.IncludeProposed)
	}

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
