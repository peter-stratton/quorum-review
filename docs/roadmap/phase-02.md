## Phase 2: Blast Radius

**Goal**: `quorum diff main..feature` parses a git diff, maps changed lines to graph nodes, and outputs the scoped set of affected nodes (callers, callees, interface implementors, related tests).

**Milestone**: `Phase 2: Blast Radius` | **Label**: `phase-2`

- Git diff parsing (changed files and line ranges from git diff output)
- Map diff hunks to graph nodes by file path and line range
- Blast radius algorithm: direct changes, affected callees, affected callers
- Blast radius: interface implementors affected by interface changes
- Blast radius: test nodes covering affected functions
- quorum diff CLI command with terminal output
- Unit tests for diff parsing and blast radius computation
