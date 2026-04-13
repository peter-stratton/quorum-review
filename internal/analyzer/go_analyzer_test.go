package analyzer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeModule creates a temporary Go module with the given source file and
// returns the directory path. The module is named "testpkg" with go 1.21.
func writeModule(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/testpkg\n\ngo 1.21\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "p.go"), []byte(src), 0o644))
	return dir
}

// writeModuleMulti creates a temporary Go module with multiple source files.
func writeModuleMulti(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/testpkg\n\ngo 1.21\n"), 0o644))
	for name, src := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644))
	}
	return dir
}

// findNode returns the first node with the given name, or fails the test.
func findNode(t *testing.T, nodes []Node, name string) Node {
	t.Helper()
	for _, n := range nodes {
		if n.Name == name {
			return n
		}
	}
	t.Fatalf("node %q not found in %d nodes", name, len(nodes))
	return Node{}
}

func TestGoAnalyzerNodes(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantName string
		wantKind NodeKind
	}{
		{
			name:     "extract-function",
			src:      "package p\n\nfunc Foo() {}\n",
			wantName: "Foo",
			wantKind: Function,
		},
		{
			name:     "extract-method",
			src:      "package p\n\ntype Svc struct{}\n\nfunc (s *Svc) Run() {}\n",
			wantName: "(*Svc).Run",
			wantKind: Method,
		},
		{
			name:     "extract-type",
			src:      "package p\n\ntype Config struct{}\n",
			wantName: "Config",
			wantKind: Type,
		},
		{
			name:     "extract-interface",
			src:      "package p\n\ntype Reader interface{ Read() }\n",
			wantName: "Reader",
			wantKind: Interface,
		},
		{
			name:     "extract-unexported",
			src:      "package p\n\nfunc helper() {}\n",
			wantName: "helper",
			wantKind: Function,
		},
		{
			name:     "extract-value-receiver-method",
			src:      "package p\n\ntype Svc struct{}\n\nfunc (s Svc) Run() {}\n",
			wantName: "(Svc).Run",
			wantKind: Method,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := writeModule(t, tt.src)
			a := NewGoAnalyzer()
			result, err := a.Analyze(context.Background(), dir, []string{"./..."})
			require.NoError(t, err)

			node := findNode(t, result.Nodes, tt.wantName)
			assert.Equal(t, tt.wantKind, node.Kind)
			assert.NotEmpty(t, node.ID)
			assert.NotEmpty(t, node.Package)
			assert.NotEmpty(t, node.File)
			assert.Greater(t, node.StartLine, 0)
			assert.GreaterOrEqual(t, node.EndLine, node.StartLine)
		})
	}
}

func TestFileInfoSHA256(t *testing.T) {
	src := "package p\n\nfunc Hello() {}\n"
	dir := writeModule(t, src)
	a := NewGoAnalyzer()
	result, err := a.Analyze(context.Background(), dir, []string{"./..."})
	require.NoError(t, err)
	require.NotEmpty(t, result.Files)

	// Find the FileInfo for p.go.
	var fi FileInfo
	target := filepath.Join(dir, "p.go")
	for _, f := range result.Files {
		if f.Path == target {
			fi = f
			break
		}
	}
	require.NotEmpty(t, fi.Path, "FileInfo for p.go not found")

	// Independently compute SHA-256 and compare.
	data, err := os.ReadFile(target)
	require.NoError(t, err)
	h := sha256.Sum256(data)
	assert.Equal(t, hex.EncodeToString(h[:]), fi.SHA256)
	assert.Equal(t, "p", fi.Package)
}

func TestMultiFilePackage(t *testing.T) {
	dir := writeModuleMulti(t, map[string]string{
		"a.go": "package p\n\nfunc A() {}\n",
		"b.go": "package p\n\nfunc B() {}\n",
	})
	a := NewGoAnalyzer()
	result, err := a.Analyze(context.Background(), dir, []string{"./..."})
	require.NoError(t, err)

	findNode(t, result.Nodes, "A")
	findNode(t, result.Nodes, "B")
	assert.GreaterOrEqual(t, len(result.Files), 2)
}

func TestLanguage(t *testing.T) {
	assert.Equal(t, "go", NewGoAnalyzer().Language())
}

func edgeKindPtr(k EdgeKind) *EdgeKind { return &k }
func intPtr(n int) *int                { return &n }

func TestEdgeExtraction(t *testing.T) {
	tests := []struct {
		name            string
		files           map[string]string
		wantEdges       []Edge
		wantNoEdgeKind  *EdgeKind // if set, assert no edge of this kind exists
		wantCallEdgeLen *int      // if set, assert exact number of Calls edges
	}{
		{
			name: "call-edge-function",
			files: map[string]string{
				"main.go": "package main\n\nfunc A() { B() }\nfunc B() {}\n",
			},
			wantEdges: []Edge{
				{
					FromID: NewNodeID("example.com/testpkg", "A", Function),
					ToID:   NewNodeID("example.com/testpkg", "B", Function),
					Kind:   Calls,
				},
			},
		},
		{
			name: "call-edge-method",
			files: map[string]string{
				"main.go": "package main\n\ntype Svc struct{}\nfunc (s *Svc) Do() {}\nfunc Run(s *Svc) { s.Do() }\n",
			},
			wantEdges: []Edge{
				{
					FromID: NewNodeID("example.com/testpkg", "Run", Function),
					ToID:   NewNodeID("example.com/testpkg", "(*Svc).Do", Method),
					Kind:   Calls,
				},
			},
		},
		{
			name: "call-edge-cross-file",
			files: map[string]string{
				"file1.go": "package main\n\nfunc Caller() { Callee() }\n",
				"file2.go": "package main\n\nfunc Callee() {}\n",
			},
			wantEdges: []Edge{
				{
					FromID: NewNodeID("example.com/testpkg", "Caller", Function),
					ToID:   NewNodeID("example.com/testpkg", "Callee", Function),
					Kind:   Calls,
				},
			},
		},
		{
			name: "implements-local-interface",
			files: map[string]string{
				"main.go": "package main\n\ntype Writer interface { Write([]byte) (int, error) }\ntype W struct{}\nfunc (W) Write(b []byte) (int, error) { return 0, nil }\n",
			},
			wantEdges: []Edge{
				{
					FromID: NewNodeID("example.com/testpkg", "W", Type),
					ToID:   NewNodeID("example.com/testpkg", "Writer", Interface),
					Kind:   Implements,
				},
			},
		},
		{
			name: "no-stdlib-implements",
			files: map[string]string{
				"main.go": "package main\n\ntype R struct{}\nfunc (R) Read(p []byte) (int, error) { return 0, nil }\n",
			},
			wantEdges:      []Edge{}, // R satisfies io.Reader but that's stdlib — no edge
			wantNoEdgeKind: edgeKindPtr(Implements),
		},
		{
			name: "no-duplicate-edges",
			files: map[string]string{
				"main.go": "package main\n\nfunc A() { B(); B() }\nfunc B() {}\n",
			},
			wantEdges: []Edge{
				{
					FromID: NewNodeID("example.com/testpkg", "A", Function),
					ToID:   NewNodeID("example.com/testpkg", "B", Function),
					Kind:   Calls,
				},
			},
			wantCallEdgeLen: intPtr(1),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := writeModuleMulti(t, tt.files)
			a := NewGoAnalyzer()
			result, err := a.Analyze(context.Background(), dir, []string{"./..."})
			require.NoError(t, err)

			for _, want := range tt.wantEdges {
				assert.Contains(t, result.Edges, want)
			}

			if tt.wantNoEdgeKind != nil {
				for _, e := range result.Edges {
					assert.NotEqual(t, *tt.wantNoEdgeKind, e.Kind, "unexpected %s edge", *tt.wantNoEdgeKind)
				}
			}

			if tt.wantCallEdgeLen != nil {
				var callEdges []Edge
				for _, e := range result.Edges {
					if e.Kind == Calls {
						callEdges = append(callEdges, e)
					}
				}
				assert.Len(t, callEdges, *tt.wantCallEdgeLen)
			}
		})
	}
}
