# lspls - LSP Protocol Type Generator
# Run `just --list` to see available recipes

# Default recipe - show help
default:
    @just --list

# ============================================================================
# Setup
# ============================================================================

# One-time setup: install git hooks
setup:
    @echo "Setting up development environment..."
    @command -v lefthook >/dev/null 2>&1 || { echo "Installing lefthook..."; go install github.com/evilmartians/lefthook@latest; }
    lefthook install
    @echo "Setup complete! Git hooks are now active."

# Install git hooks via lefthook
hooks:
    lefthook install

# ============================================================================
# Development
# ============================================================================

# Build the lspls binary
build:
    go build -o lspls ./cmd/lspls

# Run all tests
test:
    go test -v ./...

# Run tests with race detection
test-race:
    go test -v -race ./...

# Run linter
lint:
    golangci-lint run --config=tools/lint/golangci.toml

# Format all Go files
format:
    gofmt -w $(find . -name '*.go')

# Tidy go modules
tidy:
    go mod tidy

# Run format + lint + test (CI check)
check: format lint test

# Run pre-commit hooks manually
pre-commit:
    lefthook run pre-commit

# ============================================================================
# Code Generation (using lspls itself)
# ============================================================================

# Generate types (dry run - preview only)
gen-dry:
    go run ./cmd/lspls --dry-run --verbose

# Generate specific types (dry run)
gen-types types:
    go run ./cmd/lspls -t {{types}} --dry-run

# Generate InlayHint types (common use case)
gen-inlayhint:
    go run ./cmd/lspls -t InlayHint,InlayHintKind,InlayHintLabelPart,Position,Range --dry-run

# ============================================================================
# Testing
# ============================================================================

# Update golden test files
update-golden:
    go test ./internal/codegen/... -update
    go test ./e2e/... -update

# Run e2e tests only
test-e2e:
    go test -v ./e2e/...

# Run unit tests only
test-unit:
    go test -v ./internal/...

# ============================================================================
# Maintenance
# ============================================================================

# Clean build artifacts
clean:
    rm -f lspls
    go clean -cache

# Show outdated dependencies
outdated:
    go list -u -m all
