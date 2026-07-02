# Goal-Achievement Assessment

*Generated 2026-07-02. Baseline: `go test -race ./...` exits 0, `go vet ./...` exits 0.*

---

## Project Context

- **What it claims to do**: Autonomous engineering orchestrator for local LLMs — reads planning documents (`GAPS.md`, `GOALS.md`, `PLAN.md`, `ROADMAP.md`), generates atomic tasks, executes them against a local LLM (Qwen2.5/Qwen3 Coder via Ollama or any OpenAI-compatible endpoint), validates and applies unified-diff patches, commits clean git history — without human supervision.
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

- [x] In `ensureTasksFile` (or a new `mergeAuditFindings()` called immediately after), check if `audit_findings.json` exists. If it does, parse its `[]audit.Finding` and inject HIGH/CRITICAL severity findings as additional task descriptions into the task list, tagged with prefix `A`.
  - Reference: `main.go:ensureTasksFile`, `audit/findings.go:SaveFindings` (reverse: `LoadFindings`). Finding severity is already present in `audit.Finding.Severity`.
- [x] De-duplicate injected audit tasks against existing tasks by content hash (the same mechanism already used for planning-document tasks).
- [x] Re-order the task queue so tasks derived from HIGH/CRITICAL audit findings precede NORMAL tasks. Insert the ordering step in `nextExecutableTask` or as a priority field on `Task`.
- [x] **Validation**: run `--audit`, then run normally; confirm `orchestrator.log` contains tasks whose descriptions reference the audit finding messages.

---

### Priority 4 — Fix Dry-Run Build Invocation

**Why fourth**: Dry-run is described as a safe preview mode, but `build()` is called unconditionally on the unmodified working tree. Any pre-existing compilation error causes every dry-run task to report as failed. This makes dry-run unusable for its primary use case: validating a plan against a broken codebase.

- [x] In `main_exec.go`, gate `buildOut := build()` (line ≈180) on `!dryRun`. In dry-run mode, set `buildOut = ""` unconditionally and log `"dry_run_build_skipped"`.
  - Also gate `resolveBuildFailure(...)` on `!dryRun` — the fix loop has no meaning without an applied patch.
- [x] Update the dry-run log summary to report: `"patch_generated": true`, `"patch_valid": true`, `"would_apply": true` for passing tasks.
- [x] **Validation**: introduce a syntax error in the target repo, run `--dry-run`, confirm all tasks are reported as passing (no spurious build failures).

---

### Priority 5 — Harden Per-Task File Allowlist

**Why fifth**: `validateTouchedFiles` returns `nil` (skips allowlist) when `task.Files` is empty, which is the default for all planning-document-derived tasks. The path-containment check (`validateTouchedFilePaths`) prevents escape outside the repo root, partially mitigating the risk, but the per-task allowlist — a stated safety guarantee — is never consulted for the majority of tasks.

- [x] When `task.Files` is empty and `task.SymbolHint` is also empty, log a warning `"no_file_allowlist"` and rely on path-containment only (the current behaviour). This is safe but should be explicit.
- [x] During task generation in `ensureTasksFile` / `tasksFromSymbolMap`, populate `task.Files` from the filenames mentioned in the planning document source line that generated the task. Even a best-effort extraction (regex `\b\w+\.go\b`) is better than an empty allowlist.
  - Reference: `main_symtask.go:tasksFromSymbolMap` already populates `task.Files` for symbol-level tasks; apply the same logic to document-derived tasks in `main_tasks.go`.
- [x] **Validation**: generate a task from `GOALS.md` that mentions a specific file; confirm `validatePatch` rejects a diff touching a different file.

---

### Priority 6 — Replace Word-Count Token Budget with Character Budget

**Why sixth**: `enforceTokenBudget` counts whitespace-delimited words, but Go source code has many single-character tokens, identifier concatenations, and punctuation that BPE tokenisers count differently. A 1500-word budget on Go source regularly exceeds 2000–3000 BPE tokens, causing silent `context_length_exceeded` errors on 4k-context models (common in the Qwen2.5-7B range).

- [x] Replace the word-count implementation in `main_token_budget.go` with a character-count budget. At ~4 characters per token for Go source (a conservative estimate), `maxPromptTokens = 1500` translates to `maxPromptChars = 6000`. This matches the existing `compressPrompt` ceiling and is simple to implement without adding a dependency.
- [x] Document the approximation and its known failure modes in a comment above `enforceTokenBudget`. Note that `tiktoken-go` or a model-specific tokeniser would give exact counts if accurate budgeting becomes a priority.
- [x] **Validation**: construct a prompt just above 6000 characters; confirm `enforceTokenBudget` truncates it, and that the truncated form does not cause `context_length_exceeded` on a 4k-context model.

---

### Priority 7 — Decompose Critical-Path Functions to Meet Declared Invariants

**Why seventh**: The project enforces `max_function_length = 30` and `max_cyclomatic_complexity = 10` via `architecture/invariants.json` and `checkPostPatchInvariants`. The three most critical functions violate both. This matters specifically because `execute` is the primary target of `--self-evolve` mode: if the model cannot safely patch a 113-line function with CC=14, the self-improvement loop is impaired.

- [x] **`execute` (113 lines, CC=14) in `main_exec.go`**: extract the build-pass path into `handleBuildSuccess(tf, task, diff, stats, taskCache)` and the build-fail path into `handleBuildFailure(...)`. The per-iteration DAG/merge logic can become `advanceTaskFile(tf) bool`. Target: three functions of ≤40 lines each.
- [x] **`collectSymbolInfos` (71 lines, CC=13) in `audit/context.go`**: the AST visitor and the symbol extraction loop are independent concerns. Extract `visitNode(node ast.Node, info *symbolInfo)` and `buildSymbolMap(fset, files) SymbolMap`. Target: ≤30 lines each.
- [x] **`DeriveLayersFromGraph` (53 lines, CC=13) in `audit/graph.go`**: the layer-assignment loop and the violation-detection loop are separable. Extract `assignLayers(graph) map[string]int` and `detectLayerViolations(graph, layers) []LayerViolation`. Target: ≤30 lines each.
- [x] After each decomposition, re-run `checkPostPatchInvariants` to confirm compliance, then run `go test -race ./...`.
- [x] **Validation**: `go-stats-generator analyze . --skip-tests` reports zero functions exceeding the declared invariant limits.

---

### Priority 8 — Improve Documentation Coverage to 70 %

**Why eighth**: `go-stats-generator` reports 45.8 % function-level documentation coverage. The audit pass (`RunAPIPass`) flags undocumented exported symbols as findings. The orchestrator generates findings about its own packages, then discards them (Priority 3 gap). Raising coverage reduces self-generated audit noise and improves the context quality the model receives when editing documented functions.

- [ ] Focus first on exported functions in `audit/` and `memory/` — these are the packages most likely to be used or imported externally. The `audit` package currently has 88 functions with low per-function doc coverage.
  - Reference: `go-stats-generator` output lists the 22 functions with `has_comment: false` that are exported; prioritise those.
- [ ] For `main` package internal functions on the execution hot-path (`execute`, `getDiffForTask`, `resolveBuildFailure`, `ensureTasksFile`), add brief one-line doc comments explaining the contract (inputs, outputs, side effects) — not prose, just facts.
- [ ] **Validation**: `go-stats-generator analyze . --skip-tests` reports overall function documentation coverage ≥ 70 %.

---

### Priority 9 — Pass Previous Diff to Fix Loop + Dynamic Temperature

**Why**: The retry loop (`fixTask` / `attemptBuildFixRetries`) is the hottest consumer of inference cycles. On each retry, `fixTask` receives the task description, current file context, and compiler hints — but **not the diff that produced the last failure**. Without this context, a quantized model will frequently regenerate the same broken patch, producing the oscillation events already tracked by `recordRetryConvergence`. In addition, temperature is hardcoded at 0.6 for every retry regardless of whether the failure looks deterministic or exploratory.

- [ ] Add a `previousDiff string` parameter to `fixTask` in `main_tasks.go`. When non-empty, append a `PREVIOUS_ATTEMPT (failed):` block containing the first 20 lines of the prior diff to the FIX prompt. The 20-line cap prevents this from consuming the context budget.
  - Reference: `main_exec.go:attemptBuildFixRetries` accumulates `appliedFixDiffs`; pass `appliedFixDiffs[len-1]` as `previousDiff` on each call after the first retry.
- [ ] Add a `tempForRetry(retryCount int) float64` helper returning `0.3` on retry 1 (conservative restatement), `0.7` on retry 2 (exploratory), `0.5` on retry 3+ (balanced). Replace the hardcoded `0.6` in `fixTask` with this helper.
- [ ] When `recordRetryConvergence` detects two consecutive identical failure categories, immediately force the next retry to use the architect model (`roleModel(architectModelName)`) and temperature 0.8 rather than continuing with the executor model.
  - Reference: `main_exec.go:attemptBuildFixRetries:297` is where the oscillation is detected; route model selection there instead of only logging.
- [ ] **Validation**: compare `retry_convergence_alerts / retry_convergence_samples` before and after across a sample run. Target: ratio drops below 20 %. Also confirm `go test -race ./...` exits 0 after the change.

---

### Priority 10 — Per-file Context Cap + Signature-only Fallback

**Why**: `gatherFileContext` reads each file with `os.ReadFile` and concatenates raw bytes without any per-file size limit. A single large file (e.g. `main_exec.go` at ~350 lines) can consume the entire 6,000-character prompt budget, cutting off the task description and all subsequent context files. This is the dominant cause of first-attempt inference failures on tasks that reference multiple files.

- [ ] Add `const maxBytesPerFile = 2000` to `main.go`. In `gatherFileContext`, truncate each file's contribution to `maxBytesPerFile` bytes before appending. When a file exceeds the cap, replace its body with the output of a new `extractSignatures(data []byte) []byte` helper that retains only lines beginning with `func `, `type `, `var `, or `const ` — preserving the callable surface without body detail.
  - Reference: `main.go:gatherFileContext` (lines ≈169-179); `main_context.go:funcScopedContext` already does a variant of this for function bodies.
- [ ] In `funcScopedContext`, when `extractBoundaryContext` would return more than `maxBytesPerFile` bytes for a single function body, truncate at the boundary and append `"// ... (truncated)"` so the model is aware.
- [ ] Adjust `promptCharBudget` (currently 6000) to account for the memory preamble (~400 chars) and execution block (~300 chars), leaving ~5300 chars for actual file context. Document this budget split in a comment above `compressPrompt`.
  - Reference: `main_memory.go:11`.
- [ ] **Validation**: create a task referencing a file > 2000 chars; confirm the prompt sent to the model (visible in verbose mode) contains the signature-only version and total prompt length stays under 6000 chars.

---

### Priority 11 — Intra-subtask Sequential Dependency Wiring

**Why**: When `splitTask` or `splitMultiFileTask` decomposes a parent task into N subtasks, all N are set to `status: pending` with the parent's original deps — but no intra-group ordering is established. Two subtasks targeting the same file can both become eligible simultaneously. When the second subtask's patch tool applies against the tree already modified by the first, the context lines no longer match, producing a `patch_apply_failed` event that marks the subtask blocked with no meaningful error.

- [ ] In `splitTask` (`main_tasks.go`), after building the subtask slice, wire sequential deps: `subtasks[i].DependsOn = append(subtasks[i].DependsOn, subtasks[i-1].ID)` for `i ≥ 1`. This guarantees the second subtask cannot start until the first has committed.
- [ ] Apply the same sequential wiring in `splitMultiFileTask` and `splitOversizedDescription`.
  - Exception: skip sequential wiring when `symbolTasksForFiles` confirms the subtasks target non-overlapping line ranges in the same file (i.e. `fbs[i].EndLine < fbs[i+1].StartLine`). This preserves true independence where it exists.
- [ ] In `ensureTasksFile`, add a post-generation pass `injectFileOverlapDeps(tasks []Task) []Task` that inspects the `task.Files` list of every pair of pending tasks: if task B appears after task A in document order and both list file F, add A's ID to B's `DependsOn`.
  - Reference: `main.go:ensureTasksFile` — call `injectFileOverlapDeps` before `saveTasks`.
- [ ] **Validation**: generate two tasks that both reference the same file; confirm `nextExecutableTask` only returns the second after the first is `complete`. Measure `patch_apply_failed` rate for tasks with `.s\d` suffixes in their IDs before and after.

---

### Priority 12 — Tiered Deletion-Ratio Guard by ChangeType

**Why**: `validateDeletionRatio` applies a hard 30 % deletion cap to all patches regardless of semantic intent. A `MODIFY_FUNCTION` task that rewrites a function body legitimately deletes 40–60 % of lines while replacing them with cleaner code. The hard cap forces such tasks into a split-and-rewrite pattern requiring 2–3× more inference calls. Conversely, `INSERT_FUNCTION` tasks should be held to a stricter cap (≤10 % deletions) because insertions should rarely need to remove existing code.

- [ ] Change `validateDeletionRatio(diff string)` to `validateDeletionRatio(diff string, task *Task)` in `main_validatepatch.go`. Compute the cap via a new `deletionCapForChangeType(ct ChangeType) float64` helper:
  - `DELETE_FUNCTION` → `0.70`
  - `MODIFY_FUNCTION`, `MODIFY_STRUCT` → `0.50`
  - `INSERT_FUNCTION`, `ADD_IMPORT` → `0.10`
  - `GENERAL` or unset → `0.30` (current behaviour unchanged)
- [ ] Add `DELETE_FUNCTION ChangeType = "DELETE_FUNCTION"` to `main_dsl.go` alongside the existing change type constants.
- [ ] Update the `validatePatch` call to `validateDeletionRatio` to pass the task. Update `validateDSLSchema` to accept `DELETE_FUNCTION` as a valid type.
  - Reference: `main_validatepatch.go:runValidationSteps` — the deletion-ratio step at position 6 currently passes no task context.
- [ ] **Validation**: audit the last 20 entries in `logs/rejected_patches/` for the rejection reason `"patch deletes more than 30%"`. Confirm the tiered cap would have accepted the legitimate refactors. Run `go test -race ./...` after the change.

---

### Priority 13 — Structured Build Failure Artifacts + Inference Latency Logging

**Why**: Build failure artifacts are written as raw text (`.log` files), and inference latency is not measured anywhere. Without latency data, it is impossible to determine whether throughput is bottlenecked by model inference time or by patch-application / build overhead. Without structured failure artifacts, identifying which error categories repeat across runs requires log grep and is not feed-able to the adaptive metrics system.

- [ ] In `callLLMWithModel` (`main.go`), wrap the `http.Post` call with `start := time.Now()` / `latencyMs := time.Since(start).Milliseconds()` and include `latency_ms=%d` in the existing `token_usage` log event.
  - Reference: `main.go:270-298`; the log event already records model, prompt tokens, and completion tokens.
- [ ] Add `AvgInferenceLatencyMs float64` to `memory.RunSummary` and `memory.AdaptiveMetrics` in `memory/types.go`. Feed the per-call latency into `executionStats` during the run and roll it into the `RunSummary` at completion in `runExecutionMode`.
- [ ] Replace the raw-text build failure artifact in `writeBuildFailure` (`main_observability.go`) with a JSON envelope:
  ```
  { "task_id": "…", "timestamp": "…", "retry": N, "error_category": "…", "error_lines": ["…"], "raw": "…" }
  ```
  Write to `logs/build_failures/<task_id>.json` (keep the existing `.log` path as an alias or remove it). Parse `classifyBuildFailure` and `compilerErrorLines` to populate the structured fields.
  - Reference: `main_observability.go:writeBuildFailure`; `main_util.go:classifyBuildFailure`, `compilerErrorLines`.
- [ ] Persist `AvgPatchConfidence` and `AvgPatchRisk` to `AdaptiveMetrics`, fed from `evaluatePatchConfidence` and `scorePatchRisk` in `recordSuccessfulPatch`.
  - Reference: `main_exec.go:recordSuccessfulPatch` already calls both but discards the scores.
- [ ] **Validation**: after 3 runs, confirm `adaptive_metrics.json` contains `avg_inference_latency_ms`; confirm `logs/build_failures/` contains `.json` files parseable by `encoding/json`. Run `go test -race ./...`.

---

## Resolved Gaps (No Action Required)

The following issues were previously documented in `GAPS.md` and are confirmed resolved in the current codebase:

| Gap | Resolution |
|---|---|
| G-05: `go.mod` declared `go 1.26.1` (nonexistent) | Go 1.26.1 exists as of 2026-07-02; `go test -race ./...` and `go vet ./...` both exit 0 with the declared toolchain. Not a real gap. |
| G-07: Context matching used only the first word of task description | `main_context.go:matchSymbol` now iterates all symbols with whole-word matching across the full description, preferring the longest name. Sophisticated and correct. |
| G-08: `UpdateMetrics` performed mid-run branch checkout | `memory/metrics.go:LoadMetricsFromBranch` was added for read-path (no checkout). `UpdateMetrics` is called only in `runExecutionMode` after `execute()` returns (post-run, not mid-patch). Risk remains for a crash during the final memory commit, but it no longer threatens in-flight patches. |
| F-09: LLM errors silently swallowed | `main_exec.go` propagates errors from `applyDiffToWorkspace` and logs them; the task is marked blocked rather than silently skipped. |

---

*Metrics source: `go-stats-generator analyze . --skip-tests` (2026-07-02), 47 files, 3368 LOC, 3 packages.*
*Baseline: `go test -race ./...` exits 0 · `go vet ./...` exits 0 · duplication ratio 0.23% · no circular dependencies.*
