# Scenario: SQLite store schema, data model, and CRUD

Relates to: Issue #4

## Setup
- Analyzer types from issue #2 are complete (`Node`, `Edge`, `FileInfo`, `AnalysisResult`)
- A temporary directory is used for each test to hold the SQLite database file
- Store is instantiated via `graph.NewStore(dbPath)`

## Cases

### NewStore creates database and schema on fresh path
- GIVEN a path to a non-existent database file in a temporary directory
- WHEN calling `NewStore(dbPath)`
- THEN the database file is created and contains `nodes`, `edges`, and `files` tables

### NewStore is idempotent on existing database
- GIVEN a database that was already created by a previous `NewStore` call and contains data
- WHEN calling `NewStore(dbPath)` again on the same path
- THEN no error is returned and existing data is preserved

### SaveAnalysis inserts nodes, edges, and files atomically
- GIVEN an `AnalysisResult` with 2 nodes, 1 edge, and 1 file
- WHEN calling `SaveAnalysis(result)`
- THEN all rows are present in the database and `Stats()` returns `{2, 1, 1}`

### SaveAnalysis upserts nodes with matching IDs
- GIVEN a node with ID "abc" and EndLine 10 already saved
- WHEN calling `SaveAnalysis` with the same node ID but EndLine 20
- THEN `GetNode("abc")` returns EndLine 20 and `Stats().NodeCount` is still 1

### GetNode retrieves correct node by ID
- GIVEN a node saved via `SaveAnalysis`
- WHEN calling `GetNode` with that node's ID
- THEN the returned node has matching Name, Kind, Package, File, StartLine, and EndLine

### ListNodes filters by kind
- GIVEN 2 Function nodes and 1 Type node saved
- WHEN calling `ListNodes(NodeFilter{Kind: &Function})`
- THEN exactly 2 nodes are returned, both with Kind `Function`

### ListNodes filters by package
- GIVEN nodes from packages "pkg/a" and "pkg/b"
- WHEN calling `ListNodes(NodeFilter{Package: "pkg/a"})`
- THEN only nodes from "pkg/a" are returned

### ListEdges filters by from_id
- GIVEN edges from node A to B and from node C to D
- WHEN calling `ListEdges(EdgeFilter{FromID: A.ID})`
- THEN only the edge from A to B is returned

### GetFile retrieves file info by path
- GIVEN a FileInfo saved via `SaveAnalysis` with path "internal/foo.go"
- WHEN calling `GetFile("internal/foo.go")`
- THEN the returned FileInfo has the correct Package and SHA256

### Stats returns accurate counts
- GIVEN 3 nodes, 2 edges, and 1 file saved via `SaveAnalysis`
- WHEN calling `Stats()`
- THEN the result is `{NodeCount: 3, EdgeCount: 2, FileCount: 1}`

### Close releases database connection
- GIVEN an open store with data
- WHEN calling `Close()`
- THEN no error is returned and subsequent operations on the store fail gracefully
