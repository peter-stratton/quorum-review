# Scenario: Go analyzer edge extraction

Relates to: Issue #5

## Setup
- Node extraction from issue #3 is complete and passing
- A temporary Go module is created with source files containing known function calls and interface implementations
- `GoAnalyzer` is instantiated via `NewGoAnalyzer()`

## Cases

### Call edge between two functions
- GIVEN a Go source file with `func A() { B() }` and `func B() {}`
- WHEN running `GoAnalyzer.Analyze(ctx, dir, ["./..."])`
- THEN the result contains an Edge with FromID matching A's node ID, ToID matching B's node ID, and Kind `Calls`

### Call edge to a method
- GIVEN a Go source file with `type Svc struct{}`, `func (s *Svc) Do() {}`, and `func Run(s *Svc) { s.Do() }`
- WHEN running `GoAnalyzer.Analyze(ctx, dir, ["./..."])`
- THEN the result contains a `Calls` edge from `Run` to `(*Svc).Do`

### Call edge across files in same package
- GIVEN `file1.go` with `func Caller() { Callee() }` and `file2.go` with `func Callee() {}`
- WHEN running `GoAnalyzer.Analyze(ctx, dir, ["./..."])`
- THEN the result contains a `Calls` edge from `Caller` to `Callee`

### Interface satisfaction produces Implements edge
- GIVEN a Go source file with `type Writer interface { Write([]byte) }`, `type W struct{}`, and `func (W) Write(b []byte) {}`
- WHEN running `GoAnalyzer.Analyze(ctx, dir, ["./..."])`
- THEN the result contains an Edge with FromID matching W's node ID, ToID matching Writer's node ID, and Kind `Implements`

### Stdlib interface satisfaction does not produce edges
- GIVEN a Go source file with `type R struct{}` and `func (R) Read(p []byte) (int, error) { return 0, nil }` (satisfies `io.Reader`)
- WHEN running `GoAnalyzer.Analyze(ctx, dir, ["./..."])`
- THEN no `Implements` edge exists with ToID referencing any stdlib interface

### No duplicate edges
- GIVEN a function that calls the same target twice: `func A() { B(); B() }` and `func B() {}`
- WHEN running `GoAnalyzer.Analyze(ctx, dir, ["./..."])`
- THEN only one `Calls` edge exists from A to B

### Node extraction still works correctly
- GIVEN a Go source file with functions, methods, types, and interfaces
- WHEN running `GoAnalyzer.Analyze(ctx, dir, ["./..."])`
- THEN all nodes from the node extraction test cases are still present and correct
