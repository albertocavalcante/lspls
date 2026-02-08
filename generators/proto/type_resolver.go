// SPDX-License-Identifier: MIT
//
// Copyright 2026 Alberto Cavalcante. All rights reserved.
// Use of this source code is governed by a MIT-style license
// that can be found in the LICENSE file.

package proto

import (
	"iter"
	"strings"

	"github.com/albertocavalcante/lspls/model"
)

// TypeResolver handles type lookups and validations for Proto generation.
// It maintains a registry of known types and maps LSP types to Proto types.
type TypeResolver struct {
	// Maps LSP type names (definitions/aliases) to Proto types
	typeMap map[string]string

	// Set of types defined in the model (to validate references)
	definedTypes map[string]bool
}

// NewTypeResolver creates a new TypeResolver populated from the model.
func NewTypeResolver(m *model.Model, includeProposed bool, overrides map[string]string) *TypeResolver {
	r := &TypeResolver{
		typeMap:      make(map[string]string),
		definedTypes: make(map[string]bool),
	}

	r.init(m, includeProposed, overrides)
	return r
}

// init populates the resolver with types from the model.
func (r *TypeResolver) init(m *model.Model, includeProposed bool, overrides map[string]string) {
	// Add well-known proto types
	r.definedTypes["google.protobuf.Any"] = true
	r.definedTypes["google.protobuf.Value"] = true
	r.definedTypes["google.protobuf.Struct"] = true
	r.definedTypes["google.protobuf.ListValue"] = true

	// Add predefined default mappings
	for k, v := range DefaultMappings {
		r.typeMap[k] = v
	}

	// Apply overrides
	for k, v := range overrides {
		r.typeMap[k] = v
	}

	// 1. Register all defined types (Structures & Enums)
	for _, s := range m.Structures {
		if !includeProposed && s.Proposed {
			continue
		}
		r.definedTypes[s.Name] = true
	}

	for _, e := range m.Enumerations {
		if !includeProposed && e.Proposed {
			continue
		}
		r.definedTypes[e.Name] = true
	}

	// 2. Process Type Aliases
	for _, a := range m.TypeAliases {
		if !includeProposed && a.Proposed {
			continue
		}
		r.definedTypes[a.Name] = true
		r.registerTypeAlias(a)
	}
}

func (r *TypeResolver) registerTypeAlias(a *model.TypeAlias) {
	// Skip if already mapped (predefined)
	if _, exists := r.typeMap[a.Name]; exists {
		return
	}

	if a.Type == nil {
		return
	}

	switch a.Type.Kind {
	case "base":
		if protoType, err := convertBaseType(a.Type.Name); err == nil {
			r.typeMap[a.Name] = protoType
		}

	case "reference":
		r.typeMap[a.Name] = toProtoMessageName(a.Type.Name)

	case "array":
		if a.Type.Element != nil && a.Type.Element.Kind == "reference" {
			r.typeMap[a.Name] = "repeated " + toProtoMessageName(a.Type.Element.Name)
		}
	}
}

// IsMapped checks if a type alias has a manual mapping.
func (r *TypeResolver) IsMapped(lspType string) bool {
	_, exists := r.typeMap[lspType]
	return exists
}

// Resolve returns the Proto type for a given LSP type name.
// If no mapping exists, it returns the name formatted as a Proto message.
func (r *TypeResolver) Resolve(lspType string) string {
	if protoType, exists := r.typeMap[lspType]; exists {
		return protoType
	}
	// If it's a known union/structure, return the message name
	return toProtoMessageName(lspType)
}

// IsKnown checks if a type is known (defined in model, well-known, or scalar).
func (r *TypeResolver) IsKnown(typeName string) bool {
	// Scalar types are always known
	switch typeName {
	case "string", "int32", "int64", "uint32", "uint64", "bool", "float", "double", "bytes":
		return true
	}

	// Well-known proto types
	if strings.HasPrefix(typeName, "google.protobuf.") {
		return true
	}

	// Check defined types set
	return r.definedTypes[typeName]
}

// DefinedTypes returns an iterator over all defined types.
func (r *TypeResolver) DefinedTypes() iter.Seq[string] {
	return func(yield func(string) bool) {
		for t := range r.definedTypes {
			if !yield(t) {
				return
			}
		}
	}
}
