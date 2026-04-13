# Phase 1: Graph Foundation

> **Goal:** `quorum build .` parses a Go repository, extracts semantic nodes and edges from the AST, and persists the graph in SQLite. Incremental rebuilds skip unchanged files.

## Milestone

`Phase 1: Graph Foundation`

---

## Issue #1: Project scaffolding

### Description

Initialize the Go module and create the minimal project skeleton for
quorum-review. This establishes the build toolchain so subsequent issues can
compile and test independently.

### Key constraints

- Module path: `github.com/peter-stratton/quorum-review`
- `main.go` at repository root, calls into `cmd` package
- `main.go` should be a stub that compiles (`func main() {}` or minimal Cobra
  placeholder) - real CLI wiring happens in issue #7
- Dependencies in `go.mod`: `github.com/spf13/cobra`,
  `modernc.org/sqlite`, `github.com/stretchr/testify`,
  `golang.org/x/tools` (for `go/packages`)
- No placeholder `doc.go` files - directories are created by subsequent issues
  when real code lands
- Run `go mod tidy` to pin dependency versions

### Acceptance criteria

- [ ] `go.mod` exists with correct module path and all four dependencies
- [ ] `main.go` at repo root compiles with `go build .`
- [ ] `go vet ./...` passes with no errors

### Test cases

- **go-build-succeeds**: `go build .` exits 0 and produces a `quorum-review` binary
- **go-vet-clean**: `go vet ./...` exits 0

---

## Issue #2: Analyzer types and LanguageAnalyzer interface

**Blocked by**: Project scaffolding

### Description

Define the core data types and the `LanguageAnalyzer` interface in
`internal/analyzer/`. These types are consumed by both the analyzer
implementations and the graph store (which imports them directly).

### Key constraints

- Package path: `internal/analyzer`
- `NodeKind` enum: `Function`, `Method`, `Type`, `Interface` (use `iota`-based
  constants with a `String()` method)
- `EdgeKind` enum: `Calls`, `Implements`
- `Node` struct: `ID string` (content-addressable SHA-256 of
  `package + name + kind`), `Name string`, `Kind NodeKind`,
  `Package string`, `File string`, `StartLine int`, `EndLine int`
- `Edge` struct: `FromID string`, `ToID string`, `Kind EdgeKind`
- `FileInfo` struct: `Path string`, `Package string`, `SHA256 string`
- `AnalysisResult` struct: `Nodes []Node`, `Edges []Edge`, `Files []FileInfo`
- `LanguageAnalyzer` interface with two methods:
  - `Analyze(ctx context.Context, dir string, patterns []string) (*AnalysisResult, error)`
  - `Language() string`
- Node ID generation: a `NewNodeID(pkg, name string, kind NodeKind) string`
  helper that computes the deterministic hash
- Files: `types.go` (all types and enums), `analyzer.go` (interface definition
  and `NewNodeID` helper)

### Acceptance criteria

- [ ] `NodeKind` and `EdgeKind` enums have correct constants and `String()` methods
- [ ] `Node`, `Edge`, `FileInfo`, and `AnalysisResult` structs are defined with exported fields
- [ ] `LanguageAnalyzer` interface compiles and is importable from other packages
- [ ] `NewNodeID` returns deterministic IDs - same inputs always produce the same output
- [ ] `go vet ./internal/analyzer/...` passes

### Test cases

- **node-id-deterministic**: calling `NewNodeID("pkg/foo", "Bar", Function)` twice returns the same string
- **node-id-distinct**: different inputs (`pkg/foo.Bar` function vs `pkg/foo.Bar` type) produce different IDs
- **node-kind-string**: `Function.String()` returns `"Function"`, etc.
- **edge-kind-string**: `Calls.String()` returns `"Calls"`, `Implements.String()` returns `"Implements"`

---

## Issue #3: Go analyzer - node extraction

**Blocked by**: Analyzer types and LanguageAnalyzer interface

### Description

Implement the `GoAnalyzer` struct that satisfies the `LanguageAnalyzer`
interface, with node extraction for functions, methods, types, and interfaces.
Uses `golang.org/x/tools/go/packages` for parsing and type resolution. Edge
extraction is deferred to the next issue.

### Key constraints

- Package path: `internal/analyzer`
- File: `go_analyzer.go` (implementation), `go_analyzer_test.go` (tests)
- Constructor: `NewGoAnalyzer() *GoAnalyzer`
- `Language()` returns `"go"`
- Uses `packages.Load` with `NeedSyntax | NeedTypes | NeedTypesInfo | NeedName |
  NeedFiles` mode
- `dir` parameter sets `packages.Config.Dir`
- `patterns` parameter passed directly to `packages.Load` (e.g., `["./..."]`)
- Extracts both exported and unexported declarations
- Node extraction walks `ast.File.Decls`:
  - `*ast.FuncDecl` without receiver -> `Function` node
  - `*ast.FuncDecl` with receiver -> `Method` node
  - `*ast.GenDecl` with `token.TYPE` containing `*ast.InterfaceType` -> `Interface` node
  - `*ast.GenDecl` with `token.TYPE` containing other type specs -> `Type` node
- `Edges` field in `AnalysisResult` is `nil` for now (added in next issue)
- `FileInfo` populated for every file in loaded packages, with SHA-256 computed
  from file contents on disk

### Acceptance criteria

- [ ] `GoAnalyzer` implements `LanguageAnalyzer` (compile-time check via interface assertion)
- [ ] Functions extracted with correct Name, Package, File, StartLine, EndLine
- [ ] Methods extracted with receiver info reflected in Name (e.g., `(*Foo).Bar`)
- [ ] Struct and named types extracted as `Type` nodes
- [ ] Interface declarations extracted as `Interface` nodes
- [ ] Unexported declarations are included
- [ ] `FileInfo` entries populated with correct SHA-256 hashes

### Test cases

- **extract-function**: source with `func Foo() {}` produces a Function node named `Foo`
- **extract-method**: source with `func (s *Svc) Run() {}` produces a Method node named `(*Svc).Run`
- **extract-type**: source with `type Config struct{}` produces a Type node named `Config`
- **extract-interface**: source with `type Reader interface{ Read() }` produces an Interface node named `Reader`
- **extract-unexported**: source with `func helper() {}` produces a Function node named `helper`
- **file-info-sha256**: `FileInfo.SHA256` matches independently computed SHA-256 of the source file
- **multi-file-package**: a package with two files produces nodes from both files

---

## Issue #5: Go analyzer - edge extraction

**Blocked by**: Go analyzer - node extraction

### Description

Add edge extraction to the `GoAnalyzer`. Detects call edges (function/method A
calls function/method B) and interface satisfaction edges (concrete type T
implements interface I). Only checks satisfaction within the analyzed module, not
stdlib interfaces.

### Key constraints

- Modifies: `internal/analyzer/go_analyzer.go`, `internal/analyzer/go_analyzer_test.go`
- Call edge detection: walk `ast.CallExpr` nodes, resolve callee via
  `pkg.TypesInfo.Uses` or `pkg.TypesInfo.Selections`, match to a known node ID
- Interface satisfaction: for each (concrete type, interface) pair within loaded
  packages, check `types.Implements(concrete, iface)` and
  `types.Implements(pointer-to-concrete, iface)`. Only consider interfaces
  defined within the analyzed module (skip stdlib)
- Edge `FromID`/`ToID` must reference valid node IDs generated by `NewNodeID`
- Edges should not duplicate (same from/to/kind pair emitted only once)

### Acceptance criteria

- [ ] Call from `Foo()` to `Bar()` produces an edge `{Foo -> Bar, Calls}`
- [ ] Method call `s.Run()` produces a `Calls` edge to the correct method node
- [ ] Type `MyReader` with `Read()` method satisfying local `Reader` interface produces an `Implements` edge
- [ ] Stdlib interface satisfaction (e.g., `io.Reader`) does NOT produce edges
- [ ] No duplicate edges in the result
- [ ] Previously passing node extraction tests still pass

### Test cases

- **call-edge-function**: `func A() { B() }` produces `Calls` edge from A to B
- **call-edge-method**: `func Run(s *Svc) { s.Do() }` produces `Calls` edge from Run to `(*Svc).Do`
- **call-edge-cross-file**: function in file1.go calls function in file2.go within same package - edge is detected
- **implements-local-interface**: `type W struct{}; func (W) Write([]byte)` satisfies local `Writer` interface
- **no-stdlib-implements**: type satisfying `io.Reader` does not produce an `Implements` edge
- **no-duplicate-edges**: calling the same function twice in one function produces only one `Calls` edge

---

## Issue #4: SQLite store - schema, data model, and CRUD

**Blocked by**: Analyzer types and LanguageAnalyzer interface

### Description

Create the graph store backed by SQLite using `modernc.org/sqlite` (pure Go).
Defines the database schema, provides batch insert via
`SaveAnalysis(*AnalysisResult)`, and exposes minimal query methods for
verification and testing.

### Key constraints

- Package path: `internal/graph`
- Files: `store.go` (implementation), `store_test.go` (tests)
- Constructor: `NewStore(dbPath string) (*Store, error)` - opens or creates the
  database and runs schema migrations
- Uses `modernc.org/sqlite` driver, registered as `"sqlite"`
- Schema tables:
  - `nodes`: `id TEXT PRIMARY KEY, name TEXT, kind INTEGER, package TEXT, file TEXT, start_line INTEGER, end_line INTEGER`
  - `edges`: `from_id TEXT, to_id TEXT, kind INTEGER, PRIMARY KEY(from_id, to_id, kind)`
  - `files`: `path TEXT PRIMARY KEY, package TEXT, sha256 TEXT`
- `SaveAnalysis(result *analyzer.AnalysisResult) error` - wraps all inserts in a
  single transaction. Uses `INSERT OR REPLACE` for upsert semantics
- Query methods:
  - `GetNode(id string) (*analyzer.Node, error)`
  - `ListNodes(opts NodeFilter) ([]analyzer.Node, error)` where `NodeFilter` has
    optional `Kind`, `Package`, `File` fields
  - `ListEdges(opts EdgeFilter) ([]analyzer.Edge, error)` where `EdgeFilter` has
    optional `Kind`, `FromID`, `ToID` fields
  - `GetFile(path string) (*analyzer.FileInfo, error)`
  - `Stats() (*GraphStats, error)` returning node count, edge count, file count
- `GraphStats` struct: `NodeCount int`, `EdgeCount int`, `FileCount int`
- `NodeFilter` and `EdgeFilter` structs defined in `store.go`
- `Close() error` to close the database connection
- Tests use temporary databases (`t.TempDir()` + unique DB file per test)

### Acceptance criteria

- [ ] `NewStore` creates database file and schema tables on first run
- [ ] `NewStore` on an existing database is idempotent (no error, no data loss)
- [ ] `SaveAnalysis` inserts nodes, edges, and files in a single transaction
- [ ] `SaveAnalysis` with duplicate node IDs upserts (updates existing row)
- [ ] `GetNode` returns the correct node by ID
- [ ] `ListNodes` filters by kind, package, and file
- [ ] `ListEdges` filters by kind, from_id, and to_id
- [ ] `Stats` returns accurate counts

### Test cases

- **create-store-fresh**: `NewStore` on a new path creates the file and tables
- **create-store-idempotent**: `NewStore` on existing DB succeeds without error
- **save-and-get-node**: save a node via `SaveAnalysis`, retrieve via `GetNode`
- **save-and-list-edges**: save edges, retrieve via `ListEdges` with `FromID` filter
- **upsert-node**: save a node, save again with different EndLine, verify update
- **save-and-get-file**: save file info, retrieve via `GetFile`
- **stats-counts**: save 3 nodes, 2 edges, 1 file, `Stats` returns `{3, 2, 1}`
- **list-nodes-filter-kind**: save Function and Type nodes, filter by `Kind=Function`

---

## Issue #6: Incremental update via SHA-256 file hashing

**Blocked by**: SQLite store - schema, data model, and CRUD

### Description

Add incremental rebuild support to the graph store. The store compares on-disk
file hashes against stored hashes and returns the set of changed and deleted
files. Also adds `DeleteFilesData` to clean up stale nodes and edges before
re-analysis.

### Key constraints

- Modifies: `internal/graph/store.go`, `internal/graph/store_test.go`
- `ChangedFiles(current map[string]string) (changed []string, deleted []string, err error)`:
  - `current` maps file path to SHA-256 hash
  - `changed` = files where hash differs from stored OR file is new (not in DB)
  - `deleted` = files in DB but not in `current` map
- `DeleteFilesData(paths []string) error`:
  - Deletes rows from `files` table for given paths
  - Deletes rows from `nodes` table where `file` matches any given path
  - Deletes rows from `edges` table where `from_id` or `to_id` references a
    deleted node (cascade via subquery or explicit join)
  - Runs in a single transaction
- `SaveAnalysis` must also update `files` table with new hashes (already handled
  by upsert from issue #5 - verify this works for the incremental flow)

### Acceptance criteria

- [ ] `ChangedFiles` returns new files as changed
- [ ] `ChangedFiles` returns modified files (different hash) as changed
- [ ] `ChangedFiles` returns unchanged files in neither list
- [ ] `ChangedFiles` returns removed files as deleted
- [ ] `DeleteFilesData` removes file, node, and edge rows for specified paths
- [ ] `DeleteFilesData` does not affect data from other files

### Test cases

- **new-file-detected**: empty DB + `ChangedFiles({"a.go": "abc"})` returns `changed=["a.go"]`
- **unchanged-file-skipped**: save file with hash "abc", `ChangedFiles({"a.go": "abc"})` returns empty changed
- **modified-file-detected**: save file with hash "abc", `ChangedFiles({"a.go": "def"})` returns `changed=["a.go"]`
- **deleted-file-detected**: save file "a.go", `ChangedFiles({})` returns `deleted=["a.go"]`
- **delete-cascades-nodes**: save file + nodes + edges, `DeleteFilesData(["a.go"])` removes all related rows
- **delete-preserves-other-files**: save two files with nodes, delete one, other file's data intact

---

## Issue #7: `quorum build` CLI command

**Blocked by**: Go analyzer - edge extraction, Incremental update via SHA-256 file hashing

### Description

Wire the analyzer and graph store together behind a `quorum build` Cobra
command. This is the composition root that connects all phase 1 components into
a working CLI.

### Key constraints

- Creates: `cmd/root.go` (Cobra root command + `Execute()` func), `cmd/build.go`
  (build subcommand)
- Modifies: `main.go` (replace stub with `cmd.Execute()` call)
- Package path: `cmd`
- Command: `quorum build [directory]`
  - `directory` defaults to `"."` if not provided
  - `--db` flag: path to SQLite database, default `".quorum/graph.db"`
  - `--verbose` flag: sets `slog` level to `Debug`
- Build flow:
  1. Create `.quorum/` directory if it does not exist
  2. Open store via `graph.NewStore(dbPath)`
  3. Walk `directory` for `.go` files and compute SHA-256 hashes (local
     `hashFiles(dir string) (map[string]string, error)` helper in `cmd/build.go`)
  4. Call `store.ChangedFiles(hashes)` to get changed/deleted lists
  5. If no changes, log "graph is up to date" and exit
  6. Call `store.DeleteFilesData(deleted)` for removed files
  7. Call `analyzer.Analyze(ctx, dir, changedPatterns)` on changed files
  8. Call `store.SaveAnalysis(result)` to persist
  9. Call `store.Stats()` and log summary: "graph built: N nodes, M edges, F files (D duration)"
- `hashFiles` walks the directory, skips non-`.go` files, skips `vendor/` and
  `testdata/` directories, computes SHA-256 per file, returns `map[path]hash`
- The `changedPatterns` passed to `Analyze` should be file paths relative to
  `dir`, or `["./..."]` if this is the first build (no files in DB)
- Logging uses `slog.Info` for progress and `slog.Debug` for details per the
  conventions

### Acceptance criteria

- [ ] `quorum build .` succeeds on a Go repository and creates `.quorum/graph.db`
- [ ] `quorum build .` run twice: second run logs "graph is up to date" and skips analysis
- [ ] `--db custom/path.db` creates the database at the specified path
- [ ] `--verbose` enables debug-level log output
- [ ] Stats output shows correct node, edge, and file counts

### Test cases

- **build-fresh-repo**: run `quorum build` on a temp Go project, verify DB exists and `Stats` shows non-zero counts
- **build-incremental-skip**: build, then build again without changes, verify analyzer is not invoked (log says "up to date")
- **build-incremental-update**: build, modify a file, rebuild, verify only the changed file is re-analyzed
- **build-custom-db-path**: `--db /tmp/test.db` creates DB at that path
- **build-no-go-files**: run on an empty directory, verify graceful exit with zero stats

---

## Integration chain audit

```
AnalysisResult defined in "Analyzer types" issue
  → produced by GoAnalyzer.Analyze() in "Node extraction" / "Edge extraction"
  → consumed by Store.SaveAnalysis() in "SQLite store"
  → wired by build command in "quorum build CLI"
  ✓ ALL HOPS COVERED

Node / Edge / FileInfo types defined in "Analyzer types" issue
  → populated by GoAnalyzer in "Node extraction" / "Edge extraction"
  → stored/queried by Store in "SQLite store" (graph imports analyzer)
  ✓ ALL HOPS COVERED

SHA-256 hashes (map[string]string)
  → produced by hashFiles() local helper in "quorum build CLI"
  → consumed by Store.ChangedFiles() in "Incremental update"
  → result (changed list) passed to GoAnalyzer.Analyze() in "quorum build CLI"
  ✓ ALL HOPS COVERED

Store.DeleteFilesData() defined in "Incremental update"
  → called by build command in "quorum build CLI"
  ✓ ALL HOPS COVERED

NodeFilter / EdgeFilter / GraphStats defined in "SQLite store"
  → used by Store query methods in "SQLite store"
  → Stats() consumed by build command in "quorum build CLI"
  ✓ ALL HOPS COVERED
```

No uncovered hops.
