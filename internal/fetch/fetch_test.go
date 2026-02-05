// SPDX-License-Identifier: MIT
//
// Copyright 2026 Alberto Cavalcante. All rights reserved.
// Use of this source code is governed by a MIT-style license
// that can be found in the LICENSE file.

package fetch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsHex(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "empty string",
			input: "",
			want:  true, // all characters (none) are hex
		},
		{
			name:  "valid lowercase hex",
			input: "0123456789abcdef",
			want:  true,
		},
		{
			name:  "valid uppercase hex",
			input: "0123456789ABCDEF",
			want:  true,
		},
		{
			name:  "valid mixed case hex",
			input: "aAbBcCdDeEfF",
			want:  true,
		},
		{
			name:  "valid 40-char git hash",
			input: "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
			want:  true,
		},
		{
			name:  "invalid char g",
			input: "abcdefg",
			want:  false,
		},
		{
			name:  "invalid char z",
			input: "123z456",
			want:  false,
		},
		{
			name:  "invalid char !",
			input: "abc!def",
			want:  false,
		},
		{
			name:  "invalid char space",
			input: "abc def",
			want:  false,
		},
		{
			name:  "mixed valid and invalid at start",
			input: "gabc123",
			want:  false,
		},
		{
			name:  "mixed valid and invalid at end",
			input: "abc123g",
			want:  false,
		},
		{
			name:  "only digits",
			input: "0123456789",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isHex(tt.input)
			if got != tt.want {
				t.Errorf("isHex(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestInjectLineNumbers(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty input",
			input: "",
			want:  "",
		},
		{
			name:  "simple object with newline",
			input: "{\n\"key\": \"value\"\n}",
			want:  "{\"line\":1,\n\"key\": \"value\"\n}",
		},
		{
			name:  "nested objects with newlines",
			input: "{\n\"outer\": {\n\"inner\": 1\n}\n}",
			want:  "{\"line\":1,\n\"outer\": {\"line\":2,\n\"inner\": 1\n}\n}",
		},
		{
			name:  "inline object no newline",
			input: "{\"key\": {\"nested\": 1}}",
			want:  "{\"key\": {\"nested\": 1}}",
		},
		{
			name:  "array with objects",
			input: "[\n{\n\"a\": 1\n},\n{\n\"b\": 2\n}\n]",
			want:  "[\n{\"line\":2,\n\"a\": 1\n},\n{\"line\":5,\n\"b\": 2\n}\n]",
		},
		{
			name:  "no objects",
			input: "\"just a string\"",
			want:  "\"just a string\"",
		},
		{
			name:  "multiple newlines before object",
			input: "\n\n\n{\n\"key\": 1\n}",
			want:  "\n\n\n{\"line\":4,\n\"key\": 1\n}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(injectLineNumbers([]byte(tt.input)))
			if got != tt.want {
				t.Errorf("injectLineNumbers(%q) =\n%q\nwant:\n%q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseModel(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		check   func(t *testing.T, input string) // custom validation
	}{
		{
			name: "valid minimal model",
			input: `{
"metaData": {"version": "3.17.0"},
"requests": [],
"notifications": [],
"structures": [],
"enumerations": [],
"typeAliases": []
}`,
			wantErr: false,
			check: func(t *testing.T, input string) {
				m, err := parseModel([]byte(input))
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if m.Version.Version != "3.17.0" {
					t.Errorf("version = %q, want %q", m.Version.Version, "3.17.0")
				}
			},
		},
		{
			name: "valid model with request",
			input: `{
"metaData": {"version": "3.17.0"},
"requests": [{"method": "initialize", "messageDirection": "clientToServer"}],
"notifications": [],
"structures": [],
"enumerations": [],
"typeAliases": []
}`,
			wantErr: false,
			check: func(t *testing.T, input string) {
				m, err := parseModel([]byte(input))
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if len(m.Requests) != 1 {
					t.Fatalf("expected 1 request, got %d", len(m.Requests))
				}
				if m.Requests[0].Method != "initialize" {
					t.Errorf("method = %q, want %q", m.Requests[0].Method, "initialize")
				}
			},
		},
		{
			name: "valid model with structure",
			input: `{
"metaData": {"version": "3.17.0"},
"requests": [],
"notifications": [],
"structures": [{"name": "Position", "properties": [{"name": "line", "type": {"kind": "base", "name": "uinteger"}}]}],
"enumerations": [],
"typeAliases": []
}`,
			wantErr: false,
			check: func(t *testing.T, input string) {
				m, err := parseModel([]byte(input))
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if len(m.Structures) != 1 {
					t.Fatalf("expected 1 structure, got %d", len(m.Structures))
				}
				if m.Structures[0].Name != "Position" {
					t.Errorf("name = %q, want %q", m.Structures[0].Name, "Position")
				}
			},
		},
		{
			name:    "invalid JSON",
			input:   `{invalid json}`,
			wantErr: true,
		},
		{
			name:    "empty JSON object",
			input:   `{}`,
			wantErr: false,
		},
		{
			name:    "malformed JSON missing closing brace",
			input:   `{"metaData": {"version": "3.17.0"}`,
			wantErr: true,
		},
		{
			name: "invalid type kind",
			input: `{
"metaData": {"version": "3.17.0"},
"requests": [],
"notifications": [],
"structures": [{"name": "Test", "properties": [{"name": "field", "type": {"kind": "unknownKind"}}]}],
"enumerations": [],
"typeAliases": []
}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseModel([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("parseModel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.check != nil && err == nil {
				tt.check(t, tt.input)
			}
		})
	}
}

func TestFetchFromFile(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(dir string) string // returns file path
		wantErr     bool
		wantSource  string // expected source prefix
		checkResult func(t *testing.T, result *Result)
	}{
		{
			name: "valid model file",
			setup: func(dir string) string {
				content := `{
"metaData": {"version": "3.17.0"},
"requests": [],
"notifications": [],
"structures": [],
"enumerations": [],
"typeAliases": []
}`
				path := filepath.Join(dir, "metaModel.json")
				if err := os.WriteFile(path, []byte(content), 0644); err != nil {
					t.Fatalf("failed to write test file: %v", err)
				}
				return path
			},
			wantErr:    false,
			wantSource: "file://",
			checkResult: func(t *testing.T, result *Result) {
				if result.Model == nil {
					t.Fatal("expected non-nil Model")
				}
				if result.Model.Version.Version != "3.17.0" {
					t.Errorf("version = %q, want %q", result.Model.Version.Version, "3.17.0")
				}
				if result.CommitHash != "" {
					t.Errorf("expected empty CommitHash for file source, got %q", result.CommitHash)
				}
				if result.Ref != "" {
					t.Errorf("expected empty Ref for file source, got %q", result.Ref)
				}
			},
		},
		{
			name: "non-existent file",
			setup: func(dir string) string {
				return filepath.Join(dir, "does-not-exist.json")
			},
			wantErr: true,
		},
		{
			name: "invalid JSON file",
			setup: func(dir string) string {
				content := `{invalid json}`
				path := filepath.Join(dir, "invalid.json")
				if err := os.WriteFile(path, []byte(content), 0644); err != nil {
					t.Fatalf("failed to write test file: %v", err)
				}
				return path
			},
			wantErr: true,
		},
		{
			name: "empty file",
			setup: func(dir string) string {
				path := filepath.Join(dir, "empty.json")
				if err := os.WriteFile(path, []byte(""), 0644); err != nil {
					t.Fatalf("failed to write test file: %v", err)
				}
				return path
			},
			wantErr: true,
		},
		{
			name: "file with complex model",
			setup: func(dir string) string {
				content := `{
"metaData": {"version": "3.17.0"},
"requests": [
	{"method": "initialize", "messageDirection": "clientToServer"},
	{"method": "shutdown", "messageDirection": "clientToServer"}
],
"notifications": [
	{"method": "initialized", "messageDirection": "clientToServer"}
],
"structures": [
	{"name": "Position", "properties": [
		{"name": "line", "type": {"kind": "base", "name": "uinteger"}},
		{"name": "character", "type": {"kind": "base", "name": "uinteger"}}
	]}
],
"enumerations": [
	{"name": "DiagnosticSeverity", "type": {"kind": "base", "name": "uinteger"}, "values": [
		{"name": "Error", "value": 1},
		{"name": "Warning", "value": 2}
	]}
],
"typeAliases": [
	{"name": "DocumentUri", "type": {"kind": "base", "name": "string"}}
]
}`
				path := filepath.Join(dir, "complex.json")
				if err := os.WriteFile(path, []byte(content), 0644); err != nil {
					t.Fatalf("failed to write test file: %v", err)
				}
				return path
			},
			wantErr: false,
			checkResult: func(t *testing.T, result *Result) {
				if result.Model == nil {
					t.Fatal("expected non-nil Model")
				}
				if len(result.Model.Requests) != 2 {
					t.Errorf("requests count = %d, want 2", len(result.Model.Requests))
				}
				if len(result.Model.Notifications) != 1 {
					t.Errorf("notifications count = %d, want 1", len(result.Model.Notifications))
				}
				if len(result.Model.Structures) != 1 {
					t.Errorf("structures count = %d, want 1", len(result.Model.Structures))
				}
				if len(result.Model.Enumerations) != 1 {
					t.Errorf("enumerations count = %d, want 1", len(result.Model.Enumerations))
				}
				if len(result.Model.TypeAliases) != 1 {
					t.Errorf("typeAliases count = %d, want 1", len(result.Model.TypeAliases))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := tt.setup(dir)

			result, err := fetchFromFile(path)
			if (err != nil) != tt.wantErr {
				t.Errorf("fetchFromFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if tt.wantSource != "" && !strings.HasPrefix(result.Source, tt.wantSource) {
				t.Errorf("source = %q, want prefix %q", result.Source, tt.wantSource)
			}
			if tt.wantSource != "" && !strings.HasPrefix(result.Source, tt.wantSource) {
				t.Errorf("source = %q, want prefix %q", result.Source, tt.wantSource)
			}
			if tt.wantSource != "" && !strings.HasPrefix(result.Source, tt.wantSource) {
				t.Errorf("source = %q, want prefix %q", result.Source, tt.wantSource)
			}
			if tt.wantSource != "" && !strings.HasPrefix(result.Source, tt.wantSource) {
				t.Errorf("source = %q, want prefix %q", result.Source, tt.wantSource)
			}
			if tt.wantSource != "" && !strings.HasPrefix(result.Source, tt.wantSource) {
				t.Errorf("source = %q, want prefix %q", result.Source, tt.wantSource)
			}

			if tt.checkResult != nil {
				tt.checkResult(t, result)
			}
		})
	}
}

func TestGetGitHash(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(dir string) // set up the mock git repo
		wantHash string
	}{
		{
			name: "detached HEAD with direct hash",
			setup: func(dir string) {
				gitDir := filepath.Join(dir, ".git")
				if err := os.MkdirAll(gitDir, 0755); err != nil {
					t.Fatalf("failed to create .git dir: %v", err)
				}
				hash := "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
				if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte(hash+"\n"), 0644); err != nil {
					t.Fatalf("failed to write HEAD: %v", err)
				}
			},
			wantHash: "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
		},
		{
			name: "HEAD references branch",
			setup: func(dir string) {
				gitDir := filepath.Join(dir, ".git")
				refsDir := filepath.Join(gitDir, "refs", "heads")
				if err := os.MkdirAll(refsDir, 0755); err != nil {
					t.Fatalf("failed to create refs dir: %v", err)
				}
				if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0644); err != nil {
					t.Fatalf("failed to write HEAD: %v", err)
				}
				hash := "b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3"
				if err := os.WriteFile(filepath.Join(refsDir, "main"), []byte(hash+"\n"), 0644); err != nil {
					t.Fatalf("failed to write ref: %v", err)
				}
			},
			wantHash: "b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3",
		},
		{
			name: "HEAD references non-existent branch",
			setup: func(dir string) {
				gitDir := filepath.Join(dir, ".git")
				if err := os.MkdirAll(gitDir, 0755); err != nil {
					t.Fatalf("failed to create .git dir: %v", err)
				}
				if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/nonexistent\n"), 0644); err != nil {
					t.Fatalf("failed to write HEAD: %v", err)
				}
			},
			wantHash: "",
		},
		{
			name: "no .git directory",
			setup: func(dir string) {
				// Don't create anything
			},
			wantHash: "",
		},
		{
			name: "invalid HEAD content",
			setup: func(dir string) {
				gitDir := filepath.Join(dir, ".git")
				if err := os.MkdirAll(gitDir, 0755); err != nil {
					t.Fatalf("failed to create .git dir: %v", err)
				}
				if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("invalid content\n"), 0644); err != nil {
					t.Fatalf("failed to write HEAD: %v", err)
				}
			},
			wantHash: "",
		},
		{
			name: "HEAD with non-hex content same length as hash",
			setup: func(dir string) {
				gitDir := filepath.Join(dir, ".git")
				if err := os.MkdirAll(gitDir, 0755); err != nil {
					t.Fatalf("failed to create .git dir: %v", err)
				}
				// 40 chars but contains invalid hex chars
				if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ghijklmnopghijklmnopghijklmnopghijklmnop\n"), 0644); err != nil {
					t.Fatalf("failed to write HEAD: %v", err)
				}
			},
			wantHash: "",
		},
		{
			name: "ref file with extra content",
			setup: func(dir string) {
				gitDir := filepath.Join(dir, ".git")
				refsDir := filepath.Join(gitDir, "refs", "heads")
				if err := os.MkdirAll(refsDir, 0755); err != nil {
					t.Fatalf("failed to create refs dir: %v", err)
				}
				if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0644); err != nil {
					t.Fatalf("failed to write HEAD: %v", err)
				}
				// Hash with extra content after it
				hash := "c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4 extra stuff"
				if err := os.WriteFile(filepath.Join(refsDir, "main"), []byte(hash+"\n"), 0644); err != nil {
					t.Fatalf("failed to write ref: %v", err)
				}
			},
			wantHash: "c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4",
		},
		{
			name: "short hash in ref file",
			setup: func(dir string) {
				gitDir := filepath.Join(dir, ".git")
				refsDir := filepath.Join(gitDir, "refs", "heads")
				if err := os.MkdirAll(refsDir, 0755); err != nil {
					t.Fatalf("failed to create refs dir: %v", err)
				}
				if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0644); err != nil {
					t.Fatalf("failed to write HEAD: %v", err)
				}
				// Short hash
				if err := os.WriteFile(filepath.Join(refsDir, "main"), []byte("abc123\n"), 0644); err != nil {
					t.Fatalf("failed to write ref: %v", err)
				}
			},
			wantHash: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(dir)

			got := getGitHash(dir)
			if got != tt.wantHash {
				t.Errorf("getGitHash() = %q, want %q", got, tt.wantHash)
			}
		})
	}
}

func TestFetchFromRepo(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(dir string) // set up the mock repo
		ref         string
		wantErr     bool
		checkResult func(t *testing.T, result *Result)
	}{
		{
			name: "valid repo with model",
			setup: func(dir string) {
				// Create protocol directory and metaModel.json
				protocolDir := filepath.Join(dir, "protocol")
				if err := os.MkdirAll(protocolDir, 0755); err != nil {
					t.Fatalf("failed to create protocol dir: %v", err)
				}
				content := `{
"metaData": {"version": "3.17.0"},
"requests": [],
"notifications": [],
"structures": [],
"enumerations": [],
"typeAliases": []
}`
				if err := os.WriteFile(filepath.Join(protocolDir, "metaModel.json"), []byte(content), 0644); err != nil {
					t.Fatalf("failed to write metaModel.json: %v", err)
				}

				// Create .git directory with HEAD
				gitDir := filepath.Join(dir, ".git")
				if err := os.MkdirAll(gitDir, 0755); err != nil {
					t.Fatalf("failed to create .git dir: %v", err)
				}
				hash := "d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5"
				if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte(hash+"\n"), 0644); err != nil {
					t.Fatalf("failed to write HEAD: %v", err)
				}
			},
			ref:     "v3.17.0",
			wantErr: false,
			checkResult: func(t *testing.T, result *Result) {
				if result.Model == nil {
					t.Fatal("expected non-nil Model")
				}
				if result.Model.Version.Version != "3.17.0" {
					t.Errorf("version = %q, want %q", result.Model.Version.Version, "3.17.0")
				}
				if result.Ref != "v3.17.0" {
					t.Errorf("ref = %q, want %q", result.Ref, "v3.17.0")
				}
				if result.CommitHash != "d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5" {
					t.Errorf("commitHash = %q, want %q", result.CommitHash, "d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5")
				}
				if result.Source != "repo://"+filepath.Dir(filepath.Dir(result.Source[7:])) {
					// Just check it starts with repo://
					if len(result.Source) < 7 || result.Source[:7] != "repo://" {
						t.Errorf("source should start with repo://, got %q", result.Source)
					}
				}
			},
		},
		{
			name: "repo without metaModel.json",
			setup: func(dir string) {
				// Create empty protocol directory
				protocolDir := filepath.Join(dir, "protocol")
				if err := os.MkdirAll(protocolDir, 0755); err != nil {
					t.Fatalf("failed to create protocol dir: %v", err)
				}
			},
			ref:     "",
			wantErr: true,
		},
		{
			name: "repo with invalid metaModel.json",
			setup: func(dir string) {
				protocolDir := filepath.Join(dir, "protocol")
				if err := os.MkdirAll(protocolDir, 0755); err != nil {
					t.Fatalf("failed to create protocol dir: %v", err)
				}
				if err := os.WriteFile(filepath.Join(protocolDir, "metaModel.json"), []byte("{invalid}"), 0644); err != nil {
					t.Fatalf("failed to write metaModel.json: %v", err)
				}
			},
			ref:     "",
			wantErr: true,
		},
		{
			name: "repo without .git directory",
			setup: func(dir string) {
				protocolDir := filepath.Join(dir, "protocol")
				if err := os.MkdirAll(protocolDir, 0755); err != nil {
					t.Fatalf("failed to create protocol dir: %v", err)
				}
				content := `{
"metaData": {"version": "3.17.0"},
"requests": [],
"notifications": [],
"structures": [],
"enumerations": [],
"typeAliases": []
}`
				if err := os.WriteFile(filepath.Join(protocolDir, "metaModel.json"), []byte(content), 0644); err != nil {
					t.Fatalf("failed to write metaModel.json: %v", err)
				}
			},
			ref:     "",
			wantErr: false,
			checkResult: func(t *testing.T, result *Result) {
				if result.CommitHash != "" {
					t.Errorf("expected empty commitHash without .git, got %q", result.CommitHash)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(dir)

			result, err := fetchFromRepo(dir, tt.ref)
			if (err != nil) != tt.wantErr {
				t.Errorf("fetchFromRepo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if tt.checkResult != nil {
				tt.checkResult(t, result)
			}
		})
	}
}
