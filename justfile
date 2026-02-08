# lspls - LSP Protocol Type Generator
# Run `just --list` to see available recipes

# Default recipe - show help
default:
    @just --list

# ============================================================================
# Setup
# ============================================================================

# One-time setup: install git hooks and sync tools
setup:
    @echo "Setting up development environment..."
    @command -v lefthook >/dev/null 2>&1 || { echo "Installing lefthook..."; go install github.com/evilmartians/lefthook@latest; }
    lefthook install
    go mod tidy -modfile=tools.go.mod
    @echo "Setup complete! Git hooks are now active."

# Install git hooks via lefthook
hooks:
    lefthook install

# Sync tool dependencies
sync-tools:
    go mod tidy -modfile=tools.go.mod

# ============================================================================
# Development
# ============================================================================

# Build the lspls binary
build:
    go build -o lspls ./cmd/lspls

# Build with all generators embedded
build-full:
    go build -tags=lspls_full -o lspls ./cmd/lspls

# Run all tests
test:
    go test ./...

# Run tests with verbose output
test-v:
    go test -v ./...

# Run tests with gotestsum (better output)
test-sum:
    go tool -modfile=tools.go.mod gotestsum --format pkgname-and-test-fails -- ./...

# Run tests with race detection
test-race:
    go test -race ./...

# Run tests with gotestsum and race detection
test-sum-race:
    go tool -modfile=tools.go.mod gotestsum --format pkgname-and-test-fails -- -race ./...

# Run tests with coverage
test-cover:
    go test -cover ./...

# Run linter
lint:
    go tool -modfile=tools.go.mod golangci-lint run

# Run go vet
vet:
    go vet ./...

# Format all Go files
format:
    gofmt -w $(find . -name '*.go' -not -path './tools/*')

# Tidy all go modules
tidy:
    go mod tidy
    go mod tidy -modfile=tools.go.mod

# Run format + lint + test (CI check)
check: format vet lint test

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
    go test ./generators/golang/... -update
    go test ./e2e/... -update

# Run e2e tests only
test-e2e:
    go test -v ./e2e/...

# Run generator tests only
test-generator:
    go test -v ./generator/... ./generators/...

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

# Update all tool dependencies
update-tools:
    go get -modfile=tools.go.mod -tool gotest.tools/gotestsum@latest
    go get -modfile=tools.go.mod -tool github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
    go mod tidy -modfile=tools.go.mod
