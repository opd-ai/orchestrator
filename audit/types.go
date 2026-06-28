package audit

type PackageInfo struct {
	Name    string
	Path    string
	Imports []string
	Exports []string
	Files   []string
	LOC     int
}

type SymbolInfo struct {
	Name     string
	Kind     string // type, func, method, interface
	Exported bool
	Receiver string
	Package  string
}

type DependencyGraph struct {
	Packages map[string]*PackageInfo
	Edges    map[string][]string // pkg -> imports
}

type Cluster struct {
	ID       string
	Packages []string
	TotalLOC int
}

type Hotspot struct {
	File       string
	LOC        int
	Complexity int
}

type AuditContext struct {
	ClusterSummary string
	Exports        []SymbolInfo
	Imports        []string
	Hotspots       []Hotspot
	CallDensity    map[string]int
}

type Finding struct {
	Package        string
	Type           string
	Severity       string
	Description    string
	Recommendation string
	Confidence     float64
}
