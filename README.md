# quorum-review

Go CLI that builds a code graph from Go source, computes blast radius from git diffs, and runs multi-model, multi-layer code reviews.

## How it works

1. **Build** - Parse Go source with `go/ast` and `go/types`, extract functions, types, interfaces, call edges, and interface satisfaction into a SQLite graph
2. **Diff** - Compute the blast radius of a git diff by walking the graph to find affected callers, callees, implementors, and tests
3. **Review** - Send scoped context through a three-layer review pipeline (intent, structural, style) to one or more LLM providers in parallel

## Usage

```
quorum build .          # build the code graph
quorum review HEAD~1    # review changes since last commit
```

## Development

```
go build .
go test ./...
```

Requires Go 1.21+. Uses `modernc.org/sqlite` (pure Go, no CGo).
