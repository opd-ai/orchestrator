# Implementation Gaps — 2026-07-02

## G-01: File-Allowlist Guard Missing When Task Has No Files

- **Stated Goal**: GOALS.md Phase 1.2: "Validate patch: only touch allowed files." README.md §Safety: "patches are validated against a per-task file allowlist before being applied."
- **Current State**: `validateTouchedFiles` in `main_validatepatch.go:72-74` returns `nil` (no validation) when `len(task.Files) == 0`. Most tasks generated from planning documents (GOALS.md, ROADMAP.md) carry an empty `Files` slice because the file list is only populated by the symbol-task splitter for single-function tasks. The allowlist is never consulted for high-level planning tasks.
- **Impact**: Any LLM-generated diff can touch any file in the filesystem without restriction. An adversarial or corrupt model response could overwrite git configuration, CI scripts, SSH authorized_keys, or system cron files that are reachable from the working directory. This completely defeats the stated patch-safety guarantee for the majority of task types.
- **Closing the Gap**: When `task.Files` is empty, fall back to validating that all touched paths are inside the repository root (canonical path containment check using `filepath.Clean` and `strings.HasPrefix`). Optionally, for tasks that operate on specific documentation files, populate `task.Files` during task generation.

---

## G-02: No Dirty-State Detection Before Patch Application

- **Stated Goal**: GOALS.md Phase 1.2: "Detect dirty git state before applying patch." README.md implies each task starts from a clean baseline.
- **Current State**: `execute` in `main_exec.go` calls `getDiffForTask` and immediately calls `applyDiffToWorkspace` without first running `git status` or `git diff --stat` to verify the working tree is clean. If a previous task left uncommitted modifications (due to the missing revert described in G-04), or if the user has local edits, the patch is applied on top of an unknown delta, producing a potentially incorrect merged state.
- **Impact**: Tasks accumulate on top of each other's partial state. Subsequent `build()` failures are incorrectly attributed to the current task's patch, causing spurious retries and task splits. The orchestrator can never be certain it is patching against the baseline it analysed.
- **Closing the Gap**: At the start of `execute` (before `getDiffForTask`), run `git diff --quiet && git diff --cached --quiet` and return an error if the working tree is dirty. Optionally, offer a `--force-clean` flag that runs `git checkout -- .` and `git clean -fd` to reset state.

---

## G-03: No Crash Recovery or Journal File

- **Stated Goal**: GOALS.md Phase 6: "Crash Recovery: if the orchestrator is killed mid-task, it should be able to resume from the last checkpoint. A journal file records each completed step so recovery can skip already-done work."
- **Current State**: No journal file, checkpoint file, or step-level recovery mechanism exists anywhere in the codebase. `tasks.json` records task completion status, but it does not record sub-step completion (pre-patch, patched, built, committed). If the orchestrator is killed after `applyDiffToWorkspace` but before `gitCommit`, the working tree contains an applied patch, tasks.json does not mark the task complete, and the next run re-applies the patch, producing a double-application or a `git apply` failure.
- **Impact**: Any interruption (OOM kill, power loss, `Ctrl+C`) leaves the repository in an unknown state. The next run has a high probability of either duplicating work or crashing on `git apply` conflict. Users must manually inspect and clean up the repository.
- **Closing the Gap**: Introduce a lightweight journal file (e.g., `orchestrator-journal.json`) that records the current task ID and the last completed step (one of: `planned`, `patched`, `built`, `committed`). On startup, read the journal; if the last step was `patched` but not `committed`, either revert the patch (if `built` not reached) or commit it (if `built` was reached). This satisfies GOALS.md §6 without requiring a full transaction log.

---

## G-04: Working Directory Not Reset Before Each Task

- **Stated Goal**: GOALS.md Phase 1.2: "Reset working directory before each task."
- **Current State**: There is no `git checkout -- .`, `git clean -fd`, or `revertPatch` call at the top of `execute` in `main_exec.go`. If a prior task's fix loop exhausted retries, the working tree contains the failed patch. The next task's diff is generated against the corrupted state and, if applied, compounds the corruption. Even in the happy path, the orchestrator relies on each prior task completing cleanly with a commit, with no verification.
- **Impact**: Cascading failures. After the first failed task with exhausted retries, all subsequent tasks operate against incorrect baselines. `build()` failures in later tasks may be caused entirely by earlier unreverted patches, not the current task's diff.
- **Closing the Gap**: At the top of `execute`, before `getDiffForTask`, run `git checkout -- .` and `git clean -fd -x --exclude=tasks.json --exclude=orchestrator.log` to guarantee a clean baseline. Gate this on `!dryRun`.

---

## G-05: Go Version in `go.mod` Is Nonexistent

- **Stated Goal**: The module should declare a valid, supported Go toolchain version.
- **Current State**: `go.mod` declares `go 1.26.1`. As of the audit date (2026-07-02), the latest stable Go release is Go 1.24.x. Go 1.26.1 does not exist. This means:
  - `go mod tidy` with any installed Go toolchain will silently accept the directive (Go treats forward-declared versions as acceptable).
  - CI or contributor systems that enforce `go.mod` version checks will fail.
  - The `//go:build` constraint `go1.26.1` (if any were added) would be silently never satisfied.
- **Impact**: Low for current users but creates confusion for contributors and CI systems. If a `toolchain go1.26.1` directive is added, all builds would break.
- **Closing the Gap**: Update `go.mod` to declare `go 1.22` (or the actual minimum version the codebase requires), aligning with the loop-variable closure semantics the project assumes (`main_speculative.go` passes loop vars by copy, suggesting Go 1.22+ awareness).

---

## G-06: Token Budget Uses Word-Count Approximation, Not Token Count

- **Stated Goal**: README.md §Token Budget: "Enforces a token budget before each LLM call to avoid context-window overflow."
- **Current State**: `enforceTokenBudget` in `main_token_budget.go:6` splits on whitespace and counts words, then multiplies by 1.5 (an approximation). For Go source code (which has many single-character tokens, identifiers, and punctuation), the actual BPE token count is typically 1.3–2.5× the word count depending on content. A prompt that passes the word-count budget may be 30–70% over the actual token limit, causing the LLM API to return a `context_length_exceeded` error. Separately, `compressPrompt` truncates at 6000 characters without any word/token awareness.
- **Impact**: Users on models with tight context windows (7B models default to 4096 tokens) will experience silent context truncation or API errors on large tasks. The budget enforcement gives a false sense of safety.
- **Closing the Gap**: Either integrate `tiktoken-go` (or an equivalent tokeniser for the target model family) for accurate token counting, or document the approximation prominently in README with guidance on when it will fail.

---

## G-07: Context File Matching Uses Only First Word of Task Description

- **Stated Goal**: README.md §Context: "Automatically includes relevant source files as context for each task."
- **Current State**: `keyword()` in `main_context.go` calls `strings.Fields(task.Description)[0]` — it extracts only the first word of the task description and uses that as the sole search term for matching context files. A task described as "Refactor authentication handler to use JWT tokens" yields the keyword "Refactor", which matches nothing useful. The LLM receives no relevant source file context, degrading patch quality for any task whose first word is a common verb.
- **Impact**: The automatic context injection feature is effectively non-functional for all tasks whose descriptions begin with a verb (the overwhelming majority of engineering task descriptions).
- **Closing the Gap**: Extract all non-stopword nouns and identifiers from the description. Use each as a keyword and union the results. At minimum, take the last word (which is usually the subject) or iterate all words and score matches.

---

## G-08: `memory.UpdateMetrics` Performs Branch Checkout, Corrupting Uncommitted Work

- **Stated Goal**: README.md §Memory: "Persists cross-run metrics for adaptive planner tuning."
- **Current State**: `UpdateMetrics` in `memory/metrics.go` calls `checkoutBranch("memories")`, writes files, then calls `checkoutBranch(originalBranch)`. This performs two live `git checkout` operations in the middle of the execution loop. If the working tree has uncommitted modifications (the just-applied patch before commit), `git checkout` will either refuse (if files conflict) or silently clobber the pending patch. The lock-step between apply-patch → update-metrics → commit is fragile and order-dependent.
- **Impact**: In the best case, `git checkout memories` fails (because of pending changes), `UpdateMetrics` errors, and the calling code discards the error (see F-09). In the worst case, git performs the checkout and the applied patch is lost before `gitCommit` is reached.
- **Closing the Gap**: Use `git worktree add` to write to the `memories` branch in a separate working tree, or use `git update-ref` + `git hash-object` + `git commit-tree` to write directly to the ref without changing HEAD. This eliminates the branch-switch entirely.

---

## G-09: Audit Mode Output Not Fed Back Into Execution Loop

- **Stated Goal**: README.md §Audit Mode: "Runs architecture and API audits, then uses findings to guide task prioritisation."
- **Current State**: `runAuditMode` in `main_audit.go` calls the audit passes and writes findings to `audit_findings.json`. The execution loop (`runExecutionMode`) reads `tasks.json` but never reads `audit_findings.json`. Audit findings have no effect on task ordering, task descriptions, or the LLM prompt for any subsequent execution run.
- **Impact**: The stated goal of audit-guided task prioritisation is completely unimplemented. Users invoking `--audit` followed by a normal run get no benefit from the audit beyond the JSON file.
- **Closing the Gap**: In `ensureTasksFile` (or a new `mergeAuditFindings` step), read `audit_findings.json` if it exists and inject the highest-severity findings as additional task descriptions, or re-order existing tasks to prioritise files with HIGH/CRITICAL findings.

---

## G-10: `--dry-run` Flag Still Invokes the Compiler

- **Stated Goal**: README.md: "`--dry-run` mode generates and validates patches without applying them to the repository."
- **Current State**: In `execute` (`main_exec.go`), the `dryRun` flag prevents `applyDiffToWorkspace` and `gitCommit` from running. However, `build()` is called unconditionally after the patch validation stage in the fix-loop path. `build()` compiles the current working directory using `go build ./...`. In dry-run mode, no patch has been applied, so `build()` is testing the unmodified working tree — any pre-existing build errors will cause the dry-run to report the task as failed, even though the patch may be valid.
- **Impact**: Dry-run is not a safe preview mode. Pre-existing compilation errors in the repository cause dry-run to report every task as a build failure, making it unusable for validation on partially-broken codebases.
- **Closing the Gap**: Skip `build()` entirely when `dryRun == true`, or gate the `build()` call on `!dryRun`. The dry-run should report: patch generated, patch valid, patch would apply cleanly — without invoking the compiler.
