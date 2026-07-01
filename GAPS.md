# Implementation Gaps — 2026-07-01

---

## GAP-01: Audit CLI Flags Not Registered

- **Intended Behavior**: The README and `main_helper.go` declare four audit flags (`--audit`, `--audit-pattern`, `--audit-pass`, `--audit-output`). Invoking `./orchestrator --audit` should activate standalone static analysis mode.
- **Current State**: Variables `auditMode`, `auditPattern`, `auditPass`, and `auditOutput` are declared in `main_helper.go:31-34` but none are registered via `flag.BoolVar` / `flag.StringVar` in `parseFlags()`. The Go `flag` package will reject `--audit` as an unknown flag at runtime, and `auditMode` remains `false` permanently.
- **Blocked Goal**: "Static audit mode — standalone code-quality analysis" described throughout the README § Audit Mode and GOALS.md.
- **Implementation Path**:
  1. In `main_helper.go`, inside `parseFlags()`, add before `flag.Parse()`:
     - `flag.BoolVar(&auditMode, "audit", false, "Enable static analysis mode")`
     - `flag.StringVar(&auditPattern, "audit-pattern", "./...", "Go package pattern to analyse")`
     - `flag.StringVar(&auditPass, "audit-pass", "all", "One of architecture, api, concurrency, or all")`
     - `flag.StringVar(&auditOutput, "audit-output", "audit_findings.json", "Output file for findings")`
  2. No other changes needed; `main_loop.go:6` already routes `auditMode == true` to `runAuditMode()`.
- **Dependencies**: None; this is a self-contained fix.
- **Effort**: Small (4 lines of code).

---

## GAP-02: All Three Audit Passes Are Stubs

- **Intended Behavior**: `RunArchitecturePass` should detect dependency cycles, package layering violations, and oversized files/functions. `RunAPIPass` should review exported symbol surface and interface drift. `RunConcurrencyPass` should detect shared-state access patterns and mutex usage.
- **Current State**: All three functions in `audit/passes.go` return a single hardcoded `Finding` with `Severity: "info"` and a generic description string. The `AuditContext` parameter is never read. All analysis output is fabricated regardless of the actual codebase content.
- **Blocked Goal**: "Static audit mode — architecture, API surface, and concurrency passes" (README).
- **Implementation Path**:
  1. **`RunArchitecturePass`**: Iterate `ctx.Hotspots`; for each `Hotspot` with `LOC > 300` or `Complexity > 15`, emit a `Finding{Severity: "high", ...}`. Iterate `ctx.CallDensity`; emit a medium finding for packages with zero inbound density (potential dead package). Check `ctx.Imports` for stdlib packages that suggest architectural violations (e.g., `database/sql` in a presentation-layer package).
  2. **`RunAPIPass`**: Iterate `ctx.Exports`; emit a finding for any exported symbol with an empty `Name` or that is a `SymbolInfo{Kind: "interface"}` without a corresponding implementation symbol in the cluster. Emit findings for exports lacking documentation (requires `packages.NeedSyntax` to inspect `ast.GenDecl.Doc`).
  3. **`RunConcurrencyPass`**: Walk AST files in the cluster (loaded via `packages.NeedSyntax`); detect `sync.Mutex` fields accessed without a corresponding `Lock`/`Unlock` pair in the same function, or `go` statements that reference non-local variables without a mutex guard.
- **Dependencies**: GAP-03 (Exports population) must be resolved before `RunAPIPass` can produce meaningful results. GAP-04 (Hotspots wiring) must be resolved before `RunArchitecturePass` can use hotspot data.
- **Effort**: Large (each pass requires non-trivial AST analysis; estimate 200–400 lines total).

---

## GAP-03: `PackageInfo.Exports` and `AuditContext.Exports` Never Populated

- **Intended Behavior**: `PackageInfo.Exports []string` should list all exported symbol names for a package. `AuditContext.Exports []SymbolInfo` should carry structured symbol data (name, kind, receiver) to audit passes.
- **Current State**: `LoadPackages` (`audit/loader.go:9-57`) loads packages with `packages.NeedSyntax` but never walks the AST to extract exported declarations. `PackageInfo.Exports` is always `nil`. `BuildAuditContext` (`audit/context.go:8`) declares `var exports []SymbolInfo` and returns it without appending. `SymbolInfo` (defined in `audit/types.go:12`) is never instantiated anywhere.
- **Blocked Goal**: API surface review pass; architecture layering checks.
- **Implementation Path**:
  1. In `LoadPackages`, for each parsed `pkg`, range over `pkg.Syntax` (AST files); use `ast.Inspect` to collect `*ast.FuncDecl` and `*ast.GenDecl` (type, var, const) where the first character of the name is uppercase. Append to `info.Exports`.
  2. In `BuildAuditContext`, for each `pkgPath` in `cluster.Packages`, construct `SymbolInfo` from `graph.Packages[pkgPath].Exports` and append to `exports`.
  3. Add `packages.NeedTypes` to the load config if type information is needed for interface-implementation checks.
- **Dependencies**: None beyond the existing `golang.org/x/tools/go/packages` dependency.
- **Effort**: Medium (AST traversal ~50 lines).

---

## GAP-04: `DetectHotspots` Never Called; `AuditContext.Hotspots` Always Empty

- **Intended Behavior**: `DetectHotspots` should compute per-file LOC and cyclomatic complexity and store results in `AuditContext.Hotspots` for use by audit passes.
- **Current State**: `DetectHotspots` (`audit/metrics.go:9`) is fully implemented (correctly uses `go/parser` and `go/ast`) but is never called. `BuildAuditContext` always sets `Hotspots: []Hotspot{}`. No audit pass reads `ctx.Hotspots`.
- **Blocked Goal**: File and function size analysis in architecture pass (README § Audit Mode).
- **Implementation Path**:
  1. In `BuildAuditContext` (`audit/context.go`), collect all `.go` file paths from `graph.Packages` for the packages in `cluster.Packages`, then call `DetectHotspots(files)` and assign the result to `ctx.Hotspots`.
  2. In `RunArchitecturePass`, range over `ctx.Hotspots` and emit findings for entries exceeding configurable thresholds (e.g., `LOC > 300`, `Complexity > 10`).
- **Dependencies**: None; `DetectHotspots` is already correct.
- **Effort**: Small (3–5 lines in `BuildAuditContext`, 10–15 lines in `RunArchitecturePass`).

---

## GAP-05: `injectMemoryIntoPlanner` Is a No-Op

- **Intended Behavior**: Prior-run adaptive metrics (average patch size, retry rate, most-problematic file) should be embedded into LLM prompts to guide the planner toward safer, smaller changes.
- **Current State**: `injectMemoryIntoPlanner(memoryContext string)` (`main_memory.go:3-8`) receives a formatted memory string from `memory.SummarizeForPlanner()` and calls `logInfo(...)` then returns. The `memoryContext` string is never passed to `generateTasksFromDoc`, `executeTask`, or `fixTask`.
- **Blocked Goal**: "Adaptive memory — cross-run metrics are persisted and injected into the planner on the next run" (README § Memory & Adaptive Metrics).
- **Implementation Path**:
  1. Change `injectMemoryIntoPlanner` to store `memoryContext` in a package-level variable (e.g., `var globalMemoryContext string`).
  2. Prepend `globalMemoryContext` to the prompt in `generateTasksFromDoc` and `executeTask` (inside `callLLM`), or pass it as a system message in `callLLM`'s messages array.
  3. Alternatively, embed it directly: in `runExecutionMode`, pass `memoryContext` as an argument all the way to the prompt-building functions.
- **Dependencies**: GAP-06 (RunSummary stats collection) so that the memory being injected is accurate.
- **Effort**: Small (10–20 lines; mostly threading the string through function signatures or using a package-level var).

---

## GAP-06: `RunSummary` Metrics Not Collected During Execution

- **Intended Behavior**: Each run should record total tasks, completed tasks, blocked tasks, average retries, largest patch applied, and most-modified file, stored in the `memory` branch for future planner calibration.
- **Current State**: `runExecutionMode` (`main_exec.go:23-29`) creates a `memory.RunSummary` with only `Timestamp`, `Branch`, and `DurationSeconds`. All other fields are zero. The `execute()` function tracks `taskCounter` but does not count completions, blocks, retries, or patch sizes.
- **Blocked Goal**: Adaptive memory correctness; `UpdateMetrics` averages zeros, producing incorrect `AvgSuccessPatchSize` and `AvgRetryCount`.
- **Implementation Path**:
  1. In `execute()`, add counters: `tasksCompleted int`, `tasksBlocked int`, `totalRetries int`, `maxPatch int`, `modifiedFiles map[string]int`.
  2. Increment `tasksCompleted` in `completeTask`, `tasksBlocked` in `markBlocked`, `totalRetries` when `task.RetryCount` increments, `maxPatch` when a patch passes validation (use `lineCount(diff)`), `modifiedFiles[f]++` for each file in `filesTouched(diff)`.
  3. Before returning from `execute()`, expose these stats back to `runExecutionMode` (via return values or a struct) so they can be set on `RunSummary`.
  4. Set `RunSummary.MostModifiedFile` to the key with the highest count in `modifiedFiles`.
- **Dependencies**: None.
- **Effort**: Medium (~40 lines in `execute()` and its return path).

---

## GAP-07: `validatePatch` Ignores `allowedFiles` Parameter

- **Intended Behavior**: Only files listed in the task's `Files` field (the "allowed" set) should be modifiable by a patch. A patch touching files outside the allowed set should be rejected.
- **Current State**: `validatePatch(diff string, allowedFiles []string, task *Task)` (`main_validatepatch.go:8`) accepts `allowedFiles` but never reads it. The only file-related check is `len(filesTouched(diff)) > maxFilesTouched`, which enforces a count ceiling but not an allow-list.
- **Blocked Goal**: Patch safety — "reject patches modifying more than 3 files" and "enforce file-touch limits" (GOALS.md §1.1, ROADMAP.md M1).
- **Implementation Path**:
  1. In `validatePatch`, after computing `filesTouched(diff)`, check if `len(allowedFiles) > 0`. If so, build a set from `allowedFiles` and iterate `filesTouched`; return an error if any touched file is not in the set.
  2. This check only fires when the task has an explicit `Files` list; tasks with no `Files` list (resolved by `resolveContextFiles`) retain the count-only check.
- **Dependencies**: None.
- **Effort**: Small (~10 lines).

---

## GAP-08: `deletion-ratio` Check Absent from `validatePatch`

- **Intended Behavior**: Patches where more than 30% of changed lines are deletions should be rejected to prevent large-scale accidental deletions.
- **Current State**: `validatePatch` (`main_validatepatch.go`) checks only total line count and file count. No deletion-ratio logic exists anywhere in the codebase.
- **Blocked Goal**: "Add detection for deletion-heavy patches (>30% deletions)" (GOALS.md §1.1); "reject >30% file deletions" (ROADMAP.md M1).
- **Implementation Path**:
  1. Add a helper `deletionRatio(diff string) float64` that counts lines starting with `-` (excluding `---`) and lines starting with `+` (excluding `+++`), then returns `deletions / max(deletions+additions, 1)`.
  2. In `validatePatch`, call `deletionRatio(diff)` and return an error if the ratio exceeds 0.30.
  3. The threshold should be configurable (e.g., a `--max-deletion-ratio` flag defaulting to 0.30).
- **Dependencies**: None.
- **Effort**: Small (~15 lines).

---

## GAP-09: Dead Code — `averageRetries`, `FormatContextForLLM`

- **Intended Behavior**:
  - `averageRetries(totalRetries, tasks int) float64` (`main_util.go:18`) — intended as a helper for computing aggregate retry stats.
  - `FormatContextForLLM(ctx AuditContext) string` (`audit/context.go:45`) — intended to format audit context as an LLM prompt string.
- **Current State**: Both functions are defined and syntactically correct but are never called from any code path. Neither is part of an exported public API (both packages are internal or a `main` package).
- **Blocked Goal**: `averageRetries` would be needed for GAP-06 (RunSummary stats). `FormatContextForLLM` would be needed if audit passes were to delegate analysis to the LLM.
- **Implementation Path**:
  - Wire `averageRetries` into the execution-loop stat collection (GAP-06).
  - Wire `FormatContextForLLM` into a future LLM-backed enrichment step in `runAuditPasses`, or remove it with a comment documenting intent.
- **Dependencies**: GAP-06 for `averageRetries`.
- **Effort**: Small (call-site wiring only; the functions themselves are correct).

---

## GAP-10: `AdaptiveMetrics.MostCommonFailure` Never Set

- **Intended Behavior**: The most frequent build-error pattern across tasks (e.g., `undefined:`, `cannot use`) should be tracked and surfaced in the planner summary so the LLM can avoid generating the same class of error.
- **Current State**: `AdaptiveMetrics.MostCommonFailure string` (`memory/types.go:28`) is declared and JSON-serialized/deserialized but never written by any code path. `SummarizeForPlanner` never reads it.
- **Blocked Goal**: Adaptive memory correctness; failure pattern guidance to planner.
- **Implementation Path**:
  1. In `execute()`, maintain a `map[string]int` counting occurrences of the first token of each build error message (e.g., parse `buildOut` for lines matching `undefined:`, `cannot use`, `imported and not used`).
  2. After the loop, set `RunSummary`'s most-common failure (requires adding a field, or set directly on `AdaptiveMetrics` via a new `UpdateFailurePattern` function in `memory`).
  3. Include `MostCommonFailure` in `SummarizeForPlanner` output.
- **Dependencies**: GAP-06 (execution loop stat collection scaffold).
- **Effort**: Small–Medium (~20 lines).

---

## GAP-11: `call-density` Estimation Is a Placeholder

- **Intended Behavior**: Call density should reflect how frequently each package is depended on by other packages in the graph, enabling identification of high-centrality packages that should receive extra audit scrutiny.
- **Current State**: `BuildAuditContext` (`audit/context.go:20`) sets `callDensity[pkgPath] = len(pkg.Imports)` with the comment `// Placeholder for call density estimation`. This measures outbound imports, not inbound (inbound = how many other packages import this one). The variable name `callDensity` implies inbound call weight. No audit pass currently reads `CallDensity`.
- **Blocked Goal**: Architecture pass centrality analysis; ROADMAP.md M7 "implement dead code detection" and "build symbol map" both benefit from accurate centrality data.
- **Implementation Path**:
  1. In `BuildDependencyGraph` or a new `ComputeInboundDegree` function, iterate `graph.Edges` and build a `map[string]int` counting how many packages list each package as an import.
  2. Store inbound degree in a new `InboundDegree map[string]int` field on `DependencyGraph`, or compute it on the fly in `BuildAuditContext`.
  3. Replace the placeholder body in `BuildAuditContext`.
- **Dependencies**: None.
- **Effort**: Small (~15 lines).

---

## GAP-12: `parseFlags()` Called Twice

- **Intended Behavior**: CLI flags should be parsed once at programme start.
- **Current State**: `main()` (`main_loop.go:4`) calls `parseFlags()`, then `runExecutionMode()` → `execute()` (`main_exec.go:34`) calls `parseFlags()` a second time, invoking `flag.Parse()` twice on the default `CommandLine` `FlagSet`.
- **Blocked Goal**: Correctness; latent brittleness when flag definitions are extended.
- **Implementation Path**: Remove the `parseFlags()` call from `execute()` (line 34 of `main_exec.go`).
- **Dependencies**: None.
- **Effort**: Trivial (remove 1 line).

---

## GAP-13: Phase 2 Observability Logging Not Implemented

- **Intended Behavior**: GOALS.md §2.2 requires build failures saved to `logs/build_failures/<task_id>.log`, rejected patches saved to `logs/rejected_patches/<task_id>.diff`, and a per-run Markdown summary `AUTONOMOUS_RUN_SUMMARY.md`.
- **Current State**: None of these directories, write calls, or summary generation exist. The only artefacts produced are `tasks.json`, `orchestrator.log`, and the memory-branch JSON files.
- **Blocked Goal**: GOALS.md §2.2 and §2.3 observability targets.
- **Implementation Path**:
  1. In `execute()`, when `build()` returns a non-empty string, write the output to `logs/build_failures/<task.ID>.log` (create directory with `os.MkdirAll`).
  2. When `validatePatch` returns an error, write `diff` to `logs/rejected_patches/<task.ID>.diff`.
  3. In `runExecutionMode()`, after `memory.SaveRun`, generate `AUTONOMOUS_RUN_SUMMARY.md` with a formatted summary table.
- **Dependencies**: GAP-06 (so the summary has accurate counts).
- **Effort**: Medium (~40 lines across three write sites).
