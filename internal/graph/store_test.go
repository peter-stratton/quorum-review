package graph

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/peter-stratton/quorum-review/internal/analyzer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := NewStore(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	return s
}

func sampleAnalysisResult() *analyzer.AnalysisResult {
	fnID := analyzer.NewNodeID("pkg/foo", "DoStuff", analyzer.Function)
	methID := analyzer.NewNodeID("pkg/foo", "Bar.Run", analyzer.Method)
	typeID := analyzer.NewNodeID("pkg/foo", "Bar", analyzer.Type)

	return &analyzer.AnalysisResult{
		Nodes: []analyzer.Node{
			{ID: fnID, Name: "DoStuff", Kind: analyzer.Function, Package: "pkg/foo", File: "foo.go", StartLine: 10, EndLine: 20},
			{ID: methID, Name: "Bar.Run", Kind: analyzer.Method, Package: "pkg/foo", File: "foo.go", StartLine: 30, EndLine: 40},
			{ID: typeID, Name: "Bar", Kind: analyzer.Type, Package: "pkg/foo", File: "bar.go", StartLine: 1, EndLine: 5},
		},
		Edges: []analyzer.Edge{
			{FromID: fnID, ToID: methID, Kind: analyzer.Calls},
			{FromID: methID, ToID: typeID, Kind: analyzer.Calls},
		},
		Files: []analyzer.FileInfo{
			{Path: "foo.go", Package: "pkg/foo", SHA256: "abc123"},
		},
	}
}

func TestNewStore_Fresh(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	s, err := NewStore(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })

	// Verify the database file was created on disk.
	_, err = os.Stat(dbPath)
	require.NoError(t, err)

	// Verify the three tables exist by querying sqlite_master.
	rows, err := s.db.Query(`SELECT name FROM sqlite_master WHERE type='table' ORDER BY name`)
	require.NoError(t, err)
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		tables = append(tables, name)
	}
	require.NoError(t, rows.Err())
	assert.Equal(t, []string{"edges", "files", "nodes"}, tables)
}

func TestNewStore_Idempotent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// First open: create and save data.
	s1, err := NewStore(dbPath)
	require.NoError(t, err)

	result := sampleAnalysisResult()
	require.NoError(t, s1.SaveAnalysis(result))
	require.NoError(t, s1.Close())

	// Second open: should succeed and preserve data.
	s2, err := NewStore(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { s2.Close() })

	node, err := s2.GetNode(result.Nodes[0].ID)
	require.NoError(t, err)
	assert.Equal(t, result.Nodes[0].Name, node.Name)
}

func TestSaveAnalysis_InsertAndGet(t *testing.T) {
	s := newTestStore(t)

	result := sampleAnalysisResult()
	require.NoError(t, s.SaveAnalysis(result))

	want := result.Nodes[0]
	got, err := s.GetNode(want.ID)
	require.NoError(t, err)

	assert.Equal(t, want.ID, got.ID)
	assert.Equal(t, want.Name, got.Name)
	assert.Equal(t, want.Kind, got.Kind)
	assert.Equal(t, want.Package, got.Package)
	assert.Equal(t, want.File, got.File)
	assert.Equal(t, want.StartLine, got.StartLine)
	assert.Equal(t, want.EndLine, got.EndLine)
}

func TestSaveAnalysis_Upsert(t *testing.T) {
	s := newTestStore(t)

	nodeID := analyzer.NewNodeID("pkg/x", "Fn", analyzer.Function)

	// First save with EndLine=10.
	r1 := &analyzer.AnalysisResult{
		Nodes: []analyzer.Node{
			{ID: nodeID, Name: "Fn", Kind: analyzer.Function, Package: "pkg/x", File: "x.go", StartLine: 1, EndLine: 10},
		},
	}
	require.NoError(t, s.SaveAnalysis(r1))

	// Second save with EndLine=20.
	r2 := &analyzer.AnalysisResult{
		Nodes: []analyzer.Node{
			{ID: nodeID, Name: "Fn", Kind: analyzer.Function, Package: "pkg/x", File: "x.go", StartLine: 1, EndLine: 20},
		},
	}
	require.NoError(t, s.SaveAnalysis(r2))

	got, err := s.GetNode(nodeID)
	require.NoError(t, err)
	assert.Equal(t, 20, got.EndLine)
}

func TestListNodes_FilterKind(t *testing.T) {
	s := newTestStore(t)

	fn1ID := analyzer.NewNodeID("pkg/a", "Fn1", analyzer.Function)
	fn2ID := analyzer.NewNodeID("pkg/a", "Fn2", analyzer.Function)
	typeID := analyzer.NewNodeID("pkg/a", "MyType", analyzer.Type)

	result := &analyzer.AnalysisResult{
		Nodes: []analyzer.Node{
			{ID: fn1ID, Name: "Fn1", Kind: analyzer.Function, Package: "pkg/a", File: "a.go", StartLine: 1, EndLine: 5},
			{ID: fn2ID, Name: "Fn2", Kind: analyzer.Function, Package: "pkg/a", File: "a.go", StartLine: 10, EndLine: 15},
			{ID: typeID, Name: "MyType", Kind: analyzer.Type, Package: "pkg/a", File: "a.go", StartLine: 20, EndLine: 25},
		},
	}
	require.NoError(t, s.SaveAnalysis(result))

	fnKind := analyzer.Function
	got, err := s.ListNodes(NodeFilter{Kind: &fnKind})
	require.NoError(t, err)
	assert.Len(t, got, 2)
	for _, n := range got {
		assert.Equal(t, analyzer.Function, n.Kind)
	}
}

func TestListEdges_FilterFromID(t *testing.T) {
	s := newTestStore(t)

	idA := analyzer.NewNodeID("pkg", "A", analyzer.Function)
	idB := analyzer.NewNodeID("pkg", "B", analyzer.Function)
	idC := analyzer.NewNodeID("pkg", "C", analyzer.Function)

	result := &analyzer.AnalysisResult{
		Nodes: []analyzer.Node{
			{ID: idA, Name: "A", Kind: analyzer.Function, Package: "pkg", File: "a.go", StartLine: 1, EndLine: 5},
			{ID: idB, Name: "B", Kind: analyzer.Function, Package: "pkg", File: "a.go", StartLine: 10, EndLine: 15},
			{ID: idC, Name: "C", Kind: analyzer.Function, Package: "pkg", File: "a.go", StartLine: 20, EndLine: 25},
		},
		Edges: []analyzer.Edge{
			{FromID: idA, ToID: idB, Kind: analyzer.Calls},
			{FromID: idC, ToID: idB, Kind: analyzer.Calls},
		},
	}
	require.NoError(t, s.SaveAnalysis(result))

	got, err := s.ListEdges(EdgeFilter{FromID: idA})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, idA, got[0].FromID)
	assert.Equal(t, idB, got[0].ToID)
}

func TestGetFile(t *testing.T) {
	s := newTestStore(t)

	result := &analyzer.AnalysisResult{
		Files: []analyzer.FileInfo{
			{Path: "internal/graph/store.go", Package: "graph", SHA256: "deadbeef"},
		},
	}
	require.NoError(t, s.SaveAnalysis(result))

	got, err := s.GetFile("internal/graph/store.go")
	require.NoError(t, err)
	assert.Equal(t, "internal/graph/store.go", got.Path)
	assert.Equal(t, "graph", got.Package)
	assert.Equal(t, "deadbeef", got.SHA256)
}

func TestStats(t *testing.T) {
	s := newTestStore(t)

	result := sampleAnalysisResult() // 3 nodes, 2 edges, 1 file
	require.NoError(t, s.SaveAnalysis(result))

	stats, err := s.Stats()
	require.NoError(t, err)
	assert.Equal(t, 3, stats.NodeCount)
	assert.Equal(t, 2, stats.EdgeCount)
	assert.Equal(t, 1, stats.FileCount)
}

func TestChangedFiles(t *testing.T) {
	tests := []struct {
		name        string
		seed        *analyzer.AnalysisResult
		current     map[string]string
		wantChanged []string
		wantDeleted []string
	}{
		{
			name:        "new-file-detected",
			seed:        nil,
			current:     map[string]string{"a.go": "abc123"},
			wantChanged: []string{"a.go"},
		},
		{
			name: "unchanged-file-skipped",
			seed: &analyzer.AnalysisResult{
				Files: []analyzer.FileInfo{{Path: "a.go", Package: "pkg", SHA256: "abc123"}},
			},
			current: map[string]string{"a.go": "abc123"},
		},
		{
			name: "modified-file-detected",
			seed: &analyzer.AnalysisResult{
				Files: []analyzer.FileInfo{{Path: "a.go", Package: "pkg", SHA256: "abc123"}},
			},
			current:     map[string]string{"a.go": "def456"},
			wantChanged: []string{"a.go"},
		},
		{
			name: "deleted-file-detected",
			seed: &analyzer.AnalysisResult{
				Files: []analyzer.FileInfo{{Path: "a.go", Package: "pkg", SHA256: "abc123"}},
			},
			current:     map[string]string{},
			wantDeleted: []string{"a.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestStore(t)
			if tt.seed != nil {
				require.NoError(t, s.SaveAnalysis(tt.seed))
			}

			changed, deleted, err := s.ChangedFiles(tt.current)
			require.NoError(t, err)
			assert.ElementsMatch(t, tt.wantChanged, changed)
			assert.ElementsMatch(t, tt.wantDeleted, deleted)
		})
	}
}

func TestChangedFiles_UpsertFlow(t *testing.T) {
	s := newTestStore(t)

	// Save a.go with hash "abc".
	r1 := &analyzer.AnalysisResult{
		Files: []analyzer.FileInfo{{Path: "a.go", Package: "pkg", SHA256: "abc"}},
	}
	require.NoError(t, s.SaveAnalysis(r1))

	// Detect change.
	changed, deleted, err := s.ChangedFiles(map[string]string{"a.go": "def"})
	require.NoError(t, err)
	assert.Equal(t, []string{"a.go"}, changed)
	assert.Empty(t, deleted)

	// Re-save with updated hash.
	r2 := &analyzer.AnalysisResult{
		Files: []analyzer.FileInfo{{Path: "a.go", Package: "pkg", SHA256: "def"}},
	}
	require.NoError(t, s.SaveAnalysis(r2))

	// No longer detected as changed.
	changed, deleted, err = s.ChangedFiles(map[string]string{"a.go": "def"})
	require.NoError(t, err)
	assert.Empty(t, changed)
	assert.Empty(t, deleted)
}

func TestDeleteFilesData(t *testing.T) {
	t.Run("delete-cascades-nodes", func(t *testing.T) {
		s := newTestStore(t)

		nodeA := analyzer.NewNodeID("pkg", "FnA", analyzer.Function)
		nodeB := analyzer.NewNodeID("pkg", "FnB", analyzer.Function)
		result := &analyzer.AnalysisResult{
			Nodes: []analyzer.Node{
				{ID: nodeA, Name: "FnA", Kind: analyzer.Function, Package: "pkg", File: "a.go", StartLine: 1, EndLine: 5},
				{ID: nodeB, Name: "FnB", Kind: analyzer.Function, Package: "pkg", File: "a.go", StartLine: 10, EndLine: 15},
			},
			Edges: []analyzer.Edge{
				{FromID: nodeA, ToID: nodeB, Kind: analyzer.Calls},
			},
			Files: []analyzer.FileInfo{
				{Path: "a.go", Package: "pkg", SHA256: "abc123"},
			},
		}
		require.NoError(t, s.SaveAnalysis(result))

		require.NoError(t, s.DeleteFilesData([]string{"a.go"}))

		_, err := s.GetFile("a.go")
		assert.ErrorIs(t, err, sql.ErrNoRows)

		stats, err := s.Stats()
		require.NoError(t, err)
		assert.Equal(t, 0, stats.NodeCount)
		assert.Equal(t, 0, stats.EdgeCount)
		assert.Equal(t, 0, stats.FileCount)
	})

	t.Run("delete-preserves-other-files", func(t *testing.T) {
		s := newTestStore(t)

		nodeA := analyzer.NewNodeID("pkg", "FnA", analyzer.Function)
		nodeB := analyzer.NewNodeID("pkg", "FnB", analyzer.Function)

		resultA := &analyzer.AnalysisResult{
			Nodes: []analyzer.Node{
				{ID: nodeA, Name: "FnA", Kind: analyzer.Function, Package: "pkg", File: "a.go", StartLine: 1, EndLine: 5},
			},
			Files: []analyzer.FileInfo{{Path: "a.go", Package: "pkg", SHA256: "aaa"}},
		}
		resultB := &analyzer.AnalysisResult{
			Nodes: []analyzer.Node{
				{ID: nodeB, Name: "FnB", Kind: analyzer.Function, Package: "pkg", File: "b.go", StartLine: 1, EndLine: 5},
			},
			Files: []analyzer.FileInfo{{Path: "b.go", Package: "pkg", SHA256: "bbb"}},
		}
		require.NoError(t, s.SaveAnalysis(resultA))
		require.NoError(t, s.SaveAnalysis(resultB))

		require.NoError(t, s.DeleteFilesData([]string{"a.go"}))

		// b.go data is intact.
		f, err := s.GetFile("b.go")
		require.NoError(t, err)
		assert.Equal(t, "bbb", f.SHA256)

		nodes, err := s.ListNodes(NodeFilter{File: "b.go"})
		require.NoError(t, err)
		assert.Len(t, nodes, 1)

		// a.go data is gone.
		_, err = s.GetFile("a.go")
		assert.ErrorIs(t, err, sql.ErrNoRows)
	})

	t.Run("delete-empty-paths", func(t *testing.T) {
		s := newTestStore(t)
		result := &analyzer.AnalysisResult{
			Files: []analyzer.FileInfo{{Path: "a.go", Package: "pkg", SHA256: "abc"}},
		}
		require.NoError(t, s.SaveAnalysis(result))

		require.NoError(t, s.DeleteFilesData([]string{}))

		// Data is still there.
		f, err := s.GetFile("a.go")
		require.NoError(t, err)
		assert.Equal(t, "abc", f.SHA256)
	})

	t.Run("cross-file-edge-cascade", func(t *testing.T) {
		s := newTestStore(t)

		nodeA := analyzer.NewNodeID("pkg", "FnA", analyzer.Function)
		nodeB := analyzer.NewNodeID("pkg", "FnB", analyzer.Function)

		result := &analyzer.AnalysisResult{
			Nodes: []analyzer.Node{
				{ID: nodeA, Name: "FnA", Kind: analyzer.Function, Package: "pkg", File: "a.go", StartLine: 1, EndLine: 5},
				{ID: nodeB, Name: "FnB", Kind: analyzer.Function, Package: "pkg", File: "b.go", StartLine: 1, EndLine: 5},
			},
			Edges: []analyzer.Edge{
				{FromID: nodeA, ToID: nodeB, Kind: analyzer.Calls},
			},
			Files: []analyzer.FileInfo{
				{Path: "a.go", Package: "pkg", SHA256: "aaa"},
				{Path: "b.go", Package: "pkg", SHA256: "bbb"},
			},
		}
		require.NoError(t, s.SaveAnalysis(result))

		// Delete a.go — edge A→B should be removed (from_id references deleted node).
		require.NoError(t, s.DeleteFilesData([]string{"a.go"}))

		// Node B and file b.go survive.
		_, err := s.GetNode(nodeB)
		require.NoError(t, err)

		f, err := s.GetFile("b.go")
		require.NoError(t, err)
		assert.Equal(t, "bbb", f.SHA256)

		// Edge is gone.
		stats, err := s.Stats()
		require.NoError(t, err)
		assert.Equal(t, 0, stats.EdgeCount)
		assert.Equal(t, 1, stats.NodeCount)
		assert.Equal(t, 1, stats.FileCount)
	})
}
