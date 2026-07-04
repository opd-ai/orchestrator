# Implementation Roadmap

This roadmap replaces the prior assessment-style document with a forward-looking implementation plan.
It is ordered by execution risk: close correctness and recovery gaps first, then improve scheduling, observability, and self-evolution.
All file references below are relative to the repository root.

## Delivery Rules

- Keep every change backward compatible.
- Deliver each roadmap item in small, reviewable patches.
- Put new behavior behind flags when it changes operator-visible execution.
- After each milestone, confirm `go vet ./...` and `go test -race ./...` both pass.

---

## Milestone 1 — Make recovery and branch isolation trustworthy

**Objective:** interrupted runs must never commit unrelated work, and every run must execute on the branch it claims to use.

**Primary files:** `main_journal.go`, `main.go`, `main_exec.go`, `main_helper.go`

**Implementation steps**

1. Extend the execution journal to persist the task ID, patch digest, touched files, and last durable step.
2. Before recovering a `built` state, verify that the current workspace still matches the recorded patch metadata.
3. Restrict recovery commits to the recorded touched files instead of committing the whole worktree.
4. Abort recovery when journal metadata and workspace state diverge, and emit a structured log explaining the mismatch.
5. Fail fast if branch creation or checkout fails in `ensureBranch`, and verify the active branch before execution begins.
6. Add restart-path tests that cover interrupted `patched` and `built` journal states.

**Definition of done**

- Recovery never commits unrelated local changes.
- A branch checkout failure stops the run immediately.
- Restart scenarios are covered by automated tests and structured logs.

---

## Milestone 2 — Preserve task graph correctness under mutation

**Objective:** task splitting, replacement, and prioritization must keep the execution DAG valid and deterministic.

**Primary files:** `main.go`, `main_tasks.go`

**Implementation steps**

1. Rewrite downstream `depends_on` edges whenever a task is replaced by split tasks.
2. Define a deterministic replacement rule so downstream tasks depend on the correct successor task IDs.
3. Expand task priority handling to support `critical`, `high`, `normal`, and `low`.
4. Add retry-aging so repeatedly failing tasks are deprioritized without becoming permanently unreachable.
5. Emit the resolved priority and dependency state in task-selection logs.
6. Add tests that cover split-task replacement, dependency rewrites, and priority ordering.

**Definition of done**

- No task remains blocked because it depends on a removed task ID.
- Scheduling honors priority before FIFO while preserving dependency constraints.
- Aging changes execution order deterministically and is test-covered.

---

## Milestone 3 — Decouple adaptive memory writes from the active worktree

**Objective:** memory persistence must survive dirty worktrees and must not mutate the operator’s active branch.

**Primary files:** `memory/metrics.go`, `memory/runlog.go`

**Implementation steps**

1. Replace in-place branch switching for memory writes with isolated ref or worktree operations.
2. Stage and commit only memory artifacts instead of using broad `git add .` behavior.
3. Make memory-write failures non-destructive to the main run and surface them as explicit warnings.
4. Add tests for dirty-worktree persistence and branch restoration guarantees.

**Definition of done**

- Memory updates do not require checking out the `memories` branch in the active worktree.
- Dirty local files do not contaminate memory commits.
- A failed memory write cannot leave the run on the wrong branch.

---

## Milestone 4 — Improve observability and operator recovery tooling

**Objective:** operators should be able to understand failures quickly and resume work without manual forensics.

**Primary files:** `main_observability.go`, `main_exec.go`, `main_json.go`

**Implementation steps**

1. Persist build failures under `logs/build_failures/` with one artifact per task attempt.
2. Persist rejected patches under `logs/rejected_patches/` with the rejection reason in the structured log stream.
3. Generate `AUTONOMOUS_RUN_SUMMARY.md` at the end of each run with task counts, duration, branch, and blocked-task reasons.
4. Emit periodic summary metrics for completed tasks, blocked tasks, retries, and applied patches.
5. Add cleanup and retention rules so observability artifacts do not grow without bound.

**Definition of done**

- Every failed build and rejected patch leaves an inspectable artifact.
- Each run ends with a human-readable summary file.
- Operators can diagnose a failed run from logs and artifacts alone.

---

## Milestone 5 — Tighten planning and context selection

**Objective:** improve first-attempt patch quality by feeding the model better task ordering and smaller, more relevant context.

**Primary files:** `main.go`, `main_context.go`, `main_token_budget.go`

**Implementation steps**

1. Score candidate context files using task keywords, git-tracked file recency, and overlap with files associated with prior successful edits from persisted memory data.
2. Enforce a per-file context cap before concatenating raw file contents.
3. Add a signature-only fallback for oversized files so interfaces survive even when bodies are truncated.
4. Preserve total prompt limits using character-budget enforcement aligned with the existing budget logic.
5. Add tests for file-ranking determinism, per-file truncation, and prompt-budget enforcement.

**Definition of done**

- Large files can no longer consume the entire prompt budget.
- Context assembly remains deterministic for the same task input.
- Prompt trimming preserves the most useful task and interface information first.

---

## Milestone 6 — Harden controlled self-modification

**Objective:** self-evolution must stay safe when the orchestrator edits its own critical execution path.

**Primary files:** `main_helper.go`, `main_exec.go`, `main_tasks.go`

**Implementation steps**

1. Define a protected set of core runtime files that require stronger validation during self-evolution.
2. Require diff preview logging, clean rollback behavior, and passing build/test validation before committing self-edits.
3. Add a two-step apply path for protected files so candidate changes are validated before replacing the active implementation.
4. Record self-edit attempts and outcomes in structured logs and run summaries.
5. Add targeted tests for rollback and validation behavior when protected-file edits fail.

**Definition of done**

- Failed self-edits leave the orchestrator runnable and the workspace recoverable.
- Protected-file changes produce richer audit logs than ordinary task edits.
- Self-evolution remains opt-in and bounded by explicit safeguards.

---

## Milestone 7 — Close the loop between audit findings and execution

**Objective:** static analysis output should materially influence what the orchestrator works on next.

**Primary files:** `main_audit.go`, `main.go`

**Implementation steps**

1. Parse saved audit findings during task bootstrap when an audit artifact is present.
2. Convert actionable HIGH and CRITICAL findings into task inputs with stable IDs and provenance.
3. Prioritize audit-derived safety work ahead of routine backlog tasks without breaking explicit dependencies.
4. Log which scheduled tasks were sourced from audit findings.
5. Add tests that cover deduplication, priority ordering, and provenance tracking.

**Definition of done**

- Running audit mode changes the next execution plan in a deterministic way.
- Audit-derived tasks are visible in logs and distinguishable from document-derived tasks.
- Duplicate work is eliminated during task generation.

---

## Release Gate

Do not declare the roadmap complete until:

1. Each milestone ships with tests covering its new decision points.
2. `go vet ./...` passes after every milestone.
3. `go test -race ./...` passes after every milestone.
4. The orchestrator can recover from interruption, schedule split tasks correctly, and persist memory without branch contamination.
