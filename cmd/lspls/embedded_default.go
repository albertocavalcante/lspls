// SPDX-License-Identifier: MIT
//
// Copyright 2026 Alberto Cavalcante. All rights reserved.
// Use of this source code is governed by a MIT-style license
// that can be found in the LICENSE file.

//go:build !lspls_full

package main

import (
	"github.com/albertocavalcante/lspls/generator"
	"github.com/albertocavalcante/lspls/generators/golang"
)

func init() {
	// Default build: only Go generator embedded
	generator.Register(golang.NewGenerator())
}
