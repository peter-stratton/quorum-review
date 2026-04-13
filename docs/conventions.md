# quorum-review - Coding Conventions

> Standalone Go CLI that builds a code graph from Go source, computes blast
> radius from git diffs, and sends scoped context through a multi-layer review
> pipeline to one or more LLM providers in parallel.

---

## Error Handling

Wrap errors with context using `fmt.Errorf("context: %w", err)`. No third-party
error libraries.

Use typed errors for conditions callers need to switch on (e.g., `ProviderError`
with HTTP status, `AnalyzerError` with file path and line). Use sentinel errors
sparingly for well-known conditions like `ErrGraphNotBuilt` or
`ErrProviderUnavailable`.

Never swallow errors silently. If an error is intentionally discarded, comment
why.

**Pattern:**

```go
if err := s.db.QueryRow(query, id).Scan(&node.ID, &node.Name); err != nil {
    return nil, fmt.Errorf("graph: lookup node %d: %w", id, err)
}
```

---

## Logging

Use `log/slog` from the standard library with structured key-value fields.

- `Info` for user-facing progress: "building graph for 142 files"
- `Debug` for internal tracing: "parsing internal/graph/store.go"
- `Error` for failures that surface to the user
- `Warn` for degraded-but-continuing situations (e.g., provider timeout, retrying)

Library-style packages (`analyzer`, `graph`) must not log. They return errors
and let the calling layer (`cmd`, `reviewer`) decide what to log.

A `--verbose` CLI flag sets the log level to `Debug`.

**Pattern:**

```go
slog.Info("graph built", "nodes", len(nodes), "edges", len(edges), "duration", elapsed)
```

---

## Testing

Table-driven tests as the default pattern. Use `testify/assert` and
`testify/require` for assertions.

Test files are colocated: `foo_test.go` next to `foo.go`. Test helpers live in
`_test.go` files in the same package, not in a separate testutil package.

Integration tests for SQLite graph operations create a temporary database per
test. LLM provider tests use recorded HTTP fixtures - no live API calls in CI.

**Pattern:**

```go
func TestBlastRadius(t *testing.T) {
    tests := []struct {
        name     string
        changed  []string
        wantHits int
    }{
        {"single function", []string{"internal/graph/store.go:SaveNode"}, 3},
        {"interface change", []string{"internal/analyzer/analyzer.go:LanguageAnalyzer"}, 7},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := computeBlastRadius(tt.changed)
            require.NoError(t, err)
            assert.Len(t, got, tt.wantHits)
        })
    }
}
```

---

## Naming

Standard Go conventions: `PascalCase` for exported identifiers, `camelCase` for
unexported.

- Interfaces named by behavior: `LanguageAnalyzer`, `ReviewProvider` - not
  `IAnalyzer` or `AnalyzerInterface`
- Package names are singular nouns: `graph`, `analyzer`, `provider`, `output`
- Files named after the primary type they contain: `store.go` for `Store`,
  `blast.go` for blast-radius logic
- No `utils`, `helpers`, or `common` packages

---

## Dependency Injection

Constructor functions with explicit parameters. Dependencies stored as struct
fields, set via constructors. No DI framework, no global registry.

Interfaces are defined where they are consumed, not where they are implemented.

**Pattern:**

```go
type Store struct {
    db *sql.DB
}

func NewStore(db *sql.DB) *Store {
    return &Store{db: db}
}
```

---

## CGo Policy

Pure Go dependencies only unless there is a demonstrated need for CGo. Use
`modernc.org/sqlite` (pure Go) for SQLite, not `mattn/go-sqlite3` (CGo). This
eliminates the C compiler dependency and simplifies builds in sandboxed and
agent-driven environments.

---

## Agent-Friendliness Notes

- `go/ast` and `go/types` are fully deterministic with no code generation step.
  Agents can parse and understand the codebase without running generators.
- All provider configuration is explicit in config files and constructor
  parameters. No ambient credential resolution beyond standard environment
  variables (`ANTHROPIC_API_KEY`, `GEMINI_API_KEY`, `OPENAI_API_KEY`).
- The `LanguageAnalyzer` interface is the only extension point for language
  support. Adding a language means implementing one interface in one package.
- SQLite is embedded via a pure Go driver - no external database process to
  manage.
- No convention-over-configuration patterns. All wiring is explicit in `cmd/`.
