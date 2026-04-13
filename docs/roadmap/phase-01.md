## Phase 1: Graph Foundation

**Goal**: `quorum build .` parses a Go repository, extracts semantic nodes and edges from the AST, and persists the graph in SQLite. Incremental rebuilds skip unchanged files.

**Milestone**: `Phase 1: Graph Foundation` | **Label**: `phase-1` | **Issues**: #1-#7

- Go module init and project scaffolding (cmd, internal packages, go.mod)
- LanguageAnalyzer interface definition
- Go analyzer implementation using go/ast and go/types
- Node extraction: functions, methods, types, interfaces
- Edge extraction: call edges, interface satisfaction
- SQLite store using modernc.org/sqlite (pure Go)
- Graph data model (nodes, edges, file metadata)
- Incremental update via SHA-256 file hashing (skip unchanged files)
- quorum build CLI command (Cobra)
- Unit tests for analyzer and graph store
