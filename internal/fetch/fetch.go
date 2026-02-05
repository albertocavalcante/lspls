// SPDX-License-Identifier: MIT
//
// Copyright 2026 Alberto Cavalcante. All rights reserved.
// Use of this source code is governed by a MIT-style license
// that can be found in the LICENSE file.

// Package fetch provides functionality to download the LSP metaModel.json specification.
package fetch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/albertocavalcante/lspls/internal/model"
)

const (
	// VSCodeRepo is the repository containing the LSP specification.
	VSCodeRepo = "https://github.com/microsoft/vscode-languageserver-node"

	// DefaultRef is the default git reference (tag/branch) to use.
	DefaultRef = "release/protocol/3.17.6-next.14"

	// MetaModelPath is the path to metaModel.json within the repository.
	MetaModelPath = "protocol/metaModel.json"
)

// Options configures how to fetch the LSP specification.
type Options struct {
	// Ref is the git reference (tag or branch) to use.
	// If empty, DefaultRef is used.
	Ref string

	// LocalPath is a path to a local metaModel.json file.
	// If set, the file is read directly instead of fetching from git.
	LocalPath string

	// RepoDir is a path to an existing clone of vscode-languageserver-node.
	// If set, the repository is used instead of cloning.
	RepoDir string

	// Timeout for network operations.
	Timeout time.Duration
}

// Result contains the fetched specification and metadata.
type Result struct {
	// Model is the parsed LSP specification.
	Model *model.Model

	// Ref is the git reference that was used.
	Ref string

	// CommitHash is the git commit hash (if fetched from git).
	CommitHash string

	// Source describes where the specification was loaded from.
	Source string
}

// Fetch retrieves and parses the LSP metaModel.json specification.
func Fetch(ctx context.Context, opts Options) (*Result, error) {
	if opts.Timeout == 0 {
		opts.Timeout = 60 * time.Second
	}

	// Priority: LocalPath > RepoDir > Clone
	if opts.LocalPath != "" {
		return fetchFromFile(opts.LocalPath)
	}

	if opts.RepoDir != "" {
		return fetchFromRepo(opts.RepoDir, opts.Ref)
	}

	return fetchFromGit(ctx, opts)
}

// fetchFromFile reads the specification from a local file.
func fetchFromFile(path string) (*Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	m, err := parseModel(data)
	if err != nil {
		return nil, fmt.Errorf("parse model: %w", err)
	}

	return &Result{
		Model:  m,
		Source: fmt.Sprintf("file://%s", path),
	}, nil
}

// fetchFromRepo reads the specification from an existing repository clone.
func fetchFromRepo(repoDir, ref string) (*Result, error) {
	path := filepath.Join(repoDir, MetaModelPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read from repo: %w", err)
	}

	m, err := parseModel(data)
	if err != nil {
		return nil, fmt.Errorf("parse model: %w", err)
	}

	// Try to get commit hash
	hash := getGitHash(repoDir)

	return &Result{
		Model:      m,
		Ref:        ref,
		CommitHash: hash,
		Source:     fmt.Sprintf("repo://%s", repoDir),
	}, nil
}

// fetchFromGit clones the repository and reads the specification.
func fetchFromGit(ctx context.Context, opts Options) (*Result, error) {
	ref := opts.Ref
	if ref == "" {
		ref = DefaultRef
	}

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "lspls-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Clone with shallow depth and sparse checkout
	cloneCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	cmd := exec.CommandContext(cloneCtx, "git", "clone",
		"--quiet",
		"--depth=1",
		"--filter=blob:none",
		"--sparse",
		"--branch="+ref,
		"--single-branch",
		VSCodeRepo,
		tmpDir,
	)
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git clone: %w", err)
	}

	// Sparse checkout just the protocol directory
	cmd = exec.CommandContext(cloneCtx, "git", "-C", tmpDir, "sparse-checkout", "set", "protocol")
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("sparse checkout: %w", err)
	}

	// Read the file
	path := filepath.Join(tmpDir, MetaModelPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read metaModel.json: %w", err)
	}

	m, err := parseModel(data)
	if err != nil {
		return nil, fmt.Errorf("parse model: %w", err)
	}

	hash := getGitHash(tmpDir)

	return &Result{
		Model:      m,
		Ref:        ref,
		CommitHash: hash,
		Source:     fmt.Sprintf("%s@%s", VSCodeRepo, ref),
	}, nil
}

// parseModel parses metaModel.json with line number injection for debugging.
func parseModel(data []byte) (*model.Model, error) {
	// Inject line numbers into JSON for debugging
	data = injectLineNumbers(data)

	var m model.Model
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// injectLineNumbers adds a "line" field to each JSON object.
// This helps with debugging by tracking source locations.
func injectLineNumbers(data []byte) []byte {
	var result []byte
	lineNum := 1

	for i := 0; i < len(data); i++ {
		result = append(result, data[i])
		switch data[i] {
		case '{':
			// Only inject if followed by newline (not inline objects in strings)
			if i+1 < len(data) && data[i+1] == '\n' {
				result = append(result, fmt.Sprintf(`"line":%d,`, lineNum)...)
			}
		case '\n':
			lineNum++
		}
	}
	return result
}

// getGitHash returns the current commit hash for a repository.
func getGitHash(repoDir string) string {
	// Try reading HEAD directly
	headPath := filepath.Join(repoDir, ".git", "HEAD")
	data, err := os.ReadFile(headPath)
	if err != nil {
		return ""
	}

	content := strings.TrimSpace(string(data))

	// Direct hash (detached HEAD)
	if len(content) == 40 && isHex(content) {
		return content
	}

	// Reference (e.g., "ref: refs/heads/main")
	if strings.HasPrefix(content, "ref: ") {
		refPath := filepath.Join(repoDir, ".git", content[5:])
		data, err := os.ReadFile(refPath)
		if err != nil {
			return ""
		}
		hash := strings.TrimSpace(string(data))
		if len(hash) >= 40 {
			return hash[:40]
		}
	}

	return ""
}

func isHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// FetchRaw fetches the raw metaModel.json content via HTTP (for quick access).
// This is faster than cloning but doesn't provide commit hash.
func FetchRaw(ctx context.Context, ref string) ([]byte, error) {
	if ref == "" {
		ref = DefaultRef
	}

	url := fmt.Sprintf("https://raw.githubusercontent.com/microsoft/vscode-languageserver-node/%s/%s", ref, MetaModelPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return io.ReadAll(resp.Body)
}
