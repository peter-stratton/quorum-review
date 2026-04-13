package analyzer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"strings"

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

// isStdlib returns true if pkgPath belongs to the Go standard library.
// Stdlib packages have no dot in the first path segment (e.g., "io", "fmt"),
// while module paths always do (e.g., "github.com/...").
func isStdlib(pkgPath string) bool {
	return !strings.Contains(strings.SplitN(pkgPath, "/", 2)[0], ".")
}

// typesMethodName constructs a method name from a *types.Func in the same
// format as receiverName: (*T).Method or (T).Method. Returns fn.Name() for
// plain functions (no receiver).
func typesMethodName(fn *types.Func) (string, NodeKind) {
	sig := fn.Type().(*types.Signature)
	recv := sig.Recv()
	if recv == nil {
		return fn.Name(), Function
	}
	t := recv.Type()
	if ptr, ok := t.(*types.Pointer); ok {
		named := ptr.Elem().(*types.Named)
		return fmt.Sprintf("(*%s).%s", named.Obj().Name(), fn.Name()), Method
	}
	named := t.(*types.Named)
	return fmt.Sprintf("(%s).%s", named.Obj().Name(), fn.Name()), Method
}

// edgeKey is the deduplication key for edges.
type edgeKey struct {
	from string
	to   string
	kind EdgeKind
}

// Analyze parses Go source files under dir matching patterns and returns the
// extracted nodes and edges.
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

	// Build node ID lookup for edge validation.
	nodeIDs := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		nodeIDs[n.ID] = true
	}

	// Edge extraction: call edges and interface satisfaction.
	var edges []Edge
	seen := make(map[edgeKey]bool)

	addEdge := func(fromID, toID string, kind EdgeKind) {
		k := edgeKey{fromID, toID, kind}
		if !seen[k] {
			seen[k] = true
			edges = append(edges, Edge{FromID: fromID, ToID: toID, Kind: kind})
		}
	}

	// Pass 2a: Call edges.
	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 || pkg.TypesInfo == nil {
			continue
		}
		for _, file := range pkg.Syntax {
			var cachedFromID string
			var cachedFromValid bool
			var enclosingFunc *ast.FuncDecl
			var depth int
			var funcDeclDepth = -1
			ast.Inspect(file, func(n ast.Node) bool {
				if n == nil {
					depth--
					if depth == funcDeclDepth {
						enclosingFunc = nil
						cachedFromID = ""
						cachedFromValid = false
						funcDeclDepth = -1
					}
					return false
				}
				// Track enclosing FuncDecl and cache its fromID.
				if fd, ok := n.(*ast.FuncDecl); ok {
					funcDeclDepth = depth
					enclosingFunc = fd
					fromNode := extractFuncNode(pkg, fd)
					cachedFromID = fromNode.ID
					cachedFromValid = nodeIDs[fromNode.ID]
				}
				depth++

				ce, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if enclosingFunc == nil || !cachedFromValid {
					return true
				}

				var calleeFn *types.Func

				switch fun := ce.Fun.(type) {
				case *ast.Ident:
					// Direct call: B()
					obj := pkg.TypesInfo.Uses[fun]
					if fn, ok := obj.(*types.Func); ok {
						calleeFn = fn
					}
				case *ast.SelectorExpr:
					// Method call: s.Do() or qualified call: pkg.Func()
					if sel, ok := pkg.TypesInfo.Selections[fun]; ok {
						if fn, ok := sel.Obj().(*types.Func); ok {
							calleeFn = fn
						}
					} else {
						// Qualified package-level call.
						obj := pkg.TypesInfo.Uses[fun.Sel]
						if fn, ok := obj.(*types.Func); ok {
							calleeFn = fn
						}
					}
				}

				if calleeFn == nil || calleeFn.Pkg() == nil {
					return true
				}

				calleeName, calleeKind := typesMethodName(calleeFn)
				toID := NewNodeID(calleeFn.Pkg().Path(), calleeName, calleeKind)
				if !nodeIDs[toID] {
					return true
				}

				addEdge(cachedFromID, toID, Calls)
				return true
			})
		}
	}

	// Pass 2b: Interface satisfaction edges.
	type ifaceEntry struct {
		obj  *types.TypeName
		iface *types.Interface
	}
	type concreteEntry struct {
		obj *types.TypeName
	}

	var ifaces []ifaceEntry
	var concretes []concreteEntry

	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 || pkg.Types == nil {
			continue
		}
		scope := pkg.Types.Scope()
		for _, name := range scope.Names() {
			obj, ok := scope.Lookup(name).(*types.TypeName)
			if !ok {
				continue
			}
			if iface, ok := obj.Type().Underlying().(*types.Interface); ok {
				// Skip stdlib interfaces.
				if obj.Pkg() == nil || isStdlib(obj.Pkg().Path()) {
					continue
				}
				// Skip empty interfaces.
				if iface.NumMethods() == 0 {
					continue
				}
				ifaces = append(ifaces, ifaceEntry{obj, iface})
			} else {
				concretes = append(concretes, concreteEntry{obj})
			}
		}
	}

	for _, c := range concretes {
		for _, iface := range ifaces {
			valueSatisfies := types.Implements(c.obj.Type(), iface.iface)
			ptrSatisfies := types.Implements(types.NewPointer(c.obj.Type()), iface.iface)
			if !valueSatisfies && !ptrSatisfies {
				continue
			}
			fromID := NewNodeID(c.obj.Pkg().Path(), c.obj.Name(), Type)
			toID := NewNodeID(iface.obj.Pkg().Path(), iface.obj.Name(), Interface)
			if nodeIDs[fromID] && nodeIDs[toID] {
				addEdge(fromID, toID, Implements)
			}
		}
	}

	result.Edges = edges
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
