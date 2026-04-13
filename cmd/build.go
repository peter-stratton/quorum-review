package cmd

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/peter-stratton/quorum-review/internal/analyzer"
	"github.com/peter-stratton/quorum-review/internal/graph"
	"github.com/spf13/cobra"
)

var dbPath string

var buildCmd = &cobra.Command{
	Use:   "build [directory]",
	Short: "Build the code graph from a Go repository",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runBuild,
}

func init() {
	buildCmd.Flags().StringVar(&dbPath, "db", ".quorum/graph.db", "path to SQLite database")
	rootCmd.AddCommand(buildCmd)
}

func runBuild(cmd *cobra.Command, args []string) error {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	// Resolve to absolute path so file paths match what go/packages returns.
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("build: resolve directory: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return fmt.Errorf("build: create db directory: %w", err)
	}

	store, err := graph.NewStore(dbPath)
	if err != nil {
		return fmt.Errorf("build: open store: %w", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			slog.Error("build: close store", "err", err)
		}
	}()

	hashes, err := hashFiles(absDir)
	if err != nil {
		return fmt.Errorf("build: hash files: %w", err)
	}

	changed, deleted, err := store.ChangedFiles(hashes)
	if err != nil {
		return fmt.Errorf("build: changed files: %w", err)
	}

	if len(changed) == 0 && len(deleted) == 0 {
		slog.Info("graph is up to date")
		return nil
	}

	start := time.Now()

	if len(deleted) > 0 {
		if err := store.DeleteFilesData(deleted); err != nil {
			return fmt.Errorf("build: delete files data: %w", err)
		}
	}

	var patterns []string
	if len(hashes) == len(changed) {
		// First build: every file is new, analyze everything.
		patterns = []string{"./..."}
	} else {
		for _, f := range changed {
			rel, err := filepath.Rel(absDir, f)
			if err != nil {
				rel = f
			}
			pkgDir := filepath.ToSlash(filepath.Dir(rel))
			if pkgDir == "." {
				patterns = append(patterns, "./...")
			} else {
				patterns = append(patterns, "./"+pkgDir+"/...")
			}
		}
		patterns = dedupPatterns(patterns)
	}

	a := analyzer.NewGoAnalyzer()
	result, err := a.Analyze(cmd.Context(), absDir, patterns)
	if err != nil {
		return fmt.Errorf("build: analyze: %w", err)
	}

	if err := store.SaveAnalysis(result); err != nil {
		return fmt.Errorf("build: save analysis: %w", err)
	}

	stats, err := store.Stats()
	if err != nil {
		return fmt.Errorf("build: stats: %w", err)
	}
	slog.Info("graph built",
		"nodes", stats.NodeCount,
		"edges", stats.EdgeCount,
		"files", stats.FileCount,
		"duration", time.Since(start).Round(time.Millisecond),
	)

	return nil
}

// hashFiles walks dir and returns a map[path]sha256 for every .go file,
// skipping vendor/ and testdata/ directories.
func hashFiles(dir string) (map[string]string, error) {
	hashes := make(map[string]string)
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == "vendor" || name == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		h, err := sha256File(path)
		if err != nil {
			return fmt.Errorf("hash %s: %w", path, err)
		}
		hashes[path] = h
		slog.Debug("hashed file", "path", path, "sha256", h[:8])
		return nil
	})
	return hashes, err
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// dedupPatterns removes duplicate package patterns.
func dedupPatterns(patterns []string) []string {
	seen := make(map[string]bool, len(patterns))
	out := make([]string, 0, len(patterns))
	for _, p := range patterns {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}
