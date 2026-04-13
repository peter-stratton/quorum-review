# Scenario: Project scaffolding and build toolchain

Relates to: Issue #1

## Setup
- A fresh clone of the quorum-review repository with no Go module initialized
- Go toolchain installed and available on PATH

## Cases

### Go module initializes with correct path
- GIVEN the repository has no `go.mod` file
- WHEN the scaffolding issue is implemented
- THEN `go.mod` exists at the repository root with module path `github.com/peter-stratton/quorum-review`

### All required dependencies are present
- GIVEN `go.mod` has been created
- WHEN inspecting the module's dependencies
- THEN `go.mod` contains `github.com/spf13/cobra`, `modernc.org/sqlite`, `github.com/stretchr/testify`, and `golang.org/x/tools`

### Project compiles from root
- GIVEN `main.go` exists at the repository root
- WHEN running `go build .`
- THEN the command exits 0 and produces a `quorum-review` binary

### Go vet passes cleanly
- GIVEN the project has been scaffolded with `go.mod` and `main.go`
- WHEN running `go vet ./...`
- THEN the command exits 0 with no diagnostics
