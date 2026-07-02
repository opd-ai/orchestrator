# UNIVERSAL BUG AUDIT (END-TO-END) — 2026-07-02

## Project Profile
Orchestrator is a local-LLM autonomous engineering loop for Go repositories. It promises document-driven task generation, DAG-ordered execution, patch validation, automatic task splitting, build-and-fix retries, branch isolation, structured logs, adaptive memory, and static audit mode (README.md:3-23,61-65,110-125).

Primary trust boundaries:
- Untrusted model output enters as JSON tasks and unified diffs (`generateTasksFromDoc`, `executeTask`, `fixTask`).
- Untrusted patch content is applied to the repository and then committed (`applyPatch`, `gitCommit`).
- Git command outputs and filesystem state drive control flow (`ensureCleanWorkspace`, journal recovery, memory branch persistence).

Critical paths: `runExecutionMode` → `execute` → `gatherAndValidateDiff` → `applyDiffToWorkspace` → `build`/retry loop → `gitCommit`, plus startup recovery (`recoverExecutionJournal`) and memory persistence (`memory.SaveRun`, `memory.UpdateMetrics`).

## Audit Scope
- Packages audited: `github.com/opd-ai/orchestrator`, `github.com/opd-ai/orchestrator/audit`, `github.com/opd-ai/orchestrator/memory`
- Files audited: all Go files in repository (68) plus root docs/claims (`README.md`, `GOALS.md`, `ROADMAP.md`, `architecture/invariants.json`, `go.mod`)
- Functions inspected (go-stats full scan): 453
- High-risk functions manually inspected (length >50 or cyclomatic >15): 10
- Structural metrics source: `tmp/generic-audit-baseline*.json`
- Baseline validation:
  - `go test -race ./...` ✅ (3/3 packages)
  - `go vet ./...` ✅ (0 warnings)

## Coverage Log
| Package | 3b Logic | 3c Nil | 3d Errors | 3e Resources | 3f Concurrency | 3g Security | 3h Aliasing | 3i Init | 3j API |
|---------|----------|--------|-----------|--------------|----------------|-------------|-------------|---------|--------|
| main | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| audit | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| memory | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |

## Goal-Achievement Summary
| Stated Goal | Status | Blocking Findings |
|-------------|--------|-------------------|
| DAG-based execution respects dependencies (README.md:14) | ⚠️ | F-02 |
| Automatic task splitting and re-queueing (README.md:16) | ⚠️ | F-02 |
| Crash recovery resumes safely from checkpoint (GOALS.md:144-160) | ❌ | F-01 |
| Git branch isolation per run (README.md:19,63) | ⚠️ | F-05 |
| Adaptive memory persisted across runs (README.md:20,166-173) | ⚠️ | F-03 |

## Findings

### CRITICAL
- [ ] **F-01: Journal “built” recovery can commit unrelated workspace state** — `main_journal.go:119-134`, `main.go:471-476` — **error handling / data integrity** — `recoverExecutionJournal` commits all content through `gitCommit(task)` when `step=built`, but does not verify that current workspace still matches `entry.PatchHash`/`entry.PatchDiff`. **Code path:** startup `runExecutionMode` → `recoverExecutionJournal` → `gitCommit` (`git add .`). If workspace changed between crash and restart, unrelated changes are committed as recovered task output. **Remediation:** in `recoverExecutionJournal`, verify hash/diff before commit and commit only expected touched files (not `git add .`). **Validation:** `go test -race ./...` with recovery mismatch tests.

### HIGH
- [ ] **F-02: Splitting/replacement breaks downstream dependency graph** — `main_tasks.go:64-77`, `main.go:242-257` — **logic / state machine** — `replaceTask` removes original task ID and inserts subtasks but does not rewrite other tasks’ `DependsOn` references from old ID to replacement IDs. **Code path:** `enforceTaskGranularity`/`splitTask` → `replaceTask`; later `depsSatisfied` calls `isComplete` for removed ID, which returns false forever, leaving dependents permanently pending. **Remediation:** when replacing a task, rewrite every task depending on old ID to depend on replacement boundary task(s). **Validation:** `go test -race ./...` with regression where `B depends_on A` and `A` is split.
- [ ] **F-03: Memory persistence mutates branches via live checkout and workspace-wide commit** — `memory/runlog.go:18-45`, `memory/metrics.go:55-77`, `memory/branch.go:18-32` — **resource/state safety** — memory persistence switches branches (`git checkout memories`) and runs `git add .`/`git commit` from the active worktree. On dirty trees this can fail or capture unintended files into memory commits. **Code path:** `runExecutionMode` end → `memory.SaveRun`/`memory.UpdateMetrics` → `ensureMemoryBranch` + global add/commit. **Remediation:** persist memory via separate worktree or plumbing writes without changing HEAD; scope adds to memory artifacts only. **Validation:** `go test -race ./...` plus dirty-workspace integration test for memory writes.

### MEDIUM
- [ ] **F-04: Subsystem clustering is disabled by incorrect pending-task filter** — `main_subsystem.go:106-110` — **logic bug** — `buildSubsystemMap` skips tasks when `t.Status != ""`; normal pending tasks use `"pending"`, so almost all eligible tasks are excluded and clustering never triggers. **Consequence:** merge optimization path is effectively dead code. **Remediation:** filter on `t.Status != "pending"` instead of non-empty status. **Validation:** `go test -race ./...` with subsystem merge regression test.
- [ ] **F-05: Branch isolation can silently fail while reporting success** — `main_helper.go:125-128` — **error handling** — `ensureBranch` ignores `git checkout -b` errors and always logs `branch_created`. If branch creation fails (existing name, detached HEAD issue), execution proceeds on unintended branch, violating branch isolation guarantee. **Remediation:** check `exec.Command(...).Run()` error and fail fast (or retry with unique suffix) before continuing. **Validation:** `go test -race ./...` plus test for checkout failure path.

### LOW
- [ ] **F-06: Task-state persistence silently ignores write failures** — `main.go:466-469` — **error handling** — `saveTasks` discards JSON marshal and file write errors. On disk or permission failures, in-memory status diverges from persisted `tasks.json`, causing replay/skipped work on next run. **Remediation:** return and handle errors from `json.MarshalIndent` and `os.WriteFile`. **Validation:** `go test -race ./...` with injected write-failure unit test.

## Metrics Snapshot
| Metric | Value |
|--------|-------|
| Total functions | 453 |
| Functions above complexity 15 | 0 |
| Avg cyclomatic complexity | 3.26 |
| Doc coverage | 75.0% |
| Duplication ratio | 0.09% |
| Test pass rate | 3/3 |
| go vet warnings | 0 |

## False Positives Considered and Rejected
| Candidate | Reason Rejected |
|-----------|----------------|
| Loop-variable capture in `main_speculative.go` goroutines | Rejected: loop vars are explicitly shadow-copied (`i, temp := i, temp` at `main_speculative.go:30`) before goroutine launch. |
| Missing workspace reset before task execution | Rejected: `setupTaskEnv` always calls `ensureCleanWorkspace` (`main_exec.go:181`) unless explicitly disabled via `--skip-workspace-reset`. |
| Dry-run still triggering build failures | Rejected: `runBuildStep` short-circuits in dry-run (`main_exec.go:235-240`) and logs `dry_run_build_skipped`. |

## Remaining Scope (if session ended before completion)
| Package | Status | Notes |
|---------|--------|-------|
| (none) | Complete | Full pass finished; no remaining package scope. |
