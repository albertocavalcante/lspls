// SPDX-License-Identifier: MIT AND BSD-3-Clause

package golang

import (
	"bytes"
	"fmt"
	"strings"
	"unicode"
)

// methodToGoName converts an LSP method name to a Go method name.
// Examples:
//   - "textDocument/hover" -> "TextDocumentHover"
//   - "$/cancelRequest" -> "CancelRequest"
//   - "initialize" -> "Initialize"
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
