// Package model defines the data structures for parsing the LSP metaModel.json specification.
//
// The LSP specification is published as a JSON file (metaModel.json) that describes
// all requests, notifications, structures, enumerations, and type aliases in the protocol.
//
// This package provides Go types that map directly to the JSON schema, enabling
// parsing and processing of the specification for code generation.
package model

import (
	"encoding/json"
	"fmt"
)

// Model represents the complete LSP specification parsed from metaModel.json.
type Model struct {
	// Version contains metadata about the LSP specification version.
	Version Metadata `json:"metaData"`

	// Requests defines all request methods (client→server or server→client).
	// Each request has a method name, parameters, and result type.
	Requests []*Request `json:"requests"`

	// Notifications defines all notification methods.
	// Unlike requests, notifications don't expect a response.
	Notifications []*Notification `json:"notifications"`

	// Structures defines all named struct-like types in the protocol.
	// Examples: Position, Range, TextDocumentIdentifier, InlayHint
	Structures []*Structure `json:"structures"`

	// Enumerations defines all enum types with named constants.
	// Examples: DiagnosticSeverity, CompletionItemKind, InlayHintKind
	Enumerations []*Enumeration `json:"enumerations"`

	// TypeAliases defines type aliases used throughout the protocol.
	// Examples: DocumentUri, URI, ProgressToken
	TypeAliases []*TypeAlias `json:"typeAliases"`

	// Line is the source line number in metaModel.json (for debugging).
	Line int `json:"line,omitempty"`
}

// Metadata contains version information about the LSP specification.
type Metadata struct {
	// Version is the LSP protocol version (e.g., "3.17.0").
	Version string `json:"version"`
	Line    int    `json:"line,omitempty"`
}

// Request represents an LSP request method definition.
type Request struct {
	// Documentation is the markdown description of this request.
	Documentation string `json:"documentation,omitempty"`

	// ErrorData is the type of additional error data, if any.
	ErrorData *Type `json:"errorData,omitempty"`

	// Direction indicates the message direction: "clientToServer", "serverToClient", or "both".
	Direction string `json:"messageDirection"`

	// Method is the request method name (e.g., "textDocument/hover").
	Method string `json:"method"`

	// Params is the type of the request parameters.
	Params *Type `json:"params,omitempty"`

	// PartialResult is the type for partial results (for streaming responses).
	PartialResult *Type `json:"partialResult,omitempty"`

	// Proposed indicates if this is a proposed (not yet stable) feature.
	Proposed bool `json:"proposed,omitempty"`

	// RegistrationMethod is the method name used for dynamic registration.
	RegistrationMethod string `json:"registrationMethod,omitempty"`

	// RegistrationOptions is the type for registration options.
	RegistrationOptions *Type `json:"registrationOptions,omitempty"`

	// Result is the type of the response result.
	Result *Type `json:"result,omitempty"`

	// Since indicates the LSP version when this request was introduced.
	Since string `json:"since,omitempty"`

	Line int `json:"line,omitempty"`
}

// Notification represents an LSP notification method definition.
type Notification struct {
	Documentation       string `json:"documentation,omitempty"`
	Direction           string `json:"messageDirection"`
	Method              string `json:"method"`
	Params              *Type  `json:"params,omitempty"`
	Proposed            bool   `json:"proposed,omitempty"`
	RegistrationMethod  string `json:"registrationMethod,omitempty"`
	RegistrationOptions *Type  `json:"registrationOptions,omitempty"`
	Since               string `json:"since,omitempty"`
	Line                int    `json:"line,omitempty"`
}

// Structure represents a named struct type in the LSP specification.
type Structure struct {
	// Documentation is the markdown description.
	Documentation string `json:"documentation,omitempty"`

	// Extends lists types this structure extends (inheritance).
	Extends []*Type `json:"extends,omitempty"`

	// Mixins lists types mixed into this structure.
	Mixins []*Type `json:"mixins,omitempty"`

	// Name is the structure name (e.g., "InlayHint", "Position").
	Name string `json:"name"`

	// Properties lists all fields of this structure.
	Properties []Property `json:"properties,omitempty"`

	// Proposed indicates if this is a proposed feature.
	Proposed bool `json:"proposed,omitempty"`

	// Since indicates when this structure was introduced.
	Since string `json:"since,omitempty"`

	Line int `json:"line,omitempty"`
}

// Enumeration represents an enum type with named constants.
type Enumeration struct {
	Documentation string `json:"documentation,omitempty"`
	Name          string `json:"name"`
	Proposed      bool   `json:"proposed,omitempty"`
	Since         string `json:"since,omitempty"`

	// SupportsCustomValues indicates if custom values beyond the defined ones are allowed.
	SupportsCustomValues bool `json:"supportsCustomValues,omitempty"`

	// Type is the underlying type (usually "string", "integer", or "uinteger").
	Type *Type `json:"type"`

	// Values lists all enum members.
	Values []EnumValue `json:"values"`

	Line int `json:"line,omitempty"`
}

// EnumValue represents a single enum member.
type EnumValue struct {
	Documentation string `json:"documentation,omitempty"`
	Name          string `json:"name"`
	Proposed      bool   `json:"proposed,omitempty"`
	Since         string `json:"since,omitempty"`

	// Value is the actual value (string or number).
	Value any `json:"value"`

	Line int `json:"line,omitempty"`
}

// TypeAlias represents a type alias definition.
type TypeAlias struct {
	Documentation string `json:"documentation,omitempty"`
	Deprecated    string `json:"deprecated,omitempty"`
	Name          string `json:"name"`
	Proposed      bool   `json:"proposed,omitempty"`
	Since         string `json:"since,omitempty"`
	Type          *Type  `json:"type"`
	Line          int    `json:"line,omitempty"`
}

// Property represents a field in a structure.
type Property struct {
	Name          string `json:"name"`
	Type          *Type  `json:"type"`
	Optional      bool   `json:"optional,omitempty"`
	Documentation string `json:"documentation,omitempty"`
	Deprecated    string `json:"deprecated,omitempty"`
	Since         string `json:"since,omitempty"`
	Proposed      bool   `json:"proposed,omitempty"`
	Line          int    `json:"line,omitempty"`
}

// Type represents a type reference in the LSP specification.
//
// The Kind field determines which other fields are relevant:
//   - "base": Name contains the base type (string, integer, etc.)
//   - "reference": Name contains the referenced type name
//   - "array": Element contains the element type
//   - "map": Key and Value contain the map types
//   - "literal": Value contains a Literal with properties
//   - "stringLiteral": Value contains the literal string value
//   - "or": Items contains the union member types
//   - "and": Items contains the intersection member types
//   - "tuple": Items contains the tuple element types
type Type struct {
	Kind    string  `json:"kind"`
	Items   []*Type `json:"items,omitempty"`   // for "and", "or", "tuple"
	Element *Type   `json:"element,omitempty"` // for "array"
	Name    string  `json:"name,omitempty"`    // for "base", "reference"
	Key     *Type   `json:"key,omitempty"`     // for "map"
	Value   any     `json:"value,omitempty"`   // for "map", "stringLiteral", "literal"
	Line    int     `json:"line,omitempty"`
}

// Literal represents an anonymous struct type (literal object type).
type Literal struct {
	Properties []Property `json:"properties"`
}

// UnmarshalJSON implements custom unmarshaling for Type.
// This is needed because the "value" field has different types depending on "kind".
func (t *Type) UnmarshalJSON(data []byte) error {
	// First unmarshal the unambiguous fields.
	var raw struct {
		Kind    string  `json:"kind"`
		Items   []*Type `json:"items"`
		Element *Type   `json:"element"`
		Name    string  `json:"name"`
		Key     *Type   `json:"key"`
		Value   any     `json:"value"`
		Line    int     `json:"line"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	*t = Type{
		Kind:    raw.Kind,
		Items:   raw.Items,
		Element: raw.Element,
		Name:    raw.Name,
		Value:   raw.Value,
		Line:    raw.Line,
	}

	// Handle kind-specific "value" field unmarshaling.
	switch raw.Kind {
	case "map":
		var m struct {
			Key   *Type `json:"key"`
			Value *Type `json:"value"`
		}
		if err := json.Unmarshal(data, &m); err != nil {
			return fmt.Errorf("unmarshal map type: %w", err)
		}
		t.Key = m.Key
		t.Value = m.Value

	case "literal":
		var lit struct {
			Value Literal `json:"value"`
		}
		if err := json.Unmarshal(data, &lit); err != nil {
			return fmt.Errorf("unmarshal literal type: %w", err)
		}
		t.Value = lit.Value

	case "base", "reference", "array", "and", "or", "tuple", "stringLiteral":
		// These don't need special handling.

	default:
		return fmt.Errorf("unknown type kind: %q", raw.Kind)
	}

	return nil
}

// IsOptional returns true if this type represents an optional value (T | null).
func (t *Type) IsOptional() bool {
	if t.Kind != "or" || len(t.Items) != 2 {
		return false
	}
	for _, item := range t.Items {
		if item.Kind == "base" && item.Name == "null" {
			return true
		}
	}
	return false
}

// NonNullType returns the non-null component of an optional type.
// Returns nil if this is not an optional type.
func (t *Type) NonNullType() *Type {
	if !t.IsOptional() {
		return nil
	}
	for _, item := range t.Items {
		if !(item.Kind == "base" && item.Name == "null") {
			return item
		}
	}
	return nil
}
