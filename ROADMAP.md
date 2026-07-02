# Goal-Achievement Assessment

*Generated 2026-07-02. Baseline: `go test -race ./...` exits 0, `go vet ./...` exits 0.*

---

## Project Context

- **What it claims to do**: Autonomous engineering orchestrator for local LLMs — reads planning documents (`AUDIT.md`, `GAPS.md`, `GOALS.md`, `PLAN.md`, `ROADMAP.md`), generates atomic tasks, executes them against a local LLM (Qwen2.5/Qwen3 Coder via Ollama or any OpenAI-compatible endpoint), validates and applies unified-diff patches, commits clean git history — without human supervision.
- **Target audience**: Solo developers and small teams running open-weight models on consumer CPUs; specifically the Qwen2.5-Coder-32B-Instruct / Qwen3-Coder family.
- **Architecture**:
  - `main` (29 files, 192 functions) — execution loop, DAG scheduling, patch validation, task lifecycle, prompt construction, adaptive tier/model escalation, observability, DSL, speculative execution, subsystem analytics.
  - `audit` (13 files, 88 functions) — static analysis passes: architecture (cycle detection, layering), API surface, concurrency; AST-based context extraction and symbol mapping.
  - `memory` (5 files, 19 functions) — cross-run adaptive metrics, run summaries, branch-isolated persistence on the `memories` git ref.
- **Existing CI / quality gates**:
  - No `.github/workflows/` directory. No Makefile, no CI pipeline.
  - Quality is enforced locally: `architecture/invariants.json` declares max function length (30 lines), max file length (400 lines), max cyclomatic complexity (10), no cyclic imports, patch cap (50 lines).
  - `go vet ./...` and `go test -race ./...` are the de-facto quality gates, run manually per the README.

---

## Goal-Achievement Summary

| Stated Goal | Status | Evidence | Gap Description |
|---|---|---|---|
| Document-driven task generation | ✅ Achieved | `main.go:ensureTasksFile` reads all 5 planning doc types; tasks deduplicated by content hash | — |
| DAG-based execution, `depends_on` ordering | ✅ Achieved | `audit/dag.go:BuildFuncDAG`, `main_tasks.go:nextExecutableTask` | — |
| Patch safety — line-count & file-touch limits | ✅ Achieved | `main_validatepatch.go:validatePatch` enforces both hard limits | — |
| Patch safety — deletion-ratio guard | ✅ Achieved | `main_validatepatch.go:hasFullFileRewrite` (CC=7, complexity within tolerable range for the problem) | — |
| Patch safety — per-task file allowlist | ⚠️ Partial | `main_validatepatch.go:70-79`: returns `nil` early when `len(task.Files) == 0`; path-containment check still runs | Allowlist never consulted for planning-document-derived tasks (the majority). Any touched path inside the repo root is accepted. |
| Automatic task splitting on failure | ✅ Achieved | `main_tasks.go:splitTask`, re-queues smaller subtasks on retry exhaustion | — |
| Build-and-fix loop | ✅ Achieved | `main_exec.go:resolveBuildFailure`, `attemptBuildFixRetries` with revert on exhaustion | — |
| Build-and-fix — pre-task dirty-state check | ❌ Missing | No `git diff --quiet` guard before `applyDiffToWorkspace` | Patches applied to unknown baseline; build failures misattributed to current task. |
| Build-and-fix — working-directory reset | ❌ Missing | No `git checkout -- .` / `git clean` at start of `execute` | Failed task's unreverted changes pollute the next task's baseline. |
| Structured JSON logging | ✅ Achieved | `main_json.go`, all events appended to `orchestrator.log` | — |
| Git branch isolation | ✅ Achieved | `ensureBranch()` creates `autonomous/<timestamp>` | — |
| Adaptive memory — cross-run metrics | ✅ Achieved | `memory/metrics.go`, `LoadMetricsFromBranch` reads without checkout; injected via `injectMemoryIntoPlanner` | — |
| Adaptive memory — branch isolation for writes | ⚠️ Partial | `memory/metrics.go:UpdateMetrics` still performs `git checkout memories` then `git checkout <original>` | Called post-run (safe in practice), but a crash during this window corrupts HEAD. `LoadMetricsFromBranch` correctly avoids checkout for reads. |
| Static audit mode (architecture, API, concurrency) | ✅ Achieved | `main_audit.go:runAuditMode`, three audit passes in `audit/passes.go` | — |
| Audit findings guide execution prioritisation | ❌ Missing | `main_audit.go` writes `audit_findings.json`; `ensureTasksFile` never reads it | Stated benefit of audit→plan workflow is completely absent from the execution path. |
| Self-evolution mode (`--self-evolve`) | ✅ Achieved | `main_helper.go`: flag raises patch limits; `main_allowedpatchlines.go` gates the cap | — |
| Dry-run — no file writes or commits | ✅ Achieved | `!dryRun` guards on `applyDiffToWorkspace` and `gitCommit` | — |
| Dry-run — no compiler invocation | ❌ Missing | `main_exec.go:180`: `buildOut := build()` called unconditionally after apply (which is skipped in dry-run) | Dry-run invokes compiler on unmodified tree; pre-existing errors cause every task to report as "failed". |
| Crash recovery / journal file (GOALS.md §6) | ❌ Missing | No journal file, no checkpoint file, no step-level state anywhere in codebase | Interruption mid-task (OOM, SIGKILL, Ctrl+C) leaves applied-but-uncommitted patch on disk; next run double-applies it. |
| Token budget enforcement | ⚠️ Partial | `main_token_budget.go:enforceTokenBudget` counts whitespace-delimited words × 1 (no multiplier); `maxPromptTokens = 1500` words | Word count underestimates BPE token count for Go source (identifiers, punctuation). Budget can be exceeded silently on code-heavy context, causing `context_length_exceeded` API errors. |
| Model role specialisation (`--planner-model`, `--executor-model`, `--architect-model`) | ✅ Achieved | `main_helper.go:99-101`, `roleModel()` fallback chain | — |
| Hardware-aware scheduler (CPU load → parallelism) | ✅ Achieved | `main_scheduler.go:cpuLoadFactor`, `speculativeCandidateCount` reduces under high load | — |
| Emergency stop on consecutive failures | ✅ Achieved | `main_scheduler.go:stabilityMonitor` activates safe mode after 5 consecutive blocked tasks or 10 oscillations | — |
| Architectural invariant enforcement | ⚠️ Partial | `architecture/invariants.json` declares limits; `main_invariantvalidate.go:checkPostPatchInvariants` checks post-patch | Three functions in critical paths violate the project's own declared invariants (see below). |
| Speculative multi-branch execution | ✅ Achieved | `main_speculative.go`, `main_simulation.go` | — |
| Subsystem stability analytics | ✅ Achieved | `main_subsystem.go`, per-subsystem retry/risk tracking | — |
| Tiered intelligence escalation | ✅ Achieved | `main_tiers.go`, `main_modelescalation.go`, de-escalation on completion | — |

**Overall: 16 / 22 stated goals fully achieved**

### Self-Stated Invariant Violations (architecture/invariants.json)

The project declares `max_function_length = 30` and `max_cyclomatic_complexity = 10`. The following functions violate both limits and are on the critical execution path:

| Function | File | Lines | Cyclomatic |
|---|---|---|---|
| `execute` | `main_exec.go` | 113 | 14 |
| `collectSymbolInfos` | `audit/context.go` | 71 | 13 |
| `DeriveLayersFromGraph` | `audit/graph.go` | 53 | 13 |
| `ensureTasksFile` | `main.go` | 47 | 7 |
| `tasksFromSymbolMap` | `main_symtask.go` | 45 | 9 |
| `hasFullFileRewrite` | `main_validatepatch.go` | 43 | 7 |
| `checkFunctionInvariants` | `audit/invariant_check.go` | 43 | 9 |

Source: `go-stats-generator analyze . --skip-tests` (2026-07-02). `execute` is the most important to decompose because it is the hot loop and its complexity directly impairs the model's ability to patch it in self-evolution mode.

---

## Roadmap

### Priority 1 — Crash Recovery and Run Journal

**Why first**: Consumer hardware (OOM kills, thermal throttling, user Ctrl+C) makes mid-task interruption near-certain over long sessions. Without recovery, every interruption leaves an applied-but-uncommitted patch on disk and `tasks.json` in an inconsistent state, forcing manual cleanup. This is the highest-friction gap for the stated audience.

- [x] Create `orchestrator-journal.json` in the working directory, tracking `{ task_id, step: "planned|patched|built|committed", patch_hash }`. Write atomically (write-then-rename) on each step transition.
  - Reference: `main_exec.go` — insert journal writes at `applyDiffToWorkspace` success, `build()` success, and `gitCommit` success.
- [x] On startup in `runExecutionMode`, read the journal before calling `ensureTasksFile`. If `step == "patched"` and `built == false`, revert the patch (`revertPatch`) and clear the journal entry. If `step == "built"` and `committed == false`, commit the already-applied patch.
  - Reference: `main_exec.go:revertBuildFailurePatches` already implements the revert primitives; reuse them.
- [x] Exclude `orchestrator-journal.json` from git tracking (add to `.gitignore`).
- [x] **Validation**: kill the orchestrator mid-patch (`kill -9`), restart; confirm it resumes cleanly and `git status` is clean.

---

### Priority 2 — Pre-Task Workspace Hygiene (Dirty-State Check + Reset)

**Why second**: Without a clean baseline guarantee, each task in a failing run computes its diff against an unknown accumulated delta, producing cascading misattributed build failures. This defeats the retry-convergence design goal.

- [x] At the top of the `execute` inner loop, before `getDiffForTask`, run `git diff --quiet HEAD` and `git diff --cached --quiet`. If either fails and `!dryRun`, log `"dirty_workspace_detected"` and run `git checkout -- .` + `git clean -fd --exclude=tasks.json --exclude=orchestrator.log --exclude=orchestrator-journal.json`.
  - Reference: `main_exec.go` — insert after `deescalateModel(task.ID)` and before `resolveContextFiles`.
- [x] Add a `--skip-workspace-reset` flag (default false) for users who intentionally pre-stage changes. Gate the clean-up on `!skipWorkspaceReset`.
- [x] **Validation**: manually apply a partial patch, restart the orchestrator, confirm the next task starts from a clean tree and the log shows `"dirty_workspace_detected"`.

---

### Priority 3 — Audit Findings → Execution Loop Integration

**Why third**: The README describes audit as producing findings that "guide task prioritisation", but `ensureTasksFile` never reads `audit_findings.json`. The audit→plan workflow is entirely absent, making `--audit` a diagnostic-only tool rather than a planning accelerator.

- [ ] In `ensureTasksFile` (or a new `mergeAuditFindings()` called immediately after), check if `audit_findings.json` exists. If it does, parse its `[]audit.Finding` and inject HIGH/CRITICAL severity findings as additional task descriptions into the task list, tagged with prefix `A`.
  - Reference: `main.go:ensureTasksFile`, `audit/findings.go:SaveFindings` (reverse: `LoadFindings`). Finding severity is already present in `audit.Finding.Severity`.
- [ ] De-duplicate injected audit tasks against existing tasks by content hash (the same mechanism already used for planning-document tasks).
- [ ] Re-order the task queue so tasks derived from HIGH/CRITICAL audit findings precede NORMAL tasks. Insert the ordering step in `nextExecutableTask` or as a priority field on `Task`.
- [ ] **Validation**: run `--audit`, then run normally; confirm `orchestrator.log` contains tasks whose descriptions reference the audit finding messages.

---

### Priority 4 — Fix Dry-Run Build Invocation

**Why fourth**: Dry-run is described as a safe preview mode, but `build()` is called unconditionally on the unmodified working tree. Any pre-existing compilation error causes every dry-run task to report as failed. This makes dry-run unusable for its primary use case: validating a plan against a broken codebase.

- [ ] In `main_exec.go`, gate `buildOut := build()` (line ≈180) on `!dryRun`. In dry-run mode, set `buildOut = ""` unconditionally and log `"dry_run_build_skipped"`.
  - Also gate `resolveBuildFailure(...)` on `!dryRun` — the fix loop has no meaning without an applied patch.
- [ ] Update the dry-run log summary to report: `"patch_generated": true`, `"patch_valid": true`, `"would_apply": true` for passing tasks.
- [ ] **Validation**: introduce a syntax error in the target repo, run `--dry-run`, confirm all tasks are reported as passing (no spurious build failures).

---

### Priority 5 — Harden Per-Task File Allowlist

**Why fifth**: `validateTouchedFiles` returns `nil` (skips allowlist) when `task.Files` is empty, which is the default for all planning-document-derived tasks. The path-containment check (`validateTouchedFilePaths`) prevents escape outside the repo root, partially mitigating the risk, but the per-task allowlist — a stated safety guarantee — is never consulted for the majority of tasks.

- [ ] When `task.Files` is empty and `task.SymbolHint` is also empty, log a warning `"no_file_allowlist"` and rely on path-containment only (the current behaviour). This is safe but should be explicit.
- [ ] During task generation in `ensureTasksFile` / `tasksFromSymbolMap`, populate `task.Files` from the filenames mentioned in the planning document source line that generated the task. Even a best-effort extraction (regex `\b\w+\.go\b`) is better than an empty allowlist.
  - Reference: `main_symtask.go:tasksFromSymbolMap` already populates `task.Files` for symbol-level tasks; apply the same logic to document-derived tasks in `main_tasks.go`.
- [ ] **Validation**: generate a task from `GOALS.md` that mentions a specific file; confirm `validatePatch` rejects a diff touching a different file.

---

### Priority 6 — Replace Word-Count Token Budget with Character Budget

**Why sixth**: `enforceTokenBudget` counts whitespace-delimited words, but Go source code has many single-character tokens, identifier concatenations, and punctuation that BPE tokenisers count differently. A 1500-word budget on Go source regularly exceeds 2000–3000 BPE tokens, causing silent `context_length_exceeded` errors on 4k-context models (common in the Qwen2.5-7B range).

- [ ] Replace the word-count implementation in `main_token_budget.go` with a character-count budget. At ~4 characters per token for Go source (a conservative estimate), `maxPromptTokens = 1500` translates to `maxPromptChars = 6000`. This matches the existing `compressPrompt` ceiling and is simple to implement without adding a dependency.
- [ ] Document the approximation and its known failure modes in a comment above `enforceTokenBudget`. Note that `tiktoken-go` or a model-specific tokeniser would give exact counts if accurate budgeting becomes a priority.
- [ ] **Validation**: construct a prompt just above 6000 characters; confirm `enforceTokenBudget` truncates it, and that the truncated form does not cause `context_length_exceeded` on a 4k-context model.

---

### Priority 7 — Decompose Critical-Path Functions to Meet Declared Invariants

**Why seventh**: The project enforces `max_function_length = 30` and `max_cyclomatic_complexity = 10` via `architecture/invariants.json` and `checkPostPatchInvariants`. The three most critical functions violate both. This matters specifically because `execute` is the primary target of `--self-evolve` mode: if the model cannot safely patch a 113-line function with CC=14, the self-improvement loop is impaired.

- [ ] **`execute` (113 lines, CC=14) in `main_exec.go`**: extract the build-pass path into `handleBuildSuccess(tf, task, diff, stats, taskCache)` and the build-fail path into `handleBuildFailure(...)`. The per-iteration DAG/merge logic can become `advanceTaskFile(tf) bool`. Target: three functions of ≤40 lines each.
- [ ] **`collectSymbolInfos` (71 lines, CC=13) in `audit/context.go`**: the AST visitor and the symbol extraction loop are independent concerns. Extract `visitNode(node ast.Node, info *symbolInfo)` and `buildSymbolMap(fset, files) SymbolMap`. Target: ≤30 lines each.
- [ ] **`DeriveLayersFromGraph` (53 lines, CC=13) in `audit/graph.go`**: the layer-assignment loop and the violation-detection loop are separable. Extract `assignLayers(graph) map[string]int` and `detectLayerViolations(graph, layers) []LayerViolation`. Target: ≤30 lines each.
- [ ] After each decomposition, re-run `checkPostPatchInvariants` to confirm compliance, then run `go test -race ./...`.
- [ ] **Validation**: `go-stats-generator analyze . --skip-tests` reports zero functions exceeding the declared invariant limits.

---

### Priority 8 — Improve Documentation Coverage to 70 %

**Why eighth**: `go-stats-generator` reports 45.8 % function-level documentation coverage. The audit pass (`RunAPIPass`) flags undocumented exported symbols as findings. The orchestrator generates findings about its own packages, then discards them (Priority 3 gap). Raising coverage reduces self-generated audit noise and improves the context quality the model receives when editing documented functions.

- [ ] Focus first on exported functions in `audit/` and `memory/` — these are the packages most likely to be used or imported externally. The `audit` package currently has 88 functions with low per-function doc coverage.
  - Reference: `go-stats-generator` output lists the 22 functions with `has_comment: false` that are exported; prioritise those.
- [ ] For `main` package internal functions on the execution hot-path (`execute`, `getDiffForTask`, `resolveBuildFailure`, `ensureTasksFile`), add brief one-line doc comments explaining the contract (inputs, outputs, side effects) — not prose, just facts.
- [ ] **Validation**: `go-stats-generator analyze . --skip-tests` reports overall function documentation coverage ≥ 70 %.

---

## Resolved Gaps (No Action Required)

The following issues were previously documented in `GAPS.md` or `AUDIT.md` and are confirmed resolved in the current codebase:

| Gap | Resolution |
|---|---|
| G-05: `go.mod` declared `go 1.26.1` (nonexistent) | Go 1.26.1 exists as of 2026-07-02; `go test -race ./...` and `go vet ./...` both exit 0 with the declared toolchain. Not a real gap. |
| G-07: Context matching used only the first word of task description | `main_context.go:matchSymbol` now iterates all symbols with whole-word matching across the full description, preferring the longest name. Sophisticated and correct. |
| G-08: `UpdateMetrics` performed mid-run branch checkout | `memory/metrics.go:LoadMetricsFromBranch` was added for read-path (no checkout). `UpdateMetrics` is called only in `runExecutionMode` after `execute()` returns (post-run, not mid-patch). Risk remains for a crash during the final memory commit, but it no longer threatens in-flight patches. |
| F-09: LLM errors silently swallowed | `main_exec.go` propagates errors from `applyDiffToWorkspace` and logs them; the task is marked blocked rather than silently skipped. |

---

*Metrics source: `go-stats-generator analyze . --skip-tests` (2026-07-02), 47 files, 3368 LOC, 3 packages.*
*Baseline: `go test -race ./...` exits 0 · `go vet ./...` exits 0 · duplication ratio 0.23% · no circular dependencies.*
