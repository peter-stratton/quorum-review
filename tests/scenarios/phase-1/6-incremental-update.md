# Scenario: Incremental update via SHA-256 file hashing

Relates to: Issue #6

## Setup
- SQLite store from issue #4 is complete with `SaveAnalysis`, `GetFile`, and `Stats` working
- A temporary database is created per test via `NewStore(tempPath)`
- Test data includes nodes, edges, and files saved via `SaveAnalysis`

## Cases

### New file detected as changed
- GIVEN an empty database with no files stored
- WHEN calling `ChangedFiles(map[string]string{"a.go": "abc123"})`
- THEN `changed` contains `"a.go"` and `deleted` is empty

### Unchanged file is skipped
- GIVEN a file "a.go" with SHA-256 "abc123" saved via `SaveAnalysis`
- WHEN calling `ChangedFiles(map[string]string{"a.go": "abc123"})`
- THEN both `changed` and `deleted` are empty

### Modified file detected as changed
- GIVEN a file "a.go" with SHA-256 "abc123" saved via `SaveAnalysis`
- WHEN calling `ChangedFiles(map[string]string{"a.go": "def456"})`
- THEN `changed` contains `"a.go"` and `deleted` is empty

### Deleted file detected
- GIVEN a file "a.go" saved via `SaveAnalysis`
- WHEN calling `ChangedFiles(map[string]string{})` (empty map, file no longer on disk)
- THEN `deleted` contains `"a.go"` and `changed` is empty

### DeleteFilesData removes file, nodes, and edges
- GIVEN file "a.go" with 2 nodes and 1 edge between them saved via `SaveAnalysis`
- WHEN calling `DeleteFilesData([]string{"a.go"})`
- THEN `GetFile("a.go")` returns not found, `Stats().NodeCount` decreases by 2, and the edge is also removed

### DeleteFilesData preserves unrelated files
- GIVEN file "a.go" with nodes and file "b.go" with nodes, both saved via `SaveAnalysis`
- WHEN calling `DeleteFilesData([]string{"a.go"})`
- THEN `GetFile("b.go")` still returns the file and its nodes remain in the database
