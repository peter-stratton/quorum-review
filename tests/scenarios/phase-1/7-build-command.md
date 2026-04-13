# Scenario: quorum build CLI command

Relates to: Issue #7

## Setup
- All prior issues (#1-#6) are complete: analyzer extracts nodes and edges, store persists them, incremental update works
- A temporary Go module with known source files is created for each test
- The `quorum-review` binary is built and available for execution

## Cases

### Fresh build creates database and populates graph
- GIVEN a temporary Go project with at least one `.go` file containing functions and types
- WHEN running `quorum build .` in that directory
- THEN `.quorum/graph.db` is created and the output includes "graph built" with non-zero node, edge, and file counts

### Incremental build skips unchanged files
- GIVEN a successful `quorum build .` has already run on the project
- WHEN running `quorum build .` again without modifying any files
- THEN the output includes "graph is up to date" and no re-analysis is performed

### Incremental build detects modified files
- GIVEN a successful `quorum build .` has already run
- WHEN modifying one `.go` file (e.g., adding a new function) and running `quorum build .` again
- THEN the output includes "graph built" and the stats reflect the updated graph (new node count includes the added function)

### Custom database path via --db flag
- GIVEN a temporary Go project
- WHEN running `quorum build . --db /tmp/custom-test.db`
- THEN the database is created at `/tmp/custom-test.db` instead of `.quorum/graph.db`

### Verbose flag enables debug logging
- GIVEN a temporary Go project
- WHEN running `quorum build . --verbose`
- THEN the output includes debug-level log messages (e.g., file parsing details)

### Empty directory produces zero stats
- GIVEN a directory with no `.go` files
- WHEN running `quorum build .` in that directory
- THEN the command exits successfully and the output indicates zero nodes, edges, and files
