// SPDX-License-Identifier: MIT
//
// Copyright 2026 Alberto Cavalcante. All rights reserved.
// Use of this source code is governed by a MIT-style license
// that can be found in the LICENSE file.

package generator

import (
	"context"
	"testing"

	"github.com/albertocavalcante/lspls/model"
)

// mockGenerator is a test implementation of Generator.
type mockGenerator struct {
	name string
}

func (m *mockGenerator) Metadata() Metadata {
	return Metadata{
		Name:           m.name,
		Version:        "1.0.0",
		Description:    "Mock generator for testing",
		FileExtensions: []string{".mock"},
	}
}

func (m *mockGenerator) Generate(_ context.Context, _ *model.Model, _ Config) (*Output, error) {
	return Single("test.mock", []byte("mock content")), nil
}

func TestRegistry(t *testing.T) {
	// Reset registry before and after test
	Reset()
	defer Reset()

	t.Run("Register and Get", func(t *testing.T) {
		gen := &mockGenerator{name: "test"}
		Register(gen)

		got, ok := Get("test")
		if !ok {
			t.Fatal("expected to find registered generator")
		}
		if got.Metadata().Name != "test" {
			t.Errorf("got name %q, want %q", got.Metadata().Name, "test")
		}
	})

	t.Run("Get nonexistent", func(t *testing.T) {
		_, ok := Get("nonexistent")
		if ok {
			t.Error("expected not to find nonexistent generator")
		}
	})

	t.Run("List", func(t *testing.T) {
		Reset()
		Register(&mockGenerator{name: "zebra"})
		Register(&mockGenerator{name: "alpha"})

		names := List()
		if len(names) != 2 {
			t.Fatalf("got %d generators, want 2", len(names))
		}
		// Should be sorted
		if names[0] != "alpha" || names[1] != "zebra" {
			t.Errorf("got %v, want [alpha zebra]", names)
		}
	})

	t.Run("All", func(t *testing.T) {
		Reset()
		Register(&mockGenerator{name: "one"})
		Register(&mockGenerator{name: "two"})

		all := All()
		if len(all) != 2 {
			t.Fatalf("got %d generators, want 2", len(all))
		}
	})

	t.Run("Duplicate panics", func(t *testing.T) {
		Reset()
		Register(&mockGenerator{name: "dup"})

		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic on duplicate registration")
			}
		}()
		Register(&mockGenerator{name: "dup"})
	})
}

func TestConfig_Option(t *testing.T) {
	cfg := Config{
		Options: map[string]string{
			"package": "mypackage",
		},
	}

	if got := cfg.Option("package", "default"); got != "mypackage" {
		t.Errorf("got %q, want %q", got, "mypackage")
	}

	if got := cfg.Option("missing", "default"); got != "default" {
		t.Errorf("got %q, want %q", got, "default")
	}
}

func TestOutput(t *testing.T) {
	t.Run("NewOutput and Add", func(t *testing.T) {
		out := NewOutput()
		out.Add("file1.go", []byte("content1"))
		out.Add("file2.go", []byte("content2"))

		if len(out.Files) != 2 {
			t.Fatalf("got %d files, want 2", len(out.Files))
		}
	})

	t.Run("Single", func(t *testing.T) {
		out := Single("only.go", []byte("content"))

		if len(out.Files) != 1 {
			t.Fatalf("got %d files, want 1", len(out.Files))
		}
		if string(out.Files["only.go"]) != "content" {
			t.Error("content mismatch")
		}
	})
}
