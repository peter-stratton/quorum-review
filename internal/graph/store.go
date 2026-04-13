package graph

import (
	"database/sql"
	"fmt"

	"github.com/peter-stratton/quorum-review/internal/analyzer"
	_ "modernc.org/sqlite"
)

// Store provides SQLite-backed persistence for the code graph.
type Store struct {
	db *sql.DB
}

// GraphStats holds aggregate counts for the graph store.
type GraphStats struct {
	NodeCount int
	EdgeCount int
	FileCount int
}

// NodeFilter specifies optional criteria for listing nodes.
// Kind is a pointer so that the zero value (Function = 0) can be
// distinguished from "no filter".
type NodeFilter struct {
	Kind    *analyzer.NodeKind
	Package string
	File    string
}

// EdgeFilter specifies optional criteria for listing edges.
// Kind is a pointer so that the zero value (Calls = 0) can be
// distinguished from "no filter".
type EdgeFilter struct {
	Kind   *analyzer.EdgeKind
	FromID string
	ToID   string
}

// NewStore opens (or creates) a SQLite database at dbPath and ensures the
// schema tables exist.
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("graph: open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("graph: ping database: %w", err)
	}

	if err := createTables(db); err != nil {
		db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

func createTables(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS nodes (
			id TEXT PRIMARY KEY,
			name TEXT,
			kind INTEGER,
			package TEXT,
			file TEXT,
			start_line INTEGER,
			end_line INTEGER
		)`,
		`CREATE TABLE IF NOT EXISTS edges (
			from_id TEXT,
			to_id TEXT,
			kind INTEGER,
			PRIMARY KEY(from_id, to_id, kind)
		)`,
		`CREATE TABLE IF NOT EXISTS files (
			path TEXT PRIMARY KEY,
			package TEXT,
			sha256 TEXT
		)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("graph: create tables: %w", err)
		}
	}
	return nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// SaveAnalysis persists all nodes, edges, and files from an AnalysisResult
// in a single transaction. Existing rows with matching primary keys are
// replaced (INSERT OR REPLACE deletes then re-inserts on conflict).
func (s *Store) SaveAnalysis(result *analyzer.AnalysisResult) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("graph: save analysis: %w", err)
	}
	defer tx.Rollback() // no-op after successful commit

	nodeStmt, err := tx.Prepare(`INSERT OR REPLACE INTO nodes (id, name, kind, package, file, start_line, end_line) VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("graph: save analysis: %w", err)
	}
	defer nodeStmt.Close()

	for _, n := range result.Nodes {
		if _, err := nodeStmt.Exec(n.ID, n.Name, int(n.Kind), n.Package, n.File, n.StartLine, n.EndLine); err != nil {
			return fmt.Errorf("graph: save analysis: insert node %s: %w", n.ID, err)
		}
	}

	edgeStmt, err := tx.Prepare(`INSERT OR REPLACE INTO edges (from_id, to_id, kind) VALUES (?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("graph: save analysis: %w", err)
	}
	defer edgeStmt.Close()

	for _, e := range result.Edges {
		if _, err := edgeStmt.Exec(e.FromID, e.ToID, int(e.Kind)); err != nil {
			return fmt.Errorf("graph: save analysis: insert edge: %w", err)
		}
	}

	fileStmt, err := tx.Prepare(`INSERT OR REPLACE INTO files (path, package, sha256) VALUES (?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("graph: save analysis: %w", err)
	}
	defer fileStmt.Close()

	for _, f := range result.Files {
		if _, err := fileStmt.Exec(f.Path, f.Package, f.SHA256); err != nil {
			return fmt.Errorf("graph: save analysis: insert file: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("graph: save analysis: commit: %w", err)
	}
	return nil
}

// GetNode returns the node with the given ID, or an error if not found.
func (s *Store) GetNode(id string) (*analyzer.Node, error) {
	var node analyzer.Node
	err := s.db.QueryRow(
		`SELECT id, name, kind, package, file, start_line, end_line FROM nodes WHERE id = ?`, id,
	).Scan(&node.ID, &node.Name, &node.Kind, &node.Package, &node.File, &node.StartLine, &node.EndLine)
	if err != nil {
		return nil, fmt.Errorf("graph: get node %s: %w", id, err)
	}
	return &node, nil
}

// ListNodes returns nodes matching the given filter criteria.
func (s *Store) ListNodes(opts NodeFilter) ([]analyzer.Node, error) {
	query := `SELECT id, name, kind, package, file, start_line, end_line FROM nodes WHERE 1=1`
	var args []any

	if opts.Kind != nil {
		query += ` AND kind = ?`
		args = append(args, int(*opts.Kind))
	}
	if opts.Package != "" {
		query += ` AND package = ?`
		args = append(args, opts.Package)
	}
	if opts.File != "" {
		query += ` AND file = ?`
		args = append(args, opts.File)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("graph: list nodes: %w", err)
	}
	defer rows.Close()

	var nodes []analyzer.Node
	for rows.Next() {
		var n analyzer.Node
		if err := rows.Scan(&n.ID, &n.Name, &n.Kind, &n.Package, &n.File, &n.StartLine, &n.EndLine); err != nil {
			return nil, fmt.Errorf("graph: list nodes: scan: %w", err)
		}
		nodes = append(nodes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("graph: list nodes: %w", err)
	}
	return nodes, nil
}

// ListEdges returns edges matching the given filter criteria.
func (s *Store) ListEdges(opts EdgeFilter) ([]analyzer.Edge, error) {
	query := `SELECT from_id, to_id, kind FROM edges WHERE 1=1`
	var args []any

	if opts.Kind != nil {
		query += ` AND kind = ?`
		args = append(args, int(*opts.Kind))
	}
	if opts.FromID != "" {
		query += ` AND from_id = ?`
		args = append(args, opts.FromID)
	}
	if opts.ToID != "" {
		query += ` AND to_id = ?`
		args = append(args, opts.ToID)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("graph: list edges: %w", err)
	}
	defer rows.Close()

	var edges []analyzer.Edge
	for rows.Next() {
		var e analyzer.Edge
		if err := rows.Scan(&e.FromID, &e.ToID, &e.Kind); err != nil {
			return nil, fmt.Errorf("graph: list edges: scan: %w", err)
		}
		edges = append(edges, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("graph: list edges: %w", err)
	}
	return edges, nil
}

// GetFile returns the file info for the given path, or an error if not found.
func (s *Store) GetFile(path string) (*analyzer.FileInfo, error) {
	var f analyzer.FileInfo
	err := s.db.QueryRow(
		`SELECT path, package, sha256 FROM files WHERE path = ?`, path,
	).Scan(&f.Path, &f.Package, &f.SHA256)
	if err != nil {
		return nil, fmt.Errorf("graph: get file %s: %w", path, err)
	}
	return &f, nil
}

// Stats returns aggregate counts for nodes, edges, and files in the store.
func (s *Store) Stats() (*GraphStats, error) {
	var stats GraphStats
	for _, q := range []struct {
		query string
		dest  *int
	}{
		{`SELECT COUNT(*) FROM nodes`, &stats.NodeCount},
		{`SELECT COUNT(*) FROM edges`, &stats.EdgeCount},
		{`SELECT COUNT(*) FROM files`, &stats.FileCount},
	} {
		if err := s.db.QueryRow(q.query).Scan(q.dest); err != nil {
			return nil, fmt.Errorf("graph: stats: %w", err)
		}
	}
	return &stats, nil
}
