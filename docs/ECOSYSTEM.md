# LSP Ecosystem & Background

This document provides context on the LSP type generation landscape, TypeScript-to-Go approaches, and related projects. Useful for contributors, users evaluating options, and anyone interested in the broader ecosystem.

## The Problem

The LSP specification is defined in TypeScript and published as [`metaModel.json`](https://github.com/microsoft/vscode-languageserver-node/blob/main/protocol/metaModel.json). Go projects need these types, but TypeScript and Go have fundamentally different type systems:

| TypeScript       | Go               | Challenge                           |
| ---------------- | ---------------- | ----------------------------------- |
| `A \| B` (union) | No native unions | Need custom types + JSON marshaling |
| `T \| null`      | `*T`             | Must track optionality              |
| Interfaces       | Structs          | Structural vs nominal typing        |
| `any`            | `any` (1.18+)    | Type safety loss                    |

## Existing Go LSP Libraries

| Library                                                                               | Status          | Limitation                                                                                                                                                                                                |
| ------------------------------------------------------------------------------------- | --------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [go.lsp.dev/protocol](https://github.com/go-language-server/protocol)                 | LSP 3.15.3      | Stuck since 2020. [Issue #48](https://github.com/go-language-server/protocol/issues/48) asks about maintenance; [PR #52](https://github.com/go-language-server/protocol/pull/52) for 3.17 is still draft. |
| [gopls internal](https://github.com/golang/tools/tree/master/gopls/internal/protocol) | LSP 3.17+       | Uses `internal/` package - can't import. Has gopls-specific customizations.                                                                                                                               |
| [sourcegraph/go-lsp](https://github.com/sourcegraph/go-lsp)                           | LSP 3.x partial | Manually maintained subset, incomplete.                                                                                                                                                                   |

## TypeScript-to-Go Approaches

### Microsoft's TypeScript-Go Port (Project Corsa)

Microsoft is porting the TypeScript compiler to Go ([typescript-go](https://github.com/microsoft/typescript-go)), targeting TypeScript 7.0 in early 2026. Key learnings from their approach:

- **Hybrid methodology**: Manual port of scanner/parser (~1.5 months), then automated tooling for the rest
- **Data structures by hand**: JS objects (flexible) vs Go structs (rigid memory layouts) required manual conversion
- **Internal tooling not public**: They built conversion tools but haven't released them
- **Why Go over Rust**: Idiomatic Go resembles their functional TS style; GC simplified porting vs Rust's memory management

Their LSP type generation (in `internal/lsp/lsproto/`) also parses `metaModel.json` directly - the same approach as lspls and gopls.

### General TS→Go Tools

| Tool                                                  | Type                  | Notes                                                                      |
| ----------------------------------------------------- | --------------------- | -------------------------------------------------------------------------- |
| [armsnyder/ts2go](https://github.com/armsnyder/ts2go) | Type definitions only | Converts TS interfaces to Go structs. Pre-v1.0, limited union support.     |
| [quicktype](https://github.com/glideapps/quicktype)   | Multi-language        | Generates from JSON/TS/GraphQL. General-purpose, may need post-processing. |
| [leona/ts2go](https://github.com/leona/ts2go)         | Full transpiler       | Experimental, "extremely limited subset" of TS.                            |

**Why these don't fully work for LSP:**

- Union types (`A | B`) need custom JSON marshaling - generic tools output `any`
- LSP has complex nested types, optional fields, and intersection types
- Generated code often needs manual refinement

### The Right Approach: Parse metaModel.json

All mature LSP type generators (gopls, microsoft/typescript-go, lspls) parse `metaModel.json` directly rather than converting TypeScript source:

1. **Official source of truth**: Machine-readable, versioned, stable schema
2. **Avoids TS complexity**: No need to handle TS generics, mapped types, conditional types
3. **Full control**: Generate exactly the Go idioms you want

## Related Projects

| Project                                                                               | What it does                               |
| ------------------------------------------------------------------------------------- | ------------------------------------------ |
| [go.lsp.dev/protocol](https://github.com/go-language-server/protocol)                 | Go LSP types (stuck on 3.15.3)             |
| [gopls](https://github.com/golang/tools/tree/master/gopls)                            | Go language server with internal generator |
| [microsoft/typescript-go](https://github.com/microsoft/typescript-go)                 | TS compiler port to Go                     |
| [vscode-languageserver-node](https://github.com/microsoft/vscode-languageserver-node) | Official LSP spec source                   |

## Further Reading

### gopls (Go Language Server)

- 📝 [Gopls on by default in VS Code Go](https://go.dev/blog/gopls-vscode-go) - Why a single language server was needed (Jan 2021)
- 📝 [Scaling gopls for the growing Go ecosystem](https://go.dev/blog/gopls-scalability) - Memory optimization deep dive, 75% reduction via on-disk indexes (Sep 2023)
- 📝 [gopls Design Document](https://github.com/golang/tools/blob/master/gopls/doc/design/design.md) - Original 2018-2019 design with 2023 retrospective by Rob Findley
- 📝 [Protocol Generator README](https://github.com/golang/tools/blob/master/gopls/internal/protocol/generate/README.md) - How gopls generates LSP types from metaModel.json, type mapping challenges, history
- 🎥 [GopherCon 2019: "Go, pls stop breaking my editor"](https://www.youtube.com/watch?v=EFJfdWzBHwE) - Rebecca Stambler's keynote on why gopls was created (tool fragmentation, 24+ CLI tools in VS Code)

### LSP Specification

- 📝 [LSP 3.17 Specification](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/) - Official human-readable docs
- 📝 [metaModel.json](https://github.com/microsoft/vscode-languageserver-node/blob/main/protocol/metaModel.json) - Machine-readable spec source

### TypeScript-Go Port (Project Corsa)

- 📝 [A 10x Faster TypeScript](https://devblogs.microsoft.com/typescript/typescript-native-port/) - Official announcement by Anders Hejlsberg (March 2025)
- 📝 [Progress on TypeScript 7 - December 2025](https://devblogs.microsoft.com/typescript/progress-on-typescript-7-december-2025/) - Status update with benchmarks
- 📝 [A closer look at the details behind the Go port](https://2ality.com/2025/03/typescript-in-go.html) - Deep dive by Dr. Axel Rauschmayer
- 📝 [Why Go?](https://github.com/microsoft/typescript-go/discussions/411) - Official GitHub discussion on language choice
- 🎥 [TypeScript is being ported to Go](https://www.youtube.com/watch?v=10qowKUW82U) - Interview with Anders Hejlsberg (Michigan TypeScript)
- 🎥 [Anders Hejlsberg on TypeScript's Go Rewrite](https://www.youtube.com/watch?v=NrEW7F2WCNA) - Live interview by Matt Pocock
- 🎧 [TypeScript Just Got 10× Faster](https://syntax.fm/show/884/typescript-just-got-10x-faster) - Syntax podcast with Anders Hejlsberg & Daniel Rosenwasser
- 🎥 [TypeScript Origins: The Documentary](https://www.youtube.com/watch?v=U6s2pdxebSo) - 80 min documentary (OfferZen, 2023)
