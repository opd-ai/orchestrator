# IMPLEMENTATION GAP AUDIT — 2026-07-01

## Project Architecture Overview

**Module:** `github.com/opd-ai/orchestrator`
**Go version:** 1.26.1
**Dependencies:** `golang.org/x/tools` (packages loader), `golang.org/x/mod`, `golang.org/x/sync`

### Stated Goals
The orchestrator is a document-driven agentic loop for local LLMs that:
1. Reads planning documents (`AUDIT.md`, `GAPS.md`, `GOALS.md`, `PLAN.md`, `ROADMAP.md`) and generates atomic tasks via LLM.
2. Executes tasks in DAG order, applies unified-diff patches, validates builds, and commits results.
3. Provides a **static audit mode** (`--audit`) analysing architecture, API surface, and concurrency of any Go codebase.
4. Persists adaptive metrics across runs and injects them into the planner.

### Package Responsibilities
| Package | Intended Responsibility |
|---------|------------------------|
| `main` (root) | CLI entry, execution loop, patch apply/validate, LLM calls, git operations |
| `audit` | Static analysis: package loading, dependency graph, clustering, hotspot detection, architecture/API/concurrency passes |
| `memory` | Cross-run state: run log, adaptive metrics, git-branch-backed persistence |

### Structural Overview
- **3 packages**, ~1 441 LOC across 23 source files.
- Build succeeds (`go build ./...` passes).
- `go vet ./...` reports one warning (redundant `\n` in `fmt.Println` — `main_helper.go:91`).
- Zero tests exist across all packages.

---

## Gap Summary

| Category | Count | Critical | High | Medium | Low |
|----------|-------|----------|------|--------|-----|
| Stubs / Partial impl | 6 | 2 | 2 | 2 | 0 |
| Dead Code | 4 | 0 | 3 | 1 | 0 |
| Partially Wired | 5 | 1 | 2 | 2 | 0 |
| Interface / Contract Gaps | 3 | 0 | 2 | 1 | 0 |
| Dependency / Import Gaps | 1 | 0 | 0 | 0 | 1 |
| Tracked TODOs / Vet | 2 | 0 | 0 | 0 | 2 |
| **Total** | **21** | **3** | **9** | **6** | **2** |

---

## Implementation Completeness by Package

| Package | Exported Functions | Implemented | Stubs | Dead | Notes |
|---------|--------------------|-------------|-------|------|-------|
| `main` | 0 (all unexported) | ~70% | 2 | 2 | Memory injection, RunSummary stats not collected |
| `audit` | 8 | ~30% | 3 | 3 | All three passes are stubs; Exports/Hotspots never populated |
| `memory` | 5 | ~60% | 0 | 0 | MostCommonFailure field never set; RunSummary fields all zero |

---

## Findings

### CRITICAL

- [x] **Audit CLI flags not registered** — `main_helper.go:31-34` — Variables `auditMode`, `auditPattern`, `auditPass`, `auditOutput` are declared but zero of their four corresponding `flag.*Var` registrations exist in `parseFlags()`. Invoking `./orchestrator --audit` passes the flag to the Go flag library as an unknown flag and the programme exits with an error. The entire audit feature described in the README is unreachable from the CLI. **Remediation:** In `parseFlags()` (main_helper.go) add `flag.BoolVar(&auditMode, "audit", false, "Enable static analysis mode")`, `flag.StringVar(&auditPattern, "audit-pattern", "./...", ...)`, `flag.StringVar(&auditPass, "audit-pass", "all", ...)`, and `flag.StringVar(&auditOutput, "audit-output", "audit_findings.json", ...)` before `flag.Parse()`. Validate with `./orchestrator --audit --audit-pass all`. **Blocked goal:** Standalone code-quality analysis feature documented in README § Audit Mode.

- [x] **All three audit passes are stubs** — `audit/passes.go:3,14,25` — `RunArchitecturePass`, `RunAPIPass`, and `RunConcurrencyPass` each return a single hardcoded `Finding{Type: "..._review", Severity: "info", Description: "...", Confidence: 0.6}` with no code inspection at all. The `AuditContext` parameter is received but never read. No layering violations, no exported-symbol review, no concurrency primitive inspection is performed. **Remediation:** Implement each pass by iterating `ctx.Exports`, `ctx.Imports`, `ctx.CallDensity`, and `ctx.Hotspots` to produce findings based on actual data (see GAPS.md for implementation path). **Blocked goal:** "Static audit mode — standalone code-quality analysis" (README).

- [x] **`injectMemoryIntoPlanner` is a no-op** — `main_memory.go:3-8` — The function receives `memoryContext string` and calls `logInfo(...)` then returns. The memory string is never incorporated into any LLM prompt; neither `executeTask`, `fixTask`, nor `generateTasksFromDoc` receive or embed it. Adaptive learning across runs is stated as a core feature ("Adaptive memory" bullet in README). **Remediation:** Pass `memoryContext` to `generateTasksFromDoc` as an additional prompt prefix (or append it to the task execution prompt in `executeTask`). **Blocked goal:** "Adaptive memory — cross-run metrics injected into planner" (README).

### HIGH

- [x] **`RunSummary` stats not collected during execution** — `main_exec.go:23-29` — The `memory.RunSummary` struct is created with only `Timestamp`, `Branch`, and `DurationSeconds` populated. Fields `TasksTotal`, `TasksCompleted`, `TasksBlocked`, `AvgRetries`, `LargestPatch`, and `MostModifiedFile` remain zero/empty because the execution loop never records them. Consequently `UpdateMetrics` averages zeros into `AvgSuccessPatchSize` and `AvgRetryCount`, polluting the adaptive model with invalid data. **Remediation:** Add counters in `execute()` for completed tasks, blocked tasks, total retries, and max patch line count (diff line count is already computed via `lineCount`). Populate all `RunSummary` fields before calling `memory.SaveRun`. **Blocked goal:** "Adaptive memory" feature correctness.

- [x] **`validatePatch` ignores `allowedFiles` parameter** — `main_validatepatch.go:8-17` — The function signature accepts `allowedFiles []string` but the parameter is never read. GOALS.md §1.1 explicitly describes "reject patches modifying more than 3 files" and ROADMAP.md Milestone 1 describes "enforce file-touch limits". The file-count check (`len(filesTouched(diff)) > maxFilesTouched`) counts all touched files but does not verify they are within the allowed set. A patch that edits three unrelated files passes validation. **Remediation:** Add a loop comparing `filesTouched(diff)` entries against `allowedFiles`; return an error if any touched file is not in the allowed set. **Blocked goal:** Patch safety (GOALS.md §1.1, ROADMAP.md M1).

- [x] **`PackageInfo.Exports` and `AuditContext.Exports` never populated** — `audit/loader.go:9-57`, `audit/context.go:8-42` — `LoadPackages` uses `packages.NeedSyntax` but never extracts exported symbols into `PackageInfo.Exports`. `BuildAuditContext` declares `var exports []SymbolInfo` and assigns it to `AuditContext.Exports` without ever appending a single element. `SymbolInfo` is fully defined (name, kind, exported, receiver, package) but is never instantiated. Exported-symbol review (`RunAPIPass`) operates on an always-empty slice. **Remediation:** In `LoadPackages`, after parsing syntax, walk each AST file with `ast.Inspect` to collect top-level `FuncDecl` and `GenDecl` nodes with exported names into `PackageInfo.Exports`. In `BuildAuditContext`, populate `exports` from `pkg.Exports`. **Blocked goal:** API surface review pass.

- [x] **`DetectHotspots` never called; `AuditContext.Hotspots` always empty** — `audit/metrics.go:9`, `audit/context.go:40` — `DetectHotspots` correctly parses files and computes cyclomatic-complexity scores, but is never invoked. `BuildAuditContext` always returns `Hotspots: []Hotspot{}`. No audit pass reads `ctx.Hotspots`. The architecture pass described in the README ("file and function size" analysis) therefore never triggers. **Remediation:** In `BuildAuditContext`, gather all `.go` files from `cluster.Packages` and pass them to `DetectHotspots`; assign the result to `AuditContext.Hotspots`. Update `RunArchitecturePass` to iterate `ctx.Hotspots` and emit HIGH findings for files exceeding a threshold. **Blocked goal:** Architecture audit pass.

- [x] **`deletion-ratio` check absent from `validatePatch`** — `main_validatepatch.go` — GOALS.md §1.1 and ROADMAP.md Milestone 1 both mandate rejecting patches where >30% of lines are deletions. `validatePatch` checks only total line count and file count; it never counts `-` lines versus total diff lines. A patch that deletes 90% of a file passes validation. **Remediation:** Count lines beginning with `-` (excluding `---` headers) and lines beginning with `+` (excluding `+++` headers) in `validatePatch`; reject if `deletions/(deletions+additions) > 0.30`. **Blocked goal:** Patch safety (GOALS.md §1.1, ROADMAP.md M1).

- [x] **`averageRetries` is dead code** — `main_util.go:18-23` — Defined as a public utility but never called anywhere in the project. The execution loop counts retries per task but the aggregate `averageRetries` helper is unused. **Remediation:** Wire into `RunSummary.AvgRetries` computation at the end of `execute()`, or remove if the inline calculation is preferred.

- [x] **`FormatContextForLLM` is dead code** — `audit/context.go:45-63` — This function formats `AuditContext` as a human-readable string suitable for LLM prompts but is never called. The audit mode does not send findings to an LLM at all in the current implementation. **Remediation:** Either call it in `runAuditPasses` when passing context to an LLM-backed enrichment step, or remove it. If a future LLM-augmented audit pass is planned, document this with a comment.

- [x] **`FormatContextForLLM` is dead code** (duplicate listing omitted; see above).

### MEDIUM

- [x] **`AdaptiveMetrics.MostCommonFailure` never set or read** — `memory/types.go:28` — Field `MostCommonFailure string` is declared, JSON-tagged, and persisted/loaded, but no code ever assigns a value to it and `SummarizeForPlanner` never reads it. **Remediation:** In the execution loop, track error message patterns (e.g., `undefined:`, `cannot use`) and set `MostCommonFailure` in the summary before calling `memory.UpdateMetrics`.

- [ ] **`parseFlags()` called twice** — `main_loop.go:4`, `main_exec.go:34` — `main()` calls `parseFlags()`, then `runExecutionMode()` → `execute()` calls `parseFlags()` a second time. Calling `flag.Parse()` twice on the same `FlagSet` is benign in current Go stdlib but is a latent source of confusion when flags are added or when the default `CommandLine` is replaced. **Remediation:** Remove the `parseFlags()` call from `execute()`.

- [x] **`audit/context.go:20` call-density estimation is a placeholder** — `audit/context.go:20` — The comment reads `// Placeholder for call density estimation` and the body sets `callDensity[pkgPath] = len(pkg.Imports)` (import count as proxy). Import count is a poor proxy for actual call density. No audit pass currently uses `CallDensity`, so the inaccuracy is dormant but will matter once passes are implemented. **Remediation:** When populating `PackageInfo` in `LoadPackages`, use `packages.NeedTypesInfo` to walk `TypesInfo.Uses` and count actual call sites per package. Remove the placeholder comment.

- [x] **`AuditContext.Hotspots` field always empty (context gap)** — `audit/context.go:40` — Even if `DetectHotspots` were called, the `AuditContext.Hotspots` field is passed to audit passes by value and no pass reads it (see HIGH finding). This is a second layer of the same gap: the wiring between metrics collection and pass analysis is absent.

- [x] **Run summary fields silently zero** — `main_exec.go:23-29` — Related to the HIGH finding above; from the observer's perspective the persisted `tasks.json` never has its fields cross-referenced against the run summary. A log reader would see `tasks_total: 0, tasks_completed: 0` for every run. **Remediation:** See HIGH finding on `RunSummary`.

- [x] **`go vet` warning: redundant newline** — `main_helper.go:91` — `fmt.Println("  orchestrator [options]\n")` has a trailing `\n`; `Println` already appends one. This produces a blank line in the usage output. **Remediation:** Remove the `\n` from the string literal.

### LOW

- [x] **`FormatContextForLLM` dead code already classified HIGH** (no additional LOW items).

- [ ] **GOALS.md Phase 2 observability not implemented** — GOALS.md §2.2 — Build failures should be saved to `logs/build_failures/<task_id>.log` and rejected patches to `logs/rejected_patches/<task_id>.diff`. Neither directory nor write logic exists. These are tracked items in GOALS.md; they are not critical to core operation. **Remediation:** In `execute()`, after `build()` returns a non-empty error string, write output to `logs/build_failures/<task_id>.log`. After `validatePatch` returns an error, write the diff to `logs/rejected_patches/<task_id>.diff`.

- [ ] **GOALS.md §2.3 run-summary markdown not generated** — GOALS.md §2.3 — `AUTONOMOUS_RUN_SUMMARY.md` is described as a per-run artefact. Only a machine-readable JSON is written to the `memory` branch. No Markdown summary is generated. This is a tracked item. **Remediation:** In `runExecutionMode()`, after `memory.SaveRun`, write a Markdown file with task counts, duration, and branch name.

---

## False Positives Considered and Rejected

| Candidate Finding | Reason Rejected |
|-------------------|----------------|
| `callLLM` always uses temperature 0.6 (no variation) | ROADMAP mentions adaptive temperature but it is listed as Milestone 9 (future). The current fixed value is intentional. |
| `memory/branch.go:checkoutBranch` wraps a one-liner | Intentional minimalism; wrapping improves testability. Not a gap. |
| `memory/runlog.go:trimOldRuns` uses index-based file removal | The LIFO order of `os.ReadDir` is lexicographic and run files are timestamped, so oldest files sort first. Correct behaviour. |
| `extractJSON` progressive-trim is O(n²) | Performance concern only at very large LLM responses (thousands of lines). Not an implementation gap for the stated use case. |
| `audit/loader.go` does not load type information | `packages.NeedTypesInfo` is expensive; not loading it is an acceptable trade-off until audit passes need symbol resolution. Flagged under the Exports gap, not separately. |
| `keyword()` uses only the first word of a task description for context matching | Intentional simplification documented in code comments and consistent with the project's "deterministic by default" principle. |
| `Task.Hash` field stored but not used for deduplication after initial bootstrap | `ensureTasksFile` returns immediately if `tasks.json` exists; hash deduplication is only needed at bootstrap time. Correct by design. |
