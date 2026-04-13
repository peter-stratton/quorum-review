package analyzer

import (
	"context"
	"crypto/sha256"
	"fmt"
)

// LanguageAnalyzer extracts nodes and edges from source files in a directory.
type LanguageAnalyzer interface {
	// Analyze parses the source files matched by patterns under dir and returns
	// the extracted graph elements. patterns follows the same format as
	// golang.org/x/tools/go/packages (e.g., ["./..."]).
	Analyze(ctx context.Context, dir string, patterns []string) (*AnalysisResult, error)

	// Language returns the name of the language this analyzer handles (e.g., "go").
	Language() string
}

// NewNodeID computes a deterministic, content-addressable ID for a node.
// The ID is the hex-encoded SHA-256 of pkg, name, and kind.String() separated
// by null bytes to prevent ambiguity across field boundaries.
//
// The String() values of NodeKind are part of the hash contract. Changing them
// invalidates all previously generated node IDs.
func NewNodeID(pkg, name string, kind NodeKind) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00%s\x00%s", pkg, name, kind.String())
	return fmt.Sprintf("%x", h.Sum(nil))
}
