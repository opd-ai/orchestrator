# UNIVERSAL BUG AUDIT (END-TO-END) — 2026-07-02

## Project Profile

**Purpose**: Autonomous engineering orchestrator for local LLMs. Reads planning documents, generates atomic tasks, executes them against a local LLM (Qwen2.5/Qwen3 Coder via Ollama or OpenAI-compatible endpoint), validates and applies patches, commits clean git history — without human supervision.

**Target users**: Solo developers and small teams running open-weight models on consumer CPUs.

**Deployment model**: Single-process CLI tool, runs against a local git repository. Connects to a local HTTP endpoint (LLM server). Not exposed to the internet; threat model includes malicious/corrupt LLM responses.

**Critical paths**:
1. `ensureTasksFile` → `generateTasksFromDoc` (planning)
2. `execute` → `getDiffForTask` → `validatePatch` → `applyDiffToWorkspace` → `build` (execution loop)
3. `resolveBuildFailure` (fix/retry loop)
4. `memory.UpdateMetrics` / `memory.SaveRun` (cross-run persistence)
5. `audit.RunArchitecturePass` / `RunAPIPass` / `RunConcurrencyPass` (audit mode)

---

## Audit Scope

| Package | Files | Functions | Role |
|---------|-------|-----------|------|
| `github.com/opd-ai/orchestrator` (main) | 29 | 183 | Core execution loop, CLI, patch validation, task management |
| `github.com/opd-ai/orchestrator/audit` | 13 | 88 | Static analysis passes, symbol mapping, dependency graph |
| `github.com/opd-ai/orchestrator/memory` | 5 | 19 | Cross-run metrics persistence, adaptive planner data |

**go-stats-generator metrics**: 268 total functions, 3 packages, 3220 LOC. Longest function: `execute` (113 lines, complexity 20.2). Highest complexity: `collectSymbolInfos` (audit/context.go, complexity 20.4). Functions above complexity 15: 3. Average complexity: 4.5.

**Baseline**: `go test -race ./...` — all 3 packages pass (0 race conditions detected). `go vet ./...` — 0 warnings.

---

## Coverage Log

| Package | 3b Logic | 3c Nil | 3d Errors | 3e Resources | 3f Concurrency | 3g Security | 3h Aliasing | 3i Init | 3j API |
|---------|----------|--------|-----------|--------------|----------------|-------------|-------------|---------|--------|
| main (all 29 files) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| audit (all 13 files) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| memory (all 5 files) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |

---

## Goal-Achievement Summary

| Stated Goal | Status | Blocking Findings |
|-------------|--------|-------------------|
| Patch safety (file-touch limits, deletion-ratio guards) | ⚠️ | F-01: allowlist bypassed when task has no files |
| Automatic task splitting on failure | ✅ | — |
| Build-and-fix loop | ⚠️ | F-02: dirty workspace carried into split tasks |
| Git branch isolation | ✅ | — |
| Structured JSON logging | ⚠️ | F-12: log file errors silently swallowed |
| Adaptive memory (cross-run metrics) | ⚠️ | F-09, F-13: silent failures in persistence |
| Static audit mode | ⚠️ | F-03: cluster IDs are Unicode codepoints, not numbers |
| Dry-run mode | ⚠️ | build() still runs in dry-run (potential false failure signal) |
| Reset working directory before each task (GOALS.md §1.2) | ❌ | F-02: not implemented |
| Detect dirty git state before applying patch (GOALS.md §1.2) | ❌ | G-02 |
| Crash recovery / journal file (GOALS.md §6) | ❌ | G-03 |

---

## Findings

### CRITICAL

- [x] **F-01: Path traversal — file allowlist bypassed when `task.Files` is empty** — `main_validatepatch.go:72-74` — Security/Logic — `validateTouchedFiles` returns `nil` (no-op) when `len(task.Files) == 0`. A malicious or jailbroken LLM can produce a diff with `+++ b/../../../etc/cron.d/evil` and the orchestrator will apply it to the filesystem. Any task generated from planning documents (AUDIT.md, GOALS.md, etc.) has `task.Files == []` by default (files are only set by symbol-task splitter for single-file tasks). Call path: `execute` → `validatePatch` → `validateTouchedFiles(touchedFiles, allowedFiles, task)` — when `len(task.Files) == 0`, the function returns nil on line 74. **Remediation**: In `validateTouchedFiles`, when `len(task.Files) == 0`, validate each touched file path against the repository root using `filepath.Clean` and `strings.HasPrefix` to ensure it stays within the working directory. Add: `if !isSafePath(file) { return fmt.Errorf("file %q escapes repository root", file) }`. Validate with `go test -race ./... && go vet ./...`.

- [x] **F-02: Dirty workspace not reverted on build-fix loop exhaustion** — `main_exec.go:229-264` — Resource/Logic — When `resolveBuildFailure` exhausts `maxRetries` (or a `validatePatch`/`applyPatch` call fails), the function calls `splitTask` without reverting the patches already applied to the working directory. The workspace contains the original failed diff plus any partial fix diffs. The split subtasks then execute against this broken state, producing diffs that assume a corrupted baseline. GOALS.md Phase 1.2 explicitly requires "Reset working directory before each task." Call path: `execute` → `applyDiffToWorkspace` (succeeds) → `build()` (fails) → `resolveBuildFailure` → retries exhausted → `splitTask`. No `revertPatch` call occurs. **Remediation**: In `resolveBuildFailure`, before calling `splitTask`, revert the original diff: `if !dryRun { revertPatch(originalDiff) }` where `originalDiff` is the first `diff` argument. Also revert any fix diffs accumulated in the loop. Validate with `go test -race ./...`.

---

### HIGH

- [x] **F-03: `ClusterPackages` generates invalid cluster IDs using Unicode codepoints** — `audit/cluster.go:15` — Logic — `"cluster_" + string(rune(clusterID))` converts the integer `clusterID` as a Unicode code point, not a decimal string. For `clusterID=0`, the result is `"cluster_\x00"` (NUL byte); for `clusterID=65`, it is `"cluster_A"`. The cluster IDs written into `Finding.Package` and logged are meaningless or contain control characters. Expected behaviour (and what callers assume) is `"cluster_0"`, `"cluster_1"`, etc. **Remediation**: Replace with `fmt.Sprintf("cluster_%d", clusterID)` or `"cluster_" + strconv.Itoa(clusterID)` at `audit/cluster.go:15`. Validate with `go test ./audit/...`.

- [x] **F-04: `bytesTrim` panics on empty byte slice** — `memory/branch.go:32` — Nil/Boundary — `bytesTrim(b)` computes `b[:len(b)-1]`. When `len(b) == 0`, this evaluates to `b[:-1]` which is a negative slice bound and panics with `runtime error: slice bounds out of range`. `currentBranch()` calls `bytesTrim(out)` on `exec.Command(...).Output()`. If git returns an empty output with a nil error (atypical but possible with some git configurations or empty reflog states), the orchestrator panics and terminates without cleanup. **Remediation**: Guard with `if len(b) == 0 { return b }` at the top of `bytesTrim`, or replace the function with `bytes.TrimRight(out, "\n")`. Validate with `go test ./memory/...`.

- [x] **F-05: `isIdentChar` omits uppercase letters — false-positive symbol matches** — `main_context.go:112` — Logic — `isIdentChar` checks only lowercase letters (`'a'`–`'z'`), digits, and underscore. It is used by `containsWord` to determine word boundaries: if `isIdentChar(text[end]) == false`, the match is accepted as a whole-word match. Because uppercase `'A'`–`'Z'` are not recognised as identifier chars, `containsWord("BuildFuncDAG", "Build")` returns `true` even though "Build" is followed by "F" (an identifier char). This causes `matchSymbol` to incorrectly select functions whose names are prefixes of longer exported names (e.g., matching `Build` when `BuildFuncDAG` is the correct target), sending the LLM wrong function-scoped context. **Remediation**: Add `|| (c >= 'A' && c <= 'Z')` to `isIdentChar` at `main_context.go:112`. Validate with `go test -run TestContainsWord ./... ` after adding a test for this case.

- [x] **F-06: O(n²) JSON extraction in `extractJSON`** — `main_json.go:33-39` — Performance — The loop `for i := len(content); i > 0; i--` tries `n` candidate substrings, each passed to `json.Unmarshal` which is O(n). Total cost is O(n²) in response length. For a 50 KB LLM response, this performs ~2.5 billion character comparisons. On every task generation and split call, this will stall the orchestrator for seconds. The correct approach is to scan forward for matching brackets, not backward from the full length. **Remediation**: Replace the backward scan with a forward bracket-matching scan: find the outermost `[` or `{`, walk forward tracking nesting depth, extract the balanced substring, then unmarshal once. Validate with `go test -run TestExtractJSON ./...` with large inputs.

---

### MEDIUM

- [x] **F-07: `callLLMWithModel` silently discards LLM JSON unmarshal error** — `main.go:185-187` — Error Handling — `json.Unmarshal(out, &parsed)` drops its error return. If the LLM returns malformed JSON (e.g. a partial response due to timeout), `parsed` remains zeroed. `len(parsed.Choices) == 0` causes `logFatal`, which calls `os.Exit(1)`. While the fatal path is reached, the error message says "no choices in LLM response" without surfacing the actual parse error, making debugging difficult. Also, token-usage logging emits zeros for malformed responses. **Remediation**: Capture and log the unmarshal error: `if err := json.Unmarshal(out, &parsed); err != nil { logError("llm_parse_failed", "", err.Error()) }`. Validate with `go vet ./...`.

- [x] **F-08: `ensureTasksFile` silently uses empty content when a planning doc cannot be read** — `main.go:58-59` — Error Handling — `data, _ := os.ReadFile(doc.Name)` ignores the read error. The file's existence was already checked with `os.Stat`, but a subsequent permission error or I/O error causes `data` to be nil. `generateTasksFromDoc(doc.Name, string(nil))` then calls the LLM with an empty content string, generating meaningless tasks and wasting inference budget. **Remediation**: Check and log the error: `data, err := os.ReadFile(doc.Name); if err != nil { logError("doc_read_failed", "", err.Error()); continue }`. Validate with `go test -race ./...`.

- [x] **F-09: `LoadMetrics` silently ignores `json.Unmarshal` error** — `memory/metrics.go:19` — Error Handling — `json.Unmarshal(data, &m)` return value is discarded. If `adaptive_metrics.json` is truncated (e.g., by a prior crash mid-write), the function returns a zeroed `AdaptiveMetrics` and `nil` error. The orchestrator treats this as "no history" and resets its adaptive behaviour, losing all accumulated learning. **Remediation**: Return the unmarshal error: `if err := json.Unmarshal(data, &m); err != nil { return m, fmt.Errorf("metrics decode: %w", err) }`. Validate with `go test ./memory/...`.

- [x] **F-10: `loadTasks` silently ignores `json.Unmarshal` error** — `main.go:195-197` — Error Handling — `json.Unmarshal(data, &tf)` error is dropped. A corrupt `tasks.json` (e.g., from a killed mid-write) yields a zeroed `TaskFile{}`, causing the orchestrator to believe there are no tasks and call `ensureTasksFile()` again, potentially regenerating all tasks and losing progress. **Remediation**: Return or log the error and halt: `if err := json.Unmarshal(data, &tf); err != nil { logFatal("tasks_corrupt", err.Error()) }`. Validate with `go test -race ./...`.

- [x] **F-11: `gitCommit` silently ignores `git add` and `git commit` errors** — `main.go:208-210` — Error Handling — Both `exec.Command("git", "add", ".").Run()` and `exec.Command("git", "commit", ...).Run()` errors are ignored. If `git commit` fails (e.g., nothing to commit, or git identity not configured), `completeTask` returns normally, the task is marked complete, and the patch is never committed. On the next run, the applied-but-uncommitted changes are invisible to the orchestrator, which will re-run the same task and produce conflicts. **Remediation**: Check errors and call `logError`; consider marking the task blocked if commit fails. Validate with `go vet ./...`.

- [x] **F-12: `log()` silently swallows file-open and file-write errors** — `main_helper.go:151-153` — Error Handling — `f, _ := os.OpenFile(logFile, ...)` and `f.Write(...)` errors are ignored. If the log file cannot be opened (disk full, permissions), all structured telemetry is lost silently. Since `orchestrator.log` is the primary observability channel for debugging, losing log entries is operationally severe. **Remediation**: At minimum, print to stderr when log file open fails: `if f == nil { fmt.Fprintf(os.Stderr, "log write failed: %v\n", err) }`. Validate with `go test ./...`.

- [x] **F-13: `SaveRun` ignores `git add` and `git commit` errors; memory persistence silently fails** — `memory/runlog.go:34-35` — Error Handling — `exec.Command("git", "add", ".").Run()` and `exec.Command("git", "commit", ...).Run()` are called without error checks. If the `memories` branch commit fails, the run summary is written to disk but never committed; the next run's branch switch discards it. The adaptive planner silently operates on stale data. **Remediation**: Check errors and propagate them as return values from `SaveRun`. Validate with `go test ./memory/...`.

- [x] **F-14: `mergeClusteredTasks` uses stale slice indices after first element removal** — `main_subsystem.go:130-153` — Logic — `detectClusteredTasks` computes indices into `tf.Tasks` before any merges. In the iteration loop, the first merge (`tf.Tasks = append(tf.Tasks[:j], tf.Tasks[j+1:]...)`) shifts all elements after index `j` down by one. Subsequent clusters computed from the pre-merge index set may reference wrong tasks (indices are off by the number of prior removals). While the bounds check `j >= len(tf.Tasks)` prevents panics, it cannot prevent merging the wrong task pair. **Remediation**: Rebuild the cluster map after each merge, or collect all merges in a single pass using a stable index strategy (e.g., process clusters in reverse index order). Validate with `go test -run TestMergeClusteredTasks ./...`.

- [x] **F-15: `validatePatchSize` counts all diff lines including headers, not just changed lines** — `main_validatepatch.go:62` — Logic — `lineCount(diff)` counts `len(strings.Split(diff, "\n"))`, which includes `diff --git`, `index`, `---`, `+++`, and `@@` header lines. A patch modifying 10 lines across 3 files has ~15 extra header lines, consuming ~25 of the 50-line budget. Valid micro-edits are rejected or squeezed into artificially small diffs, contradicting the project's stated goal of "micro-edits." **Remediation**: Count only `+` and `-` lines (excluding `+++`/`---` prefix lines) in `validatePatchSize`, consistent with how `deletionRatio` counts them. Validate with `go test -run TestValidatePatchSize ./...`.

- [x] **F-16: Non-deterministic cluster ordering in `ClusterPackages`** — `audit/cluster.go:8` — Logic/API — Iterating `graph.Packages` (a `map`) produces non-deterministic package visit order. Two runs on the same codebase may produce different cluster assignments and different Finding outputs. This makes audit results non-reproducible and breaks diff-based tracking of audit findings over time. **Remediation**: Sort map keys before iterating: extract keys into a slice, call `sort.Strings(keys)`, then iterate. Validate with `go test ./audit/... -count=3` checking that outputs are identical across runs.

- [x] **F-17: Dual overlapping prompt truncation — `compressPrompt` (6000 chars) and `enforceTokenBudget` (1500 words) interact unpredictably** — `main_memory.go:56-57`, `main_token_budget.go:6` — Logic — Every LLM call passes through both limits. For dense code content (average 4-5 chars/word), 1500 words ≈ 6000-7500 chars, so either limit may bind first depending on content. For whitespace-heavy diffs, the token budget triggers first; for minified content, the char budget triggers first. The effective context window seen by the LLM is unpredictably variable, causing inconsistent planning quality. **Remediation**: Unify into a single truncation strategy. Pick one mechanism (preferably the char budget, which is simpler to reason about) and remove the other, or document their interaction precisely. Validate with `go test -run TestEnforceTokenBudget ./...`.

- [x] **F-18: `perFileLineDeltaCap` may block valid single-file patches** — `main_validatepatch.go:185-187` — Logic — `perFileLineDeltaCap` = `max(1, allowedPatchLines(task)/2)`. For a single-file patch with a 50-line budget, the per-file cap is 25 lines. Since `validateLineDeltaCaps` counts both `+` and `-` lines per file, a minimal change that adds 15 lines and removes 12 lines (27 total deltas) is rejected even though the overall diff is well within budget. This contradicts the self-stated design of "micro-edits." **Remediation**: Either remove `validateLineDeltaCaps` (the overall `validatePatchSize` already enforces total budget) or raise the per-file cap to the full `allowedPatchLines(task)` for single-file patches. Validate with the existing `main_validatepatch_test.go`.

---

### LOW

- [x] **F-19: `os.MkdirAll` result ignored in `SaveRun`** — `memory/runlog.go:21` — Error Handling — `os.MkdirAll(RunsDir, 0755)` return value is silently discarded. If directory creation fails (disk full, permissions), the subsequent `os.WriteFile` fails with a confusing "no such file or directory" error that is then returned from `SaveRun`. **Remediation**: Check and wrap the error: `if err := os.MkdirAll(RunsDir, 0755); err != nil { return fmt.Errorf("runs dir: %w", err) }`. Validate with `go vet ./...`.

- [x] **F-20: `compilerErrorLines` caps at 5 lines, hiding additional context from LLM** — `main_util.go:38` — Logic — The function returns at most 5 compiler error lines. For tasks with many type errors or widespread import issues, the LLM receives a truncated error view and may generate fix diffs that address only the visible errors, causing repeated retry cycles. **Remediation**: Increase the cap to 10-15 lines, or make it configurable. Validate with `go test -run TestBuildFixHints ./...`.

- [x] **F-21: `subsystemRegistry` package-level global without synchronisation** — `main_subsystem.go:59` — Concurrency — `subsystemRegistry` is a `map[string]*subsystemMetrics` written by `recordSubsystemOutcome` and `recordSubsystemPatchMetrics`, and read by `subsystemBudgetMultiplier`. Currently these are all called sequentially from the execution loop, so there is no race. However, `strategyCompete` and `speculativeExecute` launch goroutines that call `evaluateStrategyDiff` → `scorePatchRisk`, which is pure and safe. The risk is latent: any future goroutine that calls `recordSubsystemOutcome` would introduce a data race. **Remediation**: Protect `subsystemRegistry` with a `sync.RWMutex` or pass it as an explicit parameter to functions that use it. Validate with `go test -race ./...`.

- [x] **F-22: `copyFilesToDir` in simulation does not sanitise relative file paths** — `main_simulation.go:45` — Security — `dst := filepath.Join(destDir, src)` where `src` is derived from the diff's `+++ b/` header. If `src` contains `../../` components, `filepath.Join` resolves them and `dst` may point outside `destDir`. In the simulation context the damage is limited to the temp dir (which is removed by `defer os.RemoveAll(tmpDir)`), but the `os.MkdirAll` and `os.WriteFile` calls will create directories and files outside the temp dir. **Remediation**: Add `if !strings.HasPrefix(filepath.Clean(dst), destDir) { continue }` before writing. Validate with `go test ./...`.

- [x] **F-23: `allGoFiles()` runs `git ls-files` on every `resolveExpandedFiles` call** — `main_reviewmode.go:80` — Performance — Each invocation of `resolveExpandedFiles` (called per task in strategic review mode) shells out to `git ls-files`, which re-reads the index from disk. In a repository with thousands of files, this adds measurable latency per task iteration. **Remediation**: Cache the result of `allGoFiles()` in a package-level variable, populated once at `runExecutionMode` startup. Validate performance with timing logs.

- [x] **F-24: `splitOversizedDescription` shares `task.Files` slice header across subtasks** — `main_tasks.go:141` — Data Aliasing — Each subtask created by `splitOversizedDescription` is assigned `Files: task.Files` (shallow copy of slice header). All subtasks share the same underlying array. If any subtask's `Files` slice is later `append`-ed to (e.g., in `mergeInto`), and `len < cap`, the append writes into shared memory, silently corrupting sibling subtasks' file lists. **Remediation**: Copy the slice: `Files: append([]string(nil), task.Files...)`. Validate with `go test ./...`.

- [x] **F-25: `toLower` in `main_reviewmode.go` handles only ASCII uppercase** — `main_reviewmode.go:121-131` — Logic — The package-local `toLower` converts only bytes in the range `'A'`–`'Z'`. Task descriptions or keyword strings containing non-ASCII uppercase letters (e.g. accented characters from European file names) are not lowercased, causing `containsAPIKeyword` to miss keyword matches. Since `strings.ToLower` is already imported by other files in the same package, importing it here has zero cost. **Remediation**: Replace `toLower` and `contains` with `strings.ToLower` and `strings.Contains`. Validate with `go vet ./...`.

- [x] **F-26: `logFatal` during task generation is unrecoverable — no retry path** — `main.go:150-159` — Error Handling/API — `generateTasksFromDoc` calls `logFatal` (which calls `os.Exit(1)`) if the LLM returns invalid JSON for the planning phase. There is no retry, no partial task list fallback, and no graceful shutdown. A single malformed LLM response during startup terminates the orchestrator with no recovery. **Remediation**: Return an error from `generateTasksFromDoc` and let `ensureTasksFile` retry the LLM call up to `maxRetries` times before exiting. Validate with `go test -race ./...`.

- [x] **F-27: `log()` opens a new file descriptor on every log call** — `main_helper.go:151-154` — Performance — Each call to `logInfo`, `logError`, or `logFatal` opens `orchestrator.log`, writes one entry, and closes the file. In a busy execution loop generating 5-10 log entries per task, this is 10-20 `open`+`close` syscall pairs per task. For a 100-task run, this is ~2000 syscalls solely for logging. **Remediation**: Open the log file once at startup (store as a package-level `*os.File`), write entries directly, and close at exit via `defer`. Validate with `go test -race ./...`.

---

## Metrics Snapshot

| Metric | Value |
|--------|-------|
| Total functions | 268 |
| Functions above complexity 15 | 3 |
| Avg cyclomatic complexity | 4.5 |
| Doc coverage | 49.1% |
| Duplication ratio | 0.24% |
| Test pass rate | 3/3 packages (all pass with -race) |
| go vet warnings | 0 |
| Dead code (unreferenced) | 4 functions (reported by go-stats-generator) |

---

## False Positives Considered and Rejected

| Candidate | Reason Rejected |
|-----------|----------------|
| `strategyCompete` goroutines writing to shared global state | Goroutines call only pure functions (`callLLMWithModel`, `evaluateStrategyDiff`); global mutations (`activeTier`, `subsystemRegistry`) happen before goroutines start and after they join. No actual race in current code. |
| `revertPatch` missing on invariant violation path | `applyDiffToWorkspace` already calls `revertPatch(diff)` in `checkPostPatchInvariants` before returning an error. The revert IS present for the invariant path; the bug (F-02) is only on the build-failure path. |
| `callLLMWithModel` HTTP response body leak | `resp.Body.Close()` is deferred after `err != nil` check; when `err != nil`, `logFatal` calls `os.Exit(1)` before any leak. When `err == nil`, `defer resp.Body.Close()` correctly runs. No leak. |
| `extractJSON` logical incorrectness | The backward scan finds the LARGEST valid prefix, which is logically correct (it maximises valid JSON recovered). The O(n²) complexity is real but not a logic bug. Reported as HIGH for performance only. |
| `validateLineDeltaCaps` triggering on test input | The existing `main_validatepatch_test.go` exercises these paths; the test passes. The finding (F-18) is about design intent vs. implementation policy, not a crash or data corruption. |
| Loop variable capture in `strategyCompete` | `go func(s executionStrategy)` passes `s` by value as a parameter. No capture. Not a bug. |
| `compressPrompt` rune-vs-byte boundary cut | `string(runes[:promptCharBudget])` correctly converts back to string without incomplete multibyte sequences. Not a bug. |
| `isIdentChar` called in `containsWord` for right-boundary check with `'A'`–`'Z'` | This IS a real bug (F-05). Not rejected. |

## Remaining Scope

All packages and files have been fully audited. No remaining scope.
