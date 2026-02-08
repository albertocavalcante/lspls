// SPDX-License-Identifier: MIT
//
// Copyright 2026 Alberto Cavalcante. All rights reserved.
// Use of this source code is governed by a MIT-style license
// that can be found in the LICENSE file.

//go:build lspls_full

package main

import (
	"github.com/albertocavalcante/lspls/generator"
	"github.com/albertocavalcante/lspls/generators/golang"
)

func init() {
	// Full build: all generators embedded
	// Additional generators will be added here as they're implemented:
	generator.Register(golang.NewGenerator())
	// generator.Register(proto.NewGenerator())
	// generator.Register(thrift.NewGenerator())
	// generator.Register(kotlin.NewGenerator())
}
