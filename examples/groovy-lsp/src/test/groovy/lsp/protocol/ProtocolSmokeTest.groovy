package lsp.protocol

import com.fasterxml.jackson.databind.ObjectMapper
import groovy.transform.CompileStatic
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test

import static org.junit.jupiter.api.Assertions.*

/**
 * Smoke tests verifying that lspls-generated Groovy code compiles,
 * serializes, and deserializes correctly with Jackson.
 */
@CompileStatic
class ProtocolSmokeTest {

    ObjectMapper mapper

    @BeforeEach
    void setUp() {
        mapper = new ObjectMapper()
    }

    // -- Records (structures) -------------------------------------------------

    @Test
    void 'deserialize Position from JSON'() {
        def pos = mapper.readValue('{"line":10,"character":5}', Position)
        assertEquals(10, pos.line())
        assertEquals(5, pos.character())
    }

    @Test
    void 'deserialize Range from JSON with nested Position'() {
        def json = '{"start":{"line":1,"character":0},"end":{"line":1,"character":10}}'
        def range = mapper.readValue(json, Range)
        assertEquals(1, range.start().line())
        assertEquals(0, range.start().character())
        assertEquals(10, range.end().character())
    }

    @Test
    void 'deserialize TextEdit from JSON'() {
        def json = '{"range":{"start":{"line":0,"character":0},"end":{"line":0,"character":3}},"newText":"foo"}'
        def edit = mapper.readValue(json, TextEdit)
        assertEquals('foo', edit.newText())
        assertEquals(0, edit.range().start().line())
        assertEquals(3, edit.range().end().character())
    }

    @Test
    void 'deserialize TextDocumentIdentifier from JSON'() {
        def doc = mapper.readValue('{"uri":"file:///tmp/test.txt"}', TextDocumentIdentifier)
        assertEquals('file:///tmp/test.txt', doc.uri())
    }

    @Test
    void 'ignore unknown fields in JSON'() {
        def json = '{"line":1,"character":2,"extraField":"should be ignored"}'
        def pos = mapper.readValue(json, Position)
        assertEquals(1, pos.line())
        assertEquals(2, pos.character())
    }

    @Test
    void 'round-trip Position through JSON'() {
        def original = new Position(3, 7)
        def json = mapper.writeValueAsString(original)
        def parsed = mapper.readValue(json, Position)
        assertEquals(original, parsed)
    }

    @Test
    void 'round-trip Range through JSON'() {
        def original = new Range(new Position(1, 0), new Position(1, 10))
        def json = mapper.writeValueAsString(original)
        def parsed = mapper.readValue(json, Range)
        assertEquals(original, parsed)
    }

    // -- Integer enum ---------------------------------------------------------

    @Test
    void 'deserialize DiagnosticSeverity from integer'() {
        def severity = mapper.readValue('1', DiagnosticSeverity)
        assertEquals(DiagnosticSeverity.ERROR, severity)
    }

    @Test
    void 'serialize DiagnosticSeverity to integer'() {
        def json = mapper.writeValueAsString(DiagnosticSeverity.WARNING)
        assertEquals('2', json)
    }

    @Test
    void 'round-trip all DiagnosticSeverity values'() {
        for (ds in DiagnosticSeverity.values()) {
            def json = mapper.writeValueAsString(ds)
            def parsed = mapper.readValue(json, DiagnosticSeverity)
            assertEquals(ds, parsed)
        }
    }

    // -- String enum ----------------------------------------------------------

    @Test
    void 'deserialize MarkupKind from string'() {
        def kind = mapper.readValue('"plaintext"', MarkupKind)
        assertEquals(MarkupKind.PLAIN_TEXT, kind)
    }

    @Test
    void 'serialize MarkupKind to string'() {
        def json = mapper.writeValueAsString(MarkupKind.MARKDOWN)
        assertEquals('"markdown"', json)
    }

    @Test
    void 'round-trip MarkupKind values'() {
        for (mk in MarkupKind.values()) {
            def json = mapper.writeValueAsString(mk)
            def parsed = mapper.readValue(json, MarkupKind)
            assertEquals(mk, parsed)
        }
    }
}
