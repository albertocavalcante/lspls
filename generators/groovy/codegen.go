// SPDX-License-Identifier: MIT
//
// Copyright 2026 Alberto Cavalcante. All rights reserved.
// Use of this source code is governed by a MIT-style license
// that can be found in the LICENSE file.

// Package groovy generates Groovy source code from the LSP specification model.
//
// The generated code targets Groovy 5+ and uses idiomatic patterns:
//   - record with @CompileStatic for structures (immutable, built-in value semantics)
//   - enum with @JsonValue/@JsonCreator for enumerations
//   - sealed class with typed subclasses for union ("or") types
//   - @JsonIgnoreProperties(ignoreUnknown = true) for forward-compatible JSON
package groovy

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"github.com/albertocavalcante/lspls/generator"
	"github.com/albertocavalcante/lspls/internal/lspbase"
	"github.com/albertocavalcante/lspls/model"
)

// Codegen generates Groovy source from the LSP model.
type Codegen struct {
	model  *model.Model
	config Config

	types      *orderedMap[string]
	typeFilter map[string]bool

	// unionTypes tracks generated union wrapper classes to avoid duplicates.
	unionTypes *orderedMap[unionTypeInfo]

	proposedTypes map[string]bool
}

// unionTypeInfo holds information about a generated union wrapper class.
type unionTypeInfo struct {
	name     string             // e.g. "Or_Integer_String"
	variants []unionVariantInfo // sorted variant descriptors
}

// Output contains the generated Groovy content.
type Output struct {
	Groovy []byte
}

// New creates a new Groovy Codegen.
func New(m *model.Model, cfg Config) *Codegen {
	c := &Codegen{
		model:         m,
		config:        cfg,
		types:         newOrderedMap[string](),
		unionTypes:    newOrderedMap[unionTypeInfo](),
		proposedTypes: buildProposedCache(m),
	}
	if len(cfg.Types) > 0 {
		c.typeFilter = make(map[string]bool)
		for _, t := range cfg.Types {
			c.typeFilter[t] = true
		}
	}
	return c
}

func buildProposedCache(m *model.Model) map[string]bool {
	items := make([]lspbase.NamedProposal, 0, len(m.Structures)+len(m.Enumerations)+len(m.TypeAliases))
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

// Generate produces the Groovy source file.
func (g *Codegen) Generate() (*Output, error) {
	if g.typeFilter != nil && g.config.ResolveDeps {
		g.typeFilter = generator.ResolveDeps(g.model, g.typeFilter, g.config.IncludeProposed)
	}

	for _, s := range g.model.Structures {
		if !g.shouldInclude(s.Name, s.Proposed) {
			continue
		}
		g.generateStructure(s)
	}

	for _, e := range g.model.Enumerations {
		if !g.shouldInclude(e.Name, e.Proposed) {
			continue
		}
		g.generateEnumeration(e)
	}

	for _, a := range g.model.TypeAliases {
		if !g.shouldInclude(a.Name, a.Proposed) {
			continue
		}
		g.generateTypeAlias(a)
	}

	return &Output{Groovy: g.emit()}, nil
}

func (g *Codegen) shouldInclude(name string, proposed bool) bool {
	if proposed && !g.config.IncludeProposed {
		return false
	}
	if g.typeFilter != nil && !g.typeFilter[name] {
		return false
	}
	return true
}

func (g *Codegen) isProposed(name string) bool {
	return g.proposedTypes[name]
}

// -- Structure -> record with @CompileStatic ----------------------------------

func (g *Codegen) generateStructure(s *model.Structure) {
	var buf bytes.Buffer

	writeGroovydoc(&buf, s.Documentation, s.Since, "")

	// Collect properties (including inherited ones from extends/mixins)
	props := g.collectProperties(s)

	fmt.Fprintf(&buf, "@CompileStatic\n")
	fmt.Fprintf(&buf, "@JsonIgnoreProperties(ignoreUnknown = true)\n")

	if len(props) == 0 {
		fmt.Fprintf(&buf, "record %s() {}\n", typeName(s.Name))
	} else {
		fmt.Fprintf(&buf, "record %s(\n", typeName(s.Name))
		for i, p := range props {
			g.generateProperty(&buf, &p, i == len(props)-1)
		}
		buf.WriteString(") {}\n")
	}

	g.types.set(s.Name, buf.String())
}

// collectProperties gathers direct properties. Extends/mixins are flattened
// into the record because Groovy records don't support multiple inheritance.
func (g *Codegen) collectProperties(s *model.Structure) []model.Property {
	var props []model.Property

	// Flatten extends
	for _, ext := range s.Extends {
		if ext.Kind == "reference" {
			for _, parent := range g.model.Structures {
				if parent.Name == ext.Name {
					props = append(props, g.collectProperties(parent)...)
				}
			}
		}
	}

	// Flatten mixins
	for _, mix := range s.Mixins {
		if mix.Kind == "reference" {
			for _, parent := range g.model.Structures {
				if parent.Name == mix.Name {
					props = append(props, g.collectProperties(parent)...)
				}
			}
		}
	}

	// Own properties (skip proposed when not included)
	for _, p := range s.Properties {
		if p.Proposed && !g.config.IncludeProposed {
			continue
		}
		props = append(props, p)
	}

	return props
}

func (g *Codegen) generateProperty(buf *bytes.Buffer, p *model.Property, last bool) {
	// Groovydoc for property
	if p.Documentation != "" {
		for line := range strings.SplitSeq(p.Documentation, "\n") {
			fmt.Fprintf(buf, "    /** %s */\n", line)
		}
	}

	name := fieldName(p.Name)
	gt := g.groovyType(p.Type, false)

	// Determine if field needs @JsonProperty (when Groovy name differs from JSON key)
	jsonName := p.Name
	needsJSONProperty := name != jsonName

	if needsJSONProperty {
		fmt.Fprintf(buf, "    @JsonProperty(%q)\n", jsonName)
	}

	// Optional fields: box primitives and set default to null
	if p.Optional {
		gt = boxPrimitive(gt)
		fmt.Fprintf(buf, "    %s %s = null", gt, name)
	} else {
		fmt.Fprintf(buf, "    %s %s", gt, name)
	}

	if !last {
		buf.WriteString(",")
	}
	buf.WriteString("\n")
}

// -- Enumeration -> enum with Jackson annotations -----------------------------

func (g *Codegen) generateEnumeration(e *model.Enumeration) {
	var buf bytes.Buffer

	writeGroovydoc(&buf, e.Documentation, e.Since, "")

	baseType := groovyBaseType(e.Type)
	isString := baseType == "String"

	// Filter values for proposed
	var values []model.EnumValue
	for _, v := range e.Values {
		if v.Proposed && !g.config.IncludeProposed {
			continue
		}
		values = append(values, v)
	}

	fmt.Fprintf(&buf, "@CompileStatic\n")
	fmt.Fprintf(&buf, "enum %s {\n", typeName(e.Name))

	if isString {
		// String enum with @JsonValue
		for i, v := range values {
			if v.Documentation != "" {
				writeIndentedGroovydoc(&buf, v.Documentation, "    ")
			}
			strVal, _ := v.Value.(string)
			constName := enumConstName(v.Name)
			fmt.Fprintf(&buf, "    %s('%s')", constName, strVal)
			if i < len(values)-1 {
				buf.WriteString(",")
			}
			buf.WriteString("\n")
		}
		buf.WriteString("\n")
		fmt.Fprintf(&buf, "    final String value\n")
		fmt.Fprintf(&buf, "    %s(String value) { this.value = value }\n", typeName(e.Name))
		fmt.Fprintf(&buf, "    @JsonValue\n")
		fmt.Fprintf(&buf, "    String getValue() { value }\n")
	} else {
		// Integer enum with @JsonValue and @JsonCreator
		for i, v := range values {
			if v.Documentation != "" {
				writeIndentedGroovydoc(&buf, v.Documentation, "    ")
			}
			constName := enumConstName(v.Name)
			intVal := formatIntValue(v.Value)
			fmt.Fprintf(&buf, "    %s(%s)", constName, intVal)
			if i < len(values)-1 {
				buf.WriteString(",")
			}
			buf.WriteString("\n")
		}
		buf.WriteString("\n")
		fmt.Fprintf(&buf, "    final int value\n")
		fmt.Fprintf(&buf, "    %s(int value) { this.value = value }\n", typeName(e.Name))
		fmt.Fprintf(&buf, "    @JsonValue\n")
		fmt.Fprintf(&buf, "    int getValue() { value }\n")
		fmt.Fprintf(&buf, "    @JsonCreator\n")
		fmt.Fprintf(&buf, "    static %s fromValue(int value) {\n", typeName(e.Name))
		fmt.Fprintf(&buf, "        values().find { it.value == value }\n")
		fmt.Fprintf(&buf, "    }\n")
	}

	buf.WriteString("}\n")

	g.types.set(e.Name, buf.String())
}

// -- Type alias -> comment (Groovy has no typealias) --------------------------

func (g *Codegen) generateTypeAlias(a *model.TypeAlias) {
	var buf bytes.Buffer

	gt := g.groovyType(a.Type, false)

	writeGroovydoc(&buf, a.Documentation, a.Since, a.Deprecated)
	fmt.Fprintf(&buf, "// Type alias: %s = %s\n", typeName(a.Name), gt)

	g.types.set(a.Name, buf.String())
}

// -- Union sealed classes with Jackson deserializer ---------------------------

func (g *Codegen) generateUnionTypes() string {
	var buf bytes.Buffer

	keys := g.unionTypes.keys()
	for _, name := range keys {
		info := g.unionTypes.get(name)
		g.generateUnionType(&buf, info)
	}

	return buf.String()
}

func (g *Codegen) generateUnionType(buf *bytes.Buffer, info unionTypeInfo) {
	// Doc comment listing the union members
	memberTypes := make([]string, 0, len(info.variants))
	for _, v := range info.variants {
		memberTypes = append(memberTypes, v.groovyType)
	}
	fmt.Fprintf(buf, "/**\n * Union type: %s\n */\n", strings.Join(memberTypes, " | "))

	fmt.Fprintf(buf, "@CompileStatic\n")
	fmt.Fprintf(buf, "@JsonDeserialize(using = %sDeserializer)\n", info.name)
	fmt.Fprintf(buf, "sealed class %s {\n", info.name)
	fmt.Fprintf(buf, "    final Object value\n")
	fmt.Fprintf(buf, "    protected %s(Object value) { this.value = value }\n", info.name)
	fmt.Fprintf(buf, "    @JsonValue\n")
	fmt.Fprintf(buf, "    Object getValue() { value }\n")
	buf.WriteString("\n")

	for _, v := range info.variants {
		fmt.Fprintf(buf, "    static final class %sValue extends %s {\n", v.identName, info.name)
		fmt.Fprintf(buf, "        %sValue(%s value) { super(value) }\n", v.identName, v.groovyType)
		fmt.Fprintf(buf, "    }\n")
	}

	fmt.Fprintf(buf, "}\n\n")

	// Deserializer class
	g.generateUnionDeserializer(buf, info)
}

func (g *Codegen) generateUnionDeserializer(buf *bytes.Buffer, info unionTypeInfo) {
	fmt.Fprintf(buf, "@CompileStatic\n")
	fmt.Fprintf(buf, "class %sDeserializer extends JsonDeserializer<%s> {\n", info.name, info.name)
	fmt.Fprintf(buf, "    @Override\n")
	fmt.Fprintf(buf, "    %s deserialize(JsonParser p, DeserializationContext ctxt) {\n", info.name)
	fmt.Fprintf(buf, "        JsonNode node = p.readValueAsTree()\n")

	// Build discrimination logic based on JSON node type
	hasObject := false
	hasArray := false
	hasPrimitive := false
	for _, v := range info.variants {
		switch {
		case isPrimitiveGroovyType(v.groovyType):
			hasPrimitive = true
		case strings.HasPrefix(v.groovyType, "List<"):
			hasArray = true
		default:
			hasObject = true
		}
	}

	switch {
	case hasPrimitive && !hasObject && !hasArray:
		g.generatePrimitiveDiscrimination(buf, info)
	case hasObject && !hasPrimitive && !hasArray:
		g.generateObjectDiscrimination(buf, info)
	default:
		g.generateMixedDiscrimination(buf, info)
	}

	fmt.Fprintf(buf, "    }\n")
	fmt.Fprintf(buf, "}\n")
}

func (g *Codegen) generatePrimitiveDiscrimination(buf *bytes.Buffer, info unionTypeInfo) {
	for _, v := range info.variants {
		switch v.groovyType {
		case "int", "Integer":
			fmt.Fprintf(buf, "        if (node.isInt()) return new %s.%sValue(node.intValue())\n", info.name, v.identName)
		case "boolean", "Boolean":
			fmt.Fprintf(buf, "        if (node.isBoolean()) return new %s.%sValue(node.booleanValue())\n", info.name, v.identName)
		case "double", "Double":
			fmt.Fprintf(buf, "        if (node.isDouble()) return new %s.%sValue(node.doubleValue())\n", info.name, v.identName)
		default: // String and string-like
			fmt.Fprintf(buf, "        if (node.isTextual()) return new %s.%sValue(node.textValue())\n", info.name, v.identName)
		}
	}
	fmt.Fprintf(buf, "        throw ctxt.weirdStringException(node.toString(), %s, 'Expected %s')\n",
		info.name, strings.Join(variantTypeNames(info), " or "))
}

func (g *Codegen) generateObjectDiscrimination(buf *bytes.Buffer, info unionTypeInfo) {
	// For multiple object types, try each via treeToValue
	for _, v := range info.variants {
		fmt.Fprintf(buf, "        if (node.isObject()) {\n")
		fmt.Fprintf(buf, "            try {\n")
		fmt.Fprintf(buf, "                return new %s.%sValue(p.codec.treeToValue(node, %s))\n", info.name, v.identName, v.groovyType)
		fmt.Fprintf(buf, "            } catch (Exception ignored) {}\n")
		fmt.Fprintf(buf, "        }\n")
	}
	fmt.Fprintf(buf, "        throw ctxt.weirdStringException(node.toString(), %s, 'Expected %s')\n",
		info.name, strings.Join(variantTypeNames(info), " or "))
}

func (g *Codegen) generateMixedDiscrimination(buf *bytes.Buffer, info unionTypeInfo) {
	for _, v := range info.variants {
		switch {
		case strings.HasPrefix(v.groovyType, "List<"):
			// Extract element type from List<T>
			elemType := v.groovyType[len("List<") : len(v.groovyType)-1]
			fmt.Fprintf(buf, "        if (node.isArray()) {\n")
			fmt.Fprintf(buf, "            List<%s> list = []\n", elemType)
			fmt.Fprintf(buf, "            node.each { JsonNode item -> list.add(p.codec.treeToValue(item, %s)) }\n", elemType)
			fmt.Fprintf(buf, "            return new %s.%sValue(list)\n", info.name, v.identName)
			fmt.Fprintf(buf, "        }\n")
		case isPrimitiveGroovyType(v.groovyType):
			switch v.groovyType {
			case "int", "Integer":
				fmt.Fprintf(buf, "        if (node.isInt()) return new %s.%sValue(node.intValue())\n", info.name, v.identName)
			case "boolean", "Boolean":
				fmt.Fprintf(buf, "        if (node.isBoolean()) return new %s.%sValue(node.booleanValue())\n", info.name, v.identName)
			case "double", "Double":
				fmt.Fprintf(buf, "        if (node.isDouble()) return new %s.%sValue(node.doubleValue())\n", info.name, v.identName)
			default:
				fmt.Fprintf(buf, "        if (node.isTextual()) return new %s.%sValue(node.textValue())\n", info.name, v.identName)
			}
		default:
			fmt.Fprintf(buf, "        if (node.isObject()) return new %s.%sValue(p.codec.treeToValue(node, %s))\n",
				info.name, v.identName, v.groovyType)
		}
	}
	fmt.Fprintf(buf, "        throw ctxt.weirdStringException(node.toString(), %s, 'Expected %s')\n",
		info.name, strings.Join(variantTypeNames(info), " or "))
}

// -- Emit final file ----------------------------------------------------------

func (g *Codegen) emit() []byte {
	var buf bytes.Buffer

	buf.WriteString(g.fileHeader())
	fmt.Fprintf(&buf, "package %s\n\n", g.config.PackageName)

	// Collect which imports we need
	imports := g.collectImports()
	if len(imports) > 0 {
		for _, imp := range imports {
			fmt.Fprintf(&buf, "import %s\n", imp)
		}
		buf.WriteString("\n")
	}

	// Types (structures, enums, type aliases) in sorted order
	for _, name := range g.types.keys() {
		buf.WriteString(g.types.get(name))
		buf.WriteString("\n")
	}

	// Union wrapper classes
	buf.WriteString(g.generateUnionTypes())

	return buf.Bytes()
}

func (g *Codegen) collectImports() []string {
	var imports []string

	hasStructures := false
	hasEnums := false
	hasStringEnum := false
	hasIntEnum := false
	hasJSONProperty := false

	for _, s := range g.model.Structures {
		if !g.shouldInclude(s.Name, s.Proposed) {
			continue
		}
		hasStructures = true
		for _, p := range g.collectProperties(s) {
			if fieldName(p.Name) != p.Name {
				hasJSONProperty = true
			}
		}
	}

	for _, e := range g.model.Enumerations {
		if !g.shouldInclude(e.Name, e.Proposed) {
			continue
		}
		hasEnums = true
		if e.Type != nil && e.Type.Name == lspbase.TypeString {
			hasStringEnum = true
		} else {
			hasIntEnum = true
		}
	}

	// Groovy annotations
	if hasStructures || hasEnums {
		imports = append(imports, "groovy.transform.CompileStatic")
	}

	// Jackson imports for structures
	if hasStructures {
		imports = append(imports, "com.fasterxml.jackson.annotation.JsonIgnoreProperties")
	}
	if hasJSONProperty {
		imports = append(imports, "com.fasterxml.jackson.annotation.JsonProperty")
	}

	// Jackson imports for enums
	if hasStringEnum || hasIntEnum {
		imports = append(imports, "com.fasterxml.jackson.annotation.JsonValue")
	}
	if hasIntEnum {
		imports = append(imports, "com.fasterxml.jackson.annotation.JsonCreator")
	}

	// Jackson imports for union types
	if len(g.unionTypes.keys()) > 0 {
		imports = append(imports,
			"com.fasterxml.jackson.core.JsonParser",
			"com.fasterxml.jackson.databind.DeserializationContext",
			"com.fasterxml.jackson.databind.JsonDeserializer",
			"com.fasterxml.jackson.databind.JsonNode",
			"com.fasterxml.jackson.databind.annotation.JsonDeserialize",
		)
		// Wrapper classes also use @CompileStatic and @JsonValue
		if !hasStructures && !hasEnums {
			imports = append(imports, "groovy.transform.CompileStatic")
		}
		if !hasStringEnum && !hasIntEnum {
			imports = append(imports, "com.fasterxml.jackson.annotation.JsonValue")
		}
	}

	slices.Sort(imports)
	return slices.Compact(imports)
}

func (g *Codegen) fileHeader() string {
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

// -- Helpers ------------------------------------------------------------------

func writeGroovydoc(buf *bytes.Buffer, doc, since, deprecated string) {
	if doc == "" && since == "" && deprecated == "" {
		return
	}
	buf.WriteString("/**\n")
	if doc != "" {
		for line := range strings.SplitSeq(doc, "\n") {
			fmt.Fprintf(buf, " * %s\n", line)
		}
	}
	if since != "" && !strings.Contains(doc, "@since "+since) {
		fmt.Fprintf(buf, " *\n * @since %s\n", since)
	}
	if deprecated != "" {
		fmt.Fprintf(buf, " *\n * @deprecated %s\n", deprecated)
	}
	buf.WriteString(" */\n")
}

func writeIndentedGroovydoc(buf *bytes.Buffer, doc, indent string) {
	if doc == "" {
		return
	}
	fmt.Fprintf(buf, "%s/**\n", indent)
	for line := range strings.SplitSeq(doc, "\n") {
		fmt.Fprintf(buf, "%s * %s\n", indent, line)
	}
	fmt.Fprintf(buf, "%s */\n", indent)
}

func formatIntValue(v any) string {
	switch val := v.(type) {
	case float64:
		return fmt.Sprintf("%d", int64(val))
	case int:
		return fmt.Sprintf("%d", val)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func variantTypeNames(info unionTypeInfo) []string {
	names := make([]string, 0, len(info.variants))
	for _, v := range info.variants {
		names = append(names, v.groovyType)
	}
	return names
}
