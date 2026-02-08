// SPDX-License-Identifier: MIT
//
// Copyright 2026 Alberto Cavalcante. All rights reserved.
// Use of this source code is governed by a MIT-style license
// that can be found in the LICENSE file.

package generator

// Output contains generated files.
type Output struct {
	// Files maps filename to content.
	Files map[string][]byte
}

// NewOutput creates a new Output.
func NewOutput() *Output {
	return &Output{Files: make(map[string][]byte)}
}

// Add adds a file to the output.
func (o *Output) Add(name string, content []byte) {
	o.Files[name] = content
}

// Single returns an Output with a single file.
func Single(name string, content []byte) *Output {
	return &Output{Files: map[string][]byte{name: content}}
}
