package analyzer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/token"
	"os"

	"golang.org/x/tools/go/packages"
)

// Compile-time interface assertion.
var _ LanguageAnalyzer = (*GoAnalyzer)(nil)

// GoAnalyzer extracts nodes from Go source using go/packages.
type GoAnalyzer struct{}

// NewGoAnalyzer returns a ready-to-use GoAnalyzer.
func NewGoAnalyzer() *GoAnalyzer {
	return &GoAnalyzer{}
}

// Language returns "go".
func (g *GoAnalyzer) Language() string { return "go" }

// Analyze parses Go source files under dir matching patterns and returns the
// extracted nodes. Edge extraction is not yet implemented (Edges will be nil).
func (g *GoAnalyzer) Analyze(ctx context.Context, dir string, patterns []string) (*AnalysisResult, error) {
	cfg := &packages.Config{
		Mode: packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo |
			packages.NeedName | packages.NeedFiles,
		Dir:     dir,
		Context: ctx,
	}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, fmt.Errorf("go analyzer: packages.Load: %w", err)
	}

	var nodes []Node
	var result AnalysisResult
	seenFiles := make(map[string]bool)
	var files []FileInfo

	for _, pkg := range pkgs {
		if pkg.Fset == nil {
			continue
		}

		if len(pkg.Errors) > 0 {
			for _, e := range pkg.Errors {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", pkg.PkgPath, e))
			}
			continue
		}

		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				switch d := decl.(type) {
				case *ast.FuncDecl:
					nodes = append(nodes, extractFuncNode(pkg, d))
				case *ast.GenDecl:
					if d.Tok != token.TYPE {
						continue
					}
					for _, spec := range d.Specs {
						ts, ok := spec.(*ast.TypeSpec)
						if !ok {
							continue
						}
						kind := Type
						if _, isIface := ts.Type.(*ast.InterfaceType); isIface {
							kind = Interface
						}
						pos := pkg.Fset.Position(ts.Pos())
						nodes = append(nodes, Node{
							ID:        NewNodeID(pkg.PkgPath, ts.Name.Name, kind),
							Name:      ts.Name.Name,
							Kind:      kind,
							Package:   pkg.PkgPath,
							File:      pos.Filename,
							StartLine: pos.Line,
							EndLine:   pkg.Fset.Position(ts.End()).Line,
						})
					}
				}
			}
		}

		// Build FileInfo entries from pkg.GoFiles, deduplicating across packages.
		for _, fpath := range pkg.GoFiles {
			if seenFiles[fpath] {
				continue
			}
			seenFiles[fpath] = true

			data, err := os.ReadFile(fpath)
			if err != nil {
				return nil, fmt.Errorf("go analyzer: read %s: %w", fpath, err)
			}
			h := sha256.Sum256(data)
			files = append(files, FileInfo{
				Path:    fpath,
				Package: pkg.Name,
				SHA256:  hex.EncodeToString(h[:]),
			})
		}
	}

	result.Nodes = nodes
	result.Files = files
	return &result, nil
}

// extractFuncNode builds a Node from a function or method declaration.
func extractFuncNode(pkg *packages.Package, decl *ast.FuncDecl) Node {
	kind := Function
	name := decl.Name.Name

	if decl.Recv != nil && len(decl.Recv.List) > 0 {
		kind = Method
		name = receiverName(decl)
	}

	pos := pkg.Fset.Position(decl.Pos())
	return Node{
		ID:        NewNodeID(pkg.PkgPath, name, kind),
		Name:      name,
		Kind:      kind,
		Package:   pkg.PkgPath,
		File:      pos.Filename,
		StartLine: pos.Line,
		EndLine:   pkg.Fset.Position(decl.End()).Line,
	}
}

// receiverName formats a method name as (*T).Method or (T).Method.
func receiverName(decl *ast.FuncDecl) string {
	recv := decl.Recv.List[0].Type
	switch t := recv.(type) {
	case *ast.StarExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return fmt.Sprintf("(*%s).%s", ident.Name, decl.Name.Name)
		}
	case *ast.Ident:
		return fmt.Sprintf("(%s).%s", t.Name, decl.Name.Name)
	}
	// Fallback for unrecognized receiver types (e.g., generics).
	return decl.Name.Name
}
