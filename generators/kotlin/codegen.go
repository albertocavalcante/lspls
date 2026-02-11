// SPDX-License-Identifier: MIT
//
// Copyright 2026 Alberto Cavalcante. All rights reserved.
// Use of this source code is governed by a MIT-style license
// that can be found in the LICENSE file.

// Package kotlin generates Kotlin source code from the LSP specification model.
//
// The generated code uses idiomatic Kotlin patterns:
//   - data class for LSP structures
//   - enum class with explicit values for enumerations
//   - sealed class with @Serializable subtypes for union ("or") types
//   - typealias for LSP type aliases
//   - kotlinx.serialization annotations for JSON round-tripping
package kotlin

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"github.com/albertocavalcante/lspls/generator"
	"github.com/albertocavalcante/lspls/internal/lspbase"
	"github.com/albertocavalcante/lspls/model"
)

// Codegen generates Kotlin source from the LSP model.
type Codegen struct {
	model  *model.Model
	config Config

	types      *orderedMap[string]
	typeFilter map[string]bool

	// sealedTypes tracks generated sealed classes to avoid duplicates.
	sealedTypes *orderedMap[sealedTypeInfo]

	proposedTypes map[string]bool
}

// sealedTypeInfo holds information about a generated sealed class.
type sealedTypeInfo struct {
	name     string              // e.g. "Or_Int_String"
	variants []sealedVariantInfo // sorted variant descriptors
}

// Output contains the generated Kotlin content.
type Output struct {
	Kotlin []byte
}

// New creates a new Kotlin Codegen.
func New(m *model.Model, cfg Config) *Codegen {
	c := &Codegen{
		model:         m,
		config:        cfg,
		types:         newOrderedMap[string](),
		sealedTypes:   newOrderedMap[sealedTypeInfo](),
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

// Generate produces the Kotlin source file.
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

	return &Output{Kotlin: g.emit()}, nil
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

// ── Structure → data class ──────────────────────────────────────────

func (g *Codegen) generateStructure(s *model.Structure) {
	var buf bytes.Buffer

	writeKdoc(&buf, s.Documentation, s.Since, "")

	// Collect properties (including inherited ones from extends/mixins)
	props := g.collectProperties(s)

	if len(props) == 0 {
		// Empty class (no properties)
		fmt.Fprintf(&buf, "@Serializable\n")
		fmt.Fprintf(&buf, "class %s\n", typeName(s.Name))
	} else {
		fmt.Fprintf(&buf, "@Serializable\n")
		fmt.Fprintf(&buf, "data class %s(\n", typeName(s.Name))
		for i, p := range props {
			g.generateProperty(&buf, &p, i == len(props)-1)
		}
		buf.WriteString(")\n")
	}

	g.types.set(s.Name, buf.String())
}

// collectProperties gathers direct properties. Extends/mixins are flattened
// into the data class because Kotlin data classes cannot extend other data classes.
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
	// KDoc for property
	if p.Documentation != "" {
		for line := range strings.SplitSeq(p.Documentation, "\n") {
			fmt.Fprintf(buf, "    // %s\n", line)
		}
	}

	name := fieldName(p.Name)
	kt := g.kotlinType(p.Type, false)

	// Determine if field needs @SerialName (when Kotlin name differs from JSON key)
	jsonName := p.Name
	needsSerialName := name != jsonName

	if needsSerialName {
		fmt.Fprintf(buf, "    @SerialName(%q)\n", jsonName)
	}

	// Optional fields get a default of null and nullable type
	if p.Optional {
		// If the type is already nullable, don't double-up
		if !strings.HasSuffix(kt, "?") {
			kt += "?"
		}
		fmt.Fprintf(buf, "    val %s: %s = null", name, kt)
	} else {
		fmt.Fprintf(buf, "    val %s: %s", name, kt)
	}

	if !last {
		buf.WriteString(",")
	}
	buf.WriteString("\n")
}

// ── Enumeration → enum class ────────────────────────────────────────

func (g *Codegen) generateEnumeration(e *model.Enumeration) {
	var buf bytes.Buffer

	writeKdoc(&buf, e.Documentation, e.Since, "")

	baseType := kotlinBaseType(e.Type)
	isString := baseType == "String"

	// Filter values for proposed
	var values []model.EnumValue
	for _, v := range e.Values {
		if v.Proposed && !g.config.IncludeProposed {
			continue
		}
		values = append(values, v)
	}

	if isString {
		// String enum: use @Serializable enum with @SerialName on each entry
		fmt.Fprintf(&buf, "@Serializable\n")
		fmt.Fprintf(&buf, "enum class %s {\n", typeName(e.Name))
		for i, v := range values {
			if v.Documentation != "" {
				writeIndentedKdoc(&buf, v.Documentation, "    ")
			}
			strVal, _ := v.Value.(string)
			constName := enumConstName(v.Name)
			fmt.Fprintf(&buf, "    @SerialName(%q)\n", strVal)
			fmt.Fprintf(&buf, "    %s", constName)
			if i < len(values)-1 {
				buf.WriteString(",")
			} else {
				buf.WriteString(";")
			}
			buf.WriteString("\n")
		}
		buf.WriteString("}\n")
	} else {
		// Integer enum: enum class with explicit value property
		fmt.Fprintf(&buf, "@Serializable(with = %sSerializer::class)\n", typeName(e.Name))
		fmt.Fprintf(&buf, "enum class %s(val value: %s) {\n", typeName(e.Name), baseType)
		for i, v := range values {
			if v.Documentation != "" {
				writeIndentedKdoc(&buf, v.Documentation, "    ")
			}
			constName := enumConstName(v.Name)
			intVal := formatIntValue(v.Value)
			fmt.Fprintf(&buf, "    %s(%s)", constName, intVal)
			if i < len(values)-1 {
				buf.WriteString(",")
			} else {
				buf.WriteString(";")
			}
			buf.WriteString("\n")
		}

		// Companion object for lookup by value
		buf.WriteString("\n")
		fmt.Fprintf(&buf, "    companion object {\n")
		fmt.Fprintf(&buf, "        fun fromValue(value: %s): %s =\n", baseType, typeName(e.Name))
		fmt.Fprintf(&buf, "            entries.first { it.value == value }\n")
		fmt.Fprintf(&buf, "    }\n")
		buf.WriteString("}\n")

		// Custom serializer for integer enums
		buf.WriteString("\n")
		g.generateIntEnumSerializer(&buf, e, baseType)
	}

	g.types.set(e.Name, buf.String())
}

func (g *Codegen) generateIntEnumSerializer(buf *bytes.Buffer, e *model.Enumeration, baseType string) {
	name := typeName(e.Name)
	serializerType := baseType + ".serializer()"

	fmt.Fprintf(buf, "object %sSerializer : KSerializer<%s> {\n", name, name)
	fmt.Fprintf(buf, "    override val descriptor: SerialDescriptor = %s.descriptor\n", serializerType)
	fmt.Fprintf(buf, "    override fun serialize(encoder: Encoder, value: %s) {\n", name)
	fmt.Fprintf(buf, "        encoder.encode%s(value.value)\n", baseType)
	fmt.Fprintf(buf, "    }\n")
	fmt.Fprintf(buf, "    override fun deserialize(decoder: Decoder): %s {\n", name)
	fmt.Fprintf(buf, "        val value = decoder.decode%s()\n", baseType)
	fmt.Fprintf(buf, "        return %s.fromValue(value)\n", name)
	fmt.Fprintf(buf, "    }\n")
	fmt.Fprintf(buf, "}\n")
}

// ── Type alias → typealias ──────────────────────────────────────────

func (g *Codegen) generateTypeAlias(a *model.TypeAlias) {
	var buf bytes.Buffer

	writeKdoc(&buf, a.Documentation, a.Since, a.Deprecated)

	kt := g.kotlinType(a.Type, false)
	fmt.Fprintf(&buf, "typealias %s = %s\n", typeName(a.Name), kt)

	g.types.set(a.Name, buf.String())
}

// ── Sealed classes for union types ──────────────────────────────────

func (g *Codegen) generateSealedTypes() string {
	var buf bytes.Buffer

	keys := g.sealedTypes.keys()
	for _, name := range keys {
		info := g.sealedTypes.get(name)
		g.generateSealedType(&buf, info)
	}

	return buf.String()
}

func (g *Codegen) generateSealedType(buf *bytes.Buffer, info sealedTypeInfo) {
	// Doc comment listing the union members
	memberTypes := make([]string, 0, len(info.variants))
	for _, v := range info.variants {
		memberTypes = append(memberTypes, v.kotlinType)
	}
	fmt.Fprintf(buf, "/**\n * Union type: %s\n */\n", strings.Join(memberTypes, " | "))

	fmt.Fprintf(buf, "@Serializable(with = %sSerializer::class)\n", info.name)
	fmt.Fprintf(buf, "sealed class %s {\n", info.name)

	for _, v := range info.variants {
		fmt.Fprintf(buf, "    @Serializable\n")
		fmt.Fprintf(buf, "    data class %sValue(val value: %s) : %s()\n", v.identName, v.kotlinType, info.name)
	}

	buf.WriteString("}\n\n")

	// Serializer: tries each variant in order
	g.generateSealedSerializer(buf, info)
}

func (g *Codegen) generateSealedSerializer(buf *bytes.Buffer, info sealedTypeInfo) {
	fmt.Fprintf(buf, "object %sSerializer : JsonContentPolymorphicSerializer<%s>(%s::class) {\n",
		info.name, info.name, info.name)
	fmt.Fprintf(buf, "    override fun selectDeserializer(element: JsonElement): DeserializationStrategy<%s> {\n",
		info.name)

	// Build discrimination logic based on JSON element type
	// For base-type unions (e.g. Int | String) we check the JSON primitive kind.
	// For reference-type unions (e.g. TextEdit | AnnotatedTextEdit) we try object shape.
	hasObject := false
	hasArray := false
	hasPrimitive := false
	for _, v := range info.variants {
		switch {
		case isPrimitiveKotlinType(v.kotlinType):
			hasPrimitive = true
		case strings.HasPrefix(v.kotlinType, "List<"):
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

func (g *Codegen) generatePrimitiveDiscrimination(buf *bytes.Buffer, info sealedTypeInfo) {
	buf.WriteString("        return when {\n")
	for _, v := range info.variants {
		switch v.kotlinType {
		case "Int", "UInt":
			fmt.Fprintf(buf, "            element is JsonPrimitive && element.intOrNull != null ->\n")
			fmt.Fprintf(buf, "                %s.%sValue.serializer()\n", info.name, v.identName)
		case "Boolean":
			fmt.Fprintf(buf, "            element is JsonPrimitive && element.booleanOrNull != null ->\n")
			fmt.Fprintf(buf, "                %s.%sValue.serializer()\n", info.name, v.identName)
		case "Double":
			fmt.Fprintf(buf, "            element is JsonPrimitive && element.doubleOrNull != null ->\n")
			fmt.Fprintf(buf, "                %s.%sValue.serializer()\n", info.name, v.identName)
		default: // String and string-like
			fmt.Fprintf(buf, "            element is JsonPrimitive && element.isString ->\n")
			fmt.Fprintf(buf, "                %s.%sValue.serializer()\n", info.name, v.identName)
		}
	}
	fmt.Fprintf(buf, "            else -> %s.%sValue.serializer()\n", info.name, info.variants[0].identName)
	buf.WriteString("        }\n")
}

func (g *Codegen) generateObjectDiscrimination(buf *bytes.Buffer, info sealedTypeInfo) {
	// For multiple object types, return the first variant as default.
	// Full field-based discrimination would require knowing the object schemas
	// which would add significant complexity for marginal benefit —
	// the user's deserializer can handle mismatches at runtime.
	fmt.Fprintf(buf, "        return %s.%sValue.serializer()\n",
		info.name, info.variants[0].identName)
}

func (g *Codegen) generateMixedDiscrimination(buf *bytes.Buffer, info sealedTypeInfo) {
	buf.WriteString("        return when (element) {\n")

	for _, v := range info.variants {
		switch {
		case strings.HasPrefix(v.kotlinType, "List<"):
			fmt.Fprintf(buf, "            is JsonArray -> %s.%sValue.serializer()\n",
				info.name, v.identName)
		case isPrimitiveKotlinType(v.kotlinType):
			fmt.Fprintf(buf, "            is JsonPrimitive -> %s.%sValue.serializer()\n",
				info.name, v.identName)
		default:
			fmt.Fprintf(buf, "            is JsonObject -> %s.%sValue.serializer()\n",
				info.name, v.identName)
		}
	}

	fmt.Fprintf(buf, "            else -> %s.%sValue.serializer()\n", info.name, info.variants[0].identName)
	buf.WriteString("        }\n")
}

// ── Emit final file ─────────────────────────────────────────────────

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

	// Sealed classes for union types
	buf.WriteString(g.generateSealedTypes())

	return buf.Bytes()
}

func (g *Codegen) collectImports() []string {
	var imports []string

	// Always need @Serializable
	hasTypes := len(g.types.keys()) > 0
	if hasTypes {
		imports = append(imports, "kotlinx.serialization.Serializable")
	}

	// Check if any property needs @SerialName
	needsSerialName := false
	for _, s := range g.model.Structures {
		if !g.shouldInclude(s.Name, s.Proposed) {
			continue
		}
		for _, p := range g.collectProperties(s) {
			if fieldName(p.Name) != p.Name {
				needsSerialName = true
				break
			}
		}
		if needsSerialName {
			break
		}
	}
	// String enums also use @SerialName
	for _, e := range g.model.Enumerations {
		if !g.shouldInclude(e.Name, e.Proposed) {
			continue
		}
		if e.Type != nil && e.Type.Name == lspbase.TypeString {
			needsSerialName = true
			break
		}
	}
	if needsSerialName {
		imports = append(imports, "kotlinx.serialization.SerialName")
	}

	// Check if any integer enum exists (needs KSerializer etc.)
	hasIntEnum := false
	for _, e := range g.model.Enumerations {
		if !g.shouldInclude(e.Name, e.Proposed) {
			continue
		}
		if e.Type != nil && e.Type.Name != lspbase.TypeString {
			hasIntEnum = true
			break
		}
	}
	if hasIntEnum {
		imports = append(imports,
			"kotlinx.serialization.KSerializer",
			"kotlinx.serialization.descriptors.SerialDescriptor",
			"kotlinx.serialization.encoding.Decoder",
			"kotlinx.serialization.encoding.Encoder",
		)
	}

	// Check if sealed types exist (need JsonContentPolymorphicSerializer etc.)
	if len(g.sealedTypes.keys()) > 0 {
		imports = append(imports,
			"kotlinx.serialization.DeserializationStrategy",
			"kotlinx.serialization.json.JsonContentPolymorphicSerializer",
			"kotlinx.serialization.json.JsonElement",
		)

		// Determine which JSON element types are needed
		needsPrimitive := false
		needsArray := false
		needsObject := false
		for _, name := range g.sealedTypes.keys() {
			info := g.sealedTypes.get(name)
			for _, v := range info.variants {
				switch {
				case isPrimitiveKotlinType(v.kotlinType):
					needsPrimitive = true
				case strings.HasPrefix(v.kotlinType, "List<"):
					needsArray = true
				default:
					needsObject = true
				}
			}
		}
		if needsPrimitive {
			imports = append(imports,
				"kotlinx.serialization.json.JsonPrimitive",
				"kotlinx.serialization.json.intOrNull",
			)
		}
		if needsArray {
			imports = append(imports, "kotlinx.serialization.json.JsonArray")
		}
		if needsObject {
			imports = append(imports, "kotlinx.serialization.json.JsonObject")
		}
	}

	slices.Sort(imports)
	return imports
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

// ── Helpers ─────────────────────────────────────────────────────────

func writeKdoc(buf *bytes.Buffer, doc, since, deprecated string) {
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

func writeIndentedKdoc(buf *bytes.Buffer, doc, indent string) {
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

func isPrimitiveKotlinType(t string) bool {
	switch t {
	case "String", "Int", "UInt", "Double", "Boolean":
		return true
	}
	return false
}
