package analyzer

// NodeKind enumerates the kinds of code nodes the analyzer extracts.
// WARNING: iota order is load-bearing. The graph store persists these as
// integers in SQLite. Do not reorder or insert new values before existing ones.
type NodeKind int

const (
	Function  NodeKind = iota // 0
	Method                    // 1
	Type                      // 2
	Interface                 // 3
)

func (k NodeKind) String() string {
	switch k {
	case Function:
		return "Function"
	case Method:
		return "Method"
	case Type:
		return "Type"
	case Interface:
		return "Interface"
	default:
		return "Unknown"
	}
}

// EdgeKind enumerates the kinds of relationships between nodes.
// WARNING: iota order is load-bearing. See NodeKind comment.
type EdgeKind int

const (
	Calls      EdgeKind = iota // 0
	Implements                 // 1
)

func (k EdgeKind) String() string {
	switch k {
	case Calls:
		return "Calls"
	case Implements:
		return "Implements"
	default:
		return "Unknown"
	}
}

// Node represents a semantic code element (function, method, type, or interface).
// ID is a content-addressable SHA-256 derived from package, name, and kind.
type Node struct {
	ID        string
	Name      string
	Kind      NodeKind
	Package   string
	File      string
	StartLine int
	EndLine   int
}

// Edge represents a directed relationship between two nodes.
type Edge struct {
	FromID string
	ToID   string
	Kind   EdgeKind
}

// FileInfo records a source file's path, package, and content hash.
type FileInfo struct {
	Path    string
	Package string
	SHA256  string
}

// AnalysisResult is the output of a LanguageAnalyzer.Analyze call.
type AnalysisResult struct {
	Nodes []Node
	Edges []Edge
	Files []FileInfo
}
