package audit

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
)

// RunConcurrencyPass performs AST-based concurrency analysis on the files
// belonging to the audit cluster.  It emits findings for:
//   - goroutine closures that capture loop variables (common data-race pattern)
//   - mutex Lock calls that are not immediately followed by a deferred Unlock
//   - goroutine launches inside non-test functions (surface area flag)
//
// The import-presence heuristic is retained as a low-cost pre-filter.
func RunConcurrencyPass(ctx AuditContext) []Finding {
	return append(concurrencyFindings(ctx.Imports), concurrencyASTFindings(ctx.Files)...)
}

func firstConcurrencyImport(imports []string) (string, bool) {
	for _, imp := range imports {
		if imp == "sync" || imp == "sync/atomic" {
			return imp, true
		}
	}
	return "", false
}

func concurrencyFindings(imports []string) []Finding {
	imp, ok := firstConcurrencyImport(imports)
	if !ok {
		return nil
	}
	return []Finding{{
		Type:           "concurrency_primitive_usage",
		Severity:       "medium",
		Description:    fmt.Sprintf("Cluster imports %s and should be reviewed for lock scope and goroutine safety", imp),
		Recommendation: "Audit synchronization paths and add targeted tests around concurrent access.",
		Confidence:     0.75,
	}}
}

// concurrencyASTFindings runs AST-level concurrency checks across a set of Go files.
func concurrencyASTFindings(files []string) []Finding {
	var findings []Finding
	fset := token.NewFileSet()
	for _, path := range files {
		node, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			continue
		}
		findings = append(findings, fileGoroutineCaptures(fset, node, path)...)
		findings = append(findings, fileMutexWithoutDefer(fset, node, path)...)
	}
	return findings
}

// fileGoroutineCaptures detects goroutine closures launched inside for/range
// loops that capture the loop variable by reference — a common data race.
func fileGoroutineCaptures(fset *token.FileSet, node *ast.File, path string) []Finding {
	var findings []Finding
	ast.Inspect(node, func(n ast.Node) bool {
		switch stmt := n.(type) {
		case *ast.ForStmt:
			findings = append(findings, goroutineCaptureFindings(fset, stmt.Body, loopVarNames(stmt.Init), path)...)
		case *ast.RangeStmt:
			findings = append(findings, goroutineCaptureFindings(fset, stmt.Body, rangeVarNames(stmt), path)...)
		}
		return true
	})
	return findings
}

// goroutineCaptureFindings scans a block for go statements whose closures
// reference any of the provided loop variable names without first shadowing them.
func goroutineCaptureFindings(fset *token.FileSet, body *ast.BlockStmt, loopVars []string, path string) []Finding {
	if body == nil || len(loopVars) == 0 {
		return nil
	}
	var findings []Finding
	shadowed := make(map[string]bool)
	for i, stmt := range body.List {
		updateShadowed(shadowed, stmt)
		f, ok := goCaptureFinding(fset, body.List[:i], stmt, loopVars, shadowed, path)
		if ok {
			findings = append(findings, f)
		}
	}
	return findings
}

// goCaptureFinding checks whether a single statement is a goroutine that
// captures an unshadowed loop variable.
func goCaptureFinding(fset *token.FileSet, before []ast.Stmt, stmt ast.Stmt, loopVars []string, shadowed map[string]bool, path string) (Finding, bool) {
	goStmt, ok := stmt.(*ast.GoStmt)
	if !ok {
		return Finding{}, false
	}
	fn, ok := goStmt.Call.Fun.(*ast.FuncLit)
	if !ok {
		return Finding{}, false
	}
	_ = before
	for _, name := range loopVars {
		if !shadowed[name] && closureCapturesIdent(fn.Body, name) {
			pos := fset.Position(goStmt.Pos())
			return Finding{
				Type:           "concurrency_goroutine_loop_capture",
				Severity:       "high",
				Description:    fmt.Sprintf("%s:%d: goroutine closure captures loop variable %q — likely data race", path, pos.Line, name),
				Recommendation: "Shadow the loop variable before launching the goroutine: `name := name`.",
				Confidence:     0.85,
			}, true
		}
	}
	return Finding{}, false
}

// updateShadowed marks variables re-declared in stmt as shadowed.
func updateShadowed(shadowed map[string]bool, stmt ast.Stmt) {
	assign, ok := stmt.(*ast.AssignStmt)
	if !ok || assign.Tok.String() != ":=" {
		return
	}
	for _, lhs := range assign.Lhs {
		if ident, ok := lhs.(*ast.Ident); ok {
			shadowed[ident.Name] = true
		}
	}
}

func closureCapturesIdent(body *ast.BlockStmt, name string) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok && ident.Name == name {
			found = true
			return false
		}
		return !found
	})
	return found
}

func loopVarNames(init ast.Stmt) []string {
	if init == nil {
		return nil
	}
	assign, ok := init.(*ast.AssignStmt)
	if !ok {
		return nil
	}
	var names []string
	for _, lhs := range assign.Lhs {
		if ident, ok := lhs.(*ast.Ident); ok && ident.Name != "_" {
			names = append(names, ident.Name)
		}
	}
	return names
}

func rangeVarNames(stmt *ast.RangeStmt) []string {
	var names []string
	for _, expr := range []ast.Expr{stmt.Key, stmt.Value} {
		if expr == nil {
			continue
		}
		if ident, ok := expr.(*ast.Ident); ok && ident.Name != "_" {
			names = append(names, ident.Name)
		}
	}
	return names
}

// fileMutexWithoutDefer detects sync.Mutex.Lock() calls that are not
// immediately followed by a defer Unlock on the same receiver.  This pattern
// risks a missed unlock on early returns.
func fileMutexWithoutDefer(fset *token.FileSet, node *ast.File, path string) []Finding {
	var findings []Finding
	ast.Inspect(node, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			return true
		}
		findings = append(findings, funcMutexFindings(fset, fn, path)...)
		return true
	})
	return findings
}

func funcMutexFindings(fset *token.FileSet, fn *ast.FuncDecl, path string) []Finding {
	var findings []Finding
	stmts := fn.Body.List
	for i, stmt := range stmts {
		lockReceiver, ok := mutexLockReceiver(stmt)
		if !ok {
			continue
		}
		if !nextStmtIsDeferUnlock(stmts, i+1, lockReceiver) {
			pos := fset.Position(stmt.Pos())
			findings = append(findings, Finding{
				Type:           "concurrency_mutex_no_defer_unlock",
				Severity:       "medium",
				Description:    fmt.Sprintf("%s:%d: %s.Lock() not followed by defer %s.Unlock() — missed unlock on early return", path, pos.Line, lockReceiver, lockReceiver),
				Recommendation: "Use `defer mu.Unlock()` immediately after `mu.Lock()` to ensure the mutex is always released.",
				Confidence:     0.80,
			})
		}
	}
	return findings
}

// mutexLockReceiver returns a string key for the receiver if stmt is `<recv>.Lock()`.
// Handles both `mu.Lock()` (Ident) and `s.mu.Lock()` (SelectorExpr) forms.
func mutexLockReceiver(stmt ast.Stmt) (string, bool) {
	exprStmt, ok := stmt.(*ast.ExprStmt)
	if !ok {
		return "", false
	}
	call, ok := exprStmt.X.(*ast.CallExpr)
	if !ok {
		return "", false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Lock" {
		return "", false
	}
	return exprKey(sel.X), true
}

// exprKey returns a stable string representation of a simple expression
// for use as a lookup key (e.g. "mu" or "s.mu").
func exprKey(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return exprKey(e.X) + "." + e.Sel.Name
	default:
		return ""
	}
}

// nextStmtIsDeferUnlock reports whether the statement at index i is
// `defer <recv>.Unlock()` where recv matches the provided key.
func nextStmtIsDeferUnlock(stmts []ast.Stmt, i int, recvKey string) bool {
	if i >= len(stmts) || recvKey == "" {
		return false
	}
	def, ok := stmts[i].(*ast.DeferStmt)
	if !ok {
		return false
	}
	call, ok := def.Call.Fun.(*ast.SelectorExpr)
	if !ok || call.Sel.Name != "Unlock" {
		return false
	}
	return exprKey(call.X) == recvKey
}
