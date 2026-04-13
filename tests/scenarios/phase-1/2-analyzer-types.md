# Scenario: Analyzer types and LanguageAnalyzer interface

Relates to: Issue #2

## Setup
- Project scaffolding from issue #1 is complete (`go.mod`, `main.go` compile)
- Package `internal/analyzer` exists with `types.go` and `analyzer.go`

## Cases

### NodeKind enum has correct constants and string representation
- GIVEN the `NodeKind` type is defined in `internal/analyzer/types.go`
- WHEN calling `Function.String()`, `Method.String()`, `Type.String()`, and `Interface.String()`
- THEN each returns `"Function"`, `"Method"`, `"Type"`, and `"Interface"` respectively

### EdgeKind enum has correct constants and string representation
- GIVEN the `EdgeKind` type is defined in `internal/analyzer/types.go`
- WHEN calling `Calls.String()` and `Implements.String()`
- THEN they return `"Calls"` and `"Implements"` respectively

### Node struct has all required exported fields
- GIVEN the `Node` struct is defined in `internal/analyzer/types.go`
- WHEN constructing a `Node` literal
- THEN fields `ID`, `Name`, `Kind`, `Package`, `File`, `StartLine`, and `EndLine` are all accessible

### Edge struct has all required exported fields
- GIVEN the `Edge` struct is defined in `internal/analyzer/types.go`
- WHEN constructing an `Edge` literal
- THEN fields `FromID`, `ToID`, and `Kind` are all accessible

### FileInfo struct has all required exported fields
- GIVEN the `FileInfo` struct is defined in `internal/analyzer/types.go`
- WHEN constructing a `FileInfo` literal
- THEN fields `Path`, `Package`, and `SHA256` are all accessible

### AnalysisResult struct groups nodes, edges, and files
- GIVEN the `AnalysisResult` struct is defined in `internal/analyzer/types.go`
- WHEN constructing an `AnalysisResult` literal
- THEN fields `Nodes`, `Edges`, and `Files` are all accessible as slices

### LanguageAnalyzer interface is importable
- GIVEN the `LanguageAnalyzer` interface is defined in `internal/analyzer/analyzer.go`
- WHEN importing `internal/analyzer` from another package
- THEN the `LanguageAnalyzer` type is visible and declares `Analyze` and `Language` methods

### NewNodeID produces deterministic IDs
- GIVEN the `NewNodeID` function is defined in `internal/analyzer`
- WHEN calling `NewNodeID("pkg/foo", "Bar", Function)` twice
- THEN both calls return the same string

### NewNodeID produces distinct IDs for different inputs
- GIVEN the `NewNodeID` function is defined in `internal/analyzer`
- WHEN calling `NewNodeID("pkg/foo", "Bar", Function)` and `NewNodeID("pkg/foo", "Bar", Type)`
- THEN the two returned IDs are different
