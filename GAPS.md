# Implementation Gaps — 2026-07-02

## Gap 1: Safe Crash-Recovery Commit Path Is Not Enforced
- **Stated Goal**: GOALS.md Phase 6 (`GOALS.md:144-160`) promises safe checkpoint recovery after interruption.
- **Current State**: `recoverExecutionJournal` commits current workspace for `built` journal state without validating stored patch hash/diff (`main_journal.go:119-134` + `main.go:471-476`).
- **Impact**: Interrupted runs can commit unrelated local changes as recovered task output.
- **Closing the Gap**: Verify workspace hash against journal patch metadata before commit and restrict commit scope to known touched files.

## Gap 2: Dependency-Ordered Execution Breaks After Task Replacement
- **Stated Goal**: README promises DAG-based execution with dependency ordering and automatic splitting (`README.md:14-16`).
- **Current State**: Task replacement during split removes original IDs but does not rewrite downstream `depends_on` edges (`main_tasks.go:64-77`, `main.go:242-257`).
- **Impact**: Dependent tasks can remain permanently pending due to dangling dependencies.
- **Closing the Gap**: Rewrite dependent edges when replacing a task ID, mapping old dependency to replacement boundary task(s).

## Gap 3: Branch Isolation Guarantee Is Not Reliable on Failure
- **Stated Goal**: README claims each run creates isolated `autonomous/<timestamp>` branch (`README.md:19,63`).
- **Current State**: `ensureBranch` ignores checkout errors and still logs success (`main_helper.go:125-128`).
- **Impact**: Runs may continue on an unintended branch while reporting isolation.
- **Closing the Gap**: Fail fast on branch creation errors or retry with unique suffix and verify active branch before execution.

## Gap 4: Adaptive Memory Persistence Is Coupled to Main Worktree State
- **Stated Goal**: README claims adaptive memory is persisted and reused across runs (`README.md:20,166-173`).
- **Current State**: Memory writes switch branches in-place and commit with `git add .` (`memory/runlog.go:18-45`, `memory/metrics.go:55-77`).
- **Impact**: Dirty worktrees can block or contaminate memory persistence, reducing reliability of cross-run adaptation.
- **Closing the Gap**: Persist memory in isolated worktree/ref operations and stage only memory files.

## Gap 5: GOALS Priority/Aging Semantics Are Partially Implemented
- **Stated Goal**: GOALS Phase 3 requires `critical/high/normal/low` priority and retry-aging behavior (`GOALS.md:88-101`).
- **Current State**: `taskPriority` only distinguishes audit-high/critical vs all others (`main.go:197-204`); no full four-level priority model or explicit aging-based deprioritization exists.
- **Impact**: Scheduler behavior does not match documented prioritization expectations.
- **Closing the Gap**: Add explicit priority field parsing, deterministic comparator for four priority levels, and retry-aging penalties in task selection.
