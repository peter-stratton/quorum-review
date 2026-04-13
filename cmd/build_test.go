package cmd

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/peter-stratton/quorum-review/internal/graph"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTempGoProject creates a temporary Go module with a source file containing
// a function, a type, an interface, and an implements relationship.
func setupTempGoProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	goMod := "module example.com/testproject\n\ngo 1.23\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644))

	goFile := `package testproject

type Greeter interface {
	Greet(name string) string
}

type EnglishGreeter struct{}

func (g *EnglishGreeter) Greet(name string) string {
	return "Hello, " + name
}

func NewGreeter() Greeter {
	return &EnglishGreeter{}
}
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte(goFile), 0o644))

	return dir
}

// execBuild resets global flag state and runs the build command with the given args.
// It captures slog output written to stderr and returns it along with any error.
func execBuild(t *testing.T, args ...string) (string, error) {
	t.Helper()

	// Reset global flag state so previous test runs don't leak.
	verbose = false
	dbPath = ".quorum/graph.db"

	// Capture slog output by redirecting stderr through a pipe.
	origStderr := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w

	rootCmd.SetArgs(args)
	cmdErr := rootCmd.Execute()

	// Restore stderr and read captured output.
	w.Close()
	os.Stderr = origStderr

	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)
	r.Close()

	return buf.String(), cmdErr
}

func TestBuild_FreshRepo(t *testing.T) {
	dir := setupTempGoProject(t)
	dbFile := filepath.Join(t.TempDir(), "test.db")

	output, err := execBuild(t, "build", dir, "--db", dbFile)
	require.NoError(t, err)

	// DB file must exist.
	_, statErr := os.Stat(dbFile)
	assert.NoError(t, statErr, "database file should exist")

	// Verify non-zero stats via the store directly.
	store, err := graph.NewStore(dbFile)
	require.NoError(t, err)
	defer store.Close()

	stats, err := store.Stats()
	require.NoError(t, err)
	assert.Greater(t, stats.NodeCount, 0, "should have nodes")
	assert.Greater(t, stats.EdgeCount, 0, "should have edges")
	assert.Greater(t, stats.FileCount, 0, "should have files")

	assert.Contains(t, output, "graph built")
}

func TestBuild_IncrementalSkip(t *testing.T) {
	dir := setupTempGoProject(t)
	dbFile := filepath.Join(t.TempDir(), "test.db")

	// First build.
	_, err := execBuild(t, "build", dir, "--db", dbFile)
	require.NoError(t, err)

	// Second build — nothing changed.
	output, err := execBuild(t, "build", dir, "--db", dbFile)
	require.NoError(t, err)

	assert.Contains(t, output, "graph is up to date")
	assert.NotContains(t, output, "graph built")
}

func TestBuild_IncrementalUpdate(t *testing.T) {
	dir := setupTempGoProject(t)
	dbFile := filepath.Join(t.TempDir(), "test.db")

	// First build.
	_, err := execBuild(t, "build", dir, "--db", dbFile)
	require.NoError(t, err)

	store, err := graph.NewStore(dbFile)
	require.NoError(t, err)
	statsBefore, err := store.Stats()
	require.NoError(t, err)
	store.Close()

	// Append a new function to the source file.
	extra := "\nfunc ExtraFunc() string { return \"extra\" }\n"
	f, err := os.OpenFile(filepath.Join(dir, "main.go"), os.O_APPEND|os.O_WRONLY, 0o644)
	require.NoError(t, err)
	_, err = f.WriteString(extra)
	require.NoError(t, err)
	f.Close()

	// Second build — should detect the change.
	output, err := execBuild(t, "build", dir, "--db", dbFile)
	require.NoError(t, err)

	assert.Contains(t, output, "graph built")

	store, err = graph.NewStore(dbFile)
	require.NoError(t, err)
	defer store.Close()

	statsAfter, err := store.Stats()
	require.NoError(t, err)
	assert.Greater(t, statsAfter.NodeCount, statsBefore.NodeCount, "node count should increase after adding a function")
}

func TestBuild_CustomDBPath(t *testing.T) {
	dir := setupTempGoProject(t)
	customDir := filepath.Join(t.TempDir(), "custom", "nested")
	dbFile := filepath.Join(customDir, "my.db")

	_, err := execBuild(t, "build", dir, "--db", dbFile)
	require.NoError(t, err)

	_, statErr := os.Stat(dbFile)
	assert.NoError(t, statErr, "database file should exist at custom path")
}

func TestBuild_NoGoFiles(t *testing.T) {
	dir := t.TempDir()

	goMod := "module example.com/empty\n\ngo 1.23\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644))

	dbFile := filepath.Join(t.TempDir(), "test.db")

	output, err := execBuild(t, "build", dir, "--db", dbFile)
	require.NoError(t, err)

	// With no .go files and an empty DB, ChangedFiles returns nothing → "up to date".
	assert.Contains(t, output, "graph is up to date")
}

func TestBuild_VerboseFlag(t *testing.T) {
	dir := setupTempGoProject(t)
	dbFile := filepath.Join(t.TempDir(), "test.db")

	output, err := execBuild(t, "build", dir, "--db", dbFile, "--verbose")
	require.NoError(t, err)

	// Debug-level messages should appear (hashFiles logs at Debug level).
	assert.Contains(t, output, "hashed file")
}

func TestHashFiles(t *testing.T) {
	dir := t.TempDir()

	// Create a .go file and a non-.go file.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hello\n"), 0o644))

	// Create vendor/ and testdata/ directories with .go files that should be skipped.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "vendor"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "vendor", "dep.go"), []byte("package dep\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "testdata"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "testdata", "fix.go"), []byte("package fix\n"), 0o644))

	// Enable debug logging so hashFiles doesn't panic on short hashes.
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug})))

	hashes, err := hashFiles(dir)
	require.NoError(t, err)

	assert.Len(t, hashes, 1, "should only include .go files outside vendor/ and testdata/")

	goFilePath := filepath.Join(dir, "main.go")
	assert.Contains(t, hashes, goFilePath)
	assert.Len(t, hashes[goFilePath], 64, "SHA-256 hex should be 64 characters")
}

func TestDedupPatterns(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"no duplicates", []string{"./a/...", "./b/..."}, []string{"./a/...", "./b/..."}},
		{"with duplicates", []string{"./a/...", "./b/...", "./a/..."}, []string{"./a/...", "./b/..."}},
		{"all same", []string{"./a/...", "./a/...", "./a/..."}, []string{"./a/..."}},
		{"empty", nil, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dedupPatterns(tt.in)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSha256File(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := "hello world\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	hash, err := sha256File(path)
	require.NoError(t, err)
	assert.Len(t, hash, 64, "SHA-256 hex should be 64 characters")

	// Same content should produce the same hash.
	hash2, err := sha256File(path)
	require.NoError(t, err)
	assert.Equal(t, hash, hash2)

	// Different content should produce a different hash.
	path2 := filepath.Join(dir, "test2.txt")
	require.NoError(t, os.WriteFile(path2, []byte("different\n"), 0o644))
	hash3, err := sha256File(path2)
	require.NoError(t, err)
	assert.NotEqual(t, hash, hash3)
}

func TestSha256File_NotFound(t *testing.T) {
	_, err := sha256File("/nonexistent/path.txt")
	assert.Error(t, err)
}

func TestHashFiles_SkipsVendorAndTestdata(t *testing.T) {
	dir := t.TempDir()

	// Create nested directory structure.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "pkg"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pkg", "lib.go"), []byte("package pkg\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "vendor", "dep"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "vendor", "dep", "dep.go"), []byte("package dep\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "testdata"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "testdata", "golden.go"), []byte("package golden\n"), 0o644))

	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug})))

	hashes, err := hashFiles(dir)
	require.NoError(t, err)

	// Only pkg/lib.go should be included.
	assert.Len(t, hashes, 1)
	assert.Contains(t, hashes, filepath.Join(dir, "pkg", "lib.go"))

	// Verify vendor and testdata are excluded.
	for path := range hashes {
		assert.False(t, strings.Contains(path, "vendor"), "vendor files should be excluded")
		assert.False(t, strings.Contains(path, "testdata"), "testdata files should be excluded")
	}
}
