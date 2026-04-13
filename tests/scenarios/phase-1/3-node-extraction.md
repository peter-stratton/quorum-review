# Scenario: Go analyzer node extraction

Relates to: Issue #3

## Setup
- Analyzer types from issue #2 are complete (`Node`, `NodeKind`, `LanguageAnalyzer` interface)
- A temporary Go module is created with source files containing known declarations
- `GoAnalyzer` is instantiated via `NewGoAnalyzer()`

## Cases

### GoAnalyzer satisfies LanguageAnalyzer interface
- GIVEN `GoAnalyzer` is defined in `internal/analyzer`
- WHEN checking the compile-time interface assertion `var _ LanguageAnalyzer = (*GoAnalyzer)(nil)`
- THEN the project compiles without error

### Extracts top-level function as Function node
- GIVEN a Go source file containing `func Foo() {}`
- WHEN running `GoAnalyzer.Analyze(ctx, dir, ["./..."])`
- THEN the result contains a Node with Name `"Foo"`, Kind `Function`, and correct File, StartLine, EndLine

### Extracts method as Method node with receiver in name
- GIVEN a Go source file containing `func (s *Svc) Run() {}`
- WHEN running `GoAnalyzer.Analyze(ctx, dir, ["./..."])`
- THEN the result contains a Node with Name `"(*Svc).Run"` and Kind `Method`

### Extracts struct type as Type node
- GIVEN a Go source file containing `type Config struct{ Port int }`
- WHEN running `GoAnalyzer.Analyze(ctx, dir, ["./..."])`
- THEN the result contains a Node with Name `"Config"` and Kind `Type`

### Extracts interface declaration as Interface node
- GIVEN a Go source file containing `type Reader interface{ Read() }`
- WHEN running `GoAnalyzer.Analyze(ctx, dir, ["./..."])`
- THEN the result contains a Node with Name `"Reader"` and Kind `Interface`

### Extracts unexported declarations
- GIVEN a Go source file containing `func helper() {}`
- WHEN running `GoAnalyzer.Analyze(ctx, dir, ["./..."])`
- THEN the result contains a Node with Name `"helper"` and Kind `Function`

### FileInfo entries have correct SHA-256 hashes
- GIVEN a Go source file with known content
- WHEN running `GoAnalyzer.Analyze(ctx, dir, ["./..."])`
- THEN the `FileInfo.SHA256` for that file matches an independently computed SHA-256 of the file's content

### Multi-file package produces nodes from all files
- GIVEN a package with `a.go` defining `func A()` and `b.go` defining `func B()`
- WHEN running `GoAnalyzer.Analyze(ctx, dir, ["./..."])`
- THEN the result contains nodes for both `A` and `B`

### Language method returns go
- GIVEN a `GoAnalyzer` instance
- WHEN calling `Language()`
- THEN the return value is `"go"`
