Self-Improvement Goals for the Autonomous Engineering Orchestrator

This document defines the next evolutionary goals for the orchestrator itself.
All improvements must be incremental, safe, and reversible.

The orchestrator must improve without destabilizing its own execution loop.

---

## GLOBAL CONSTRAINTS

- All changes must remain backward compatible.
- No single patch may exceed 50 lines.
- No large refactors in one task.
- Execution loop must never be removed or rewritten in one change.
- Git history must remain readable.
- System must remain deterministic.
- All new features must be behind flags when possible.

---

# PHASE 1 — Stability & Safety Hardening

## 1.1 Patch Safety

- Add detection for deletion-heavy patches (>30% deletions).
- Reject patches modifying more than 3 files.
- Add protection against modification of orchestrator.go unless explicitly allowed.

## 1.2 Execution Safety

- Add automatic rollback if a task becomes blocked.
- Reset working directory before each task.
- Detect dirty git state before applying patch.

## 1.3 LLM Response Validation

- Validate unified diff format before applying.
- Validate JSON from planner and splitter strictly.
- Retry LLM call if malformed output is returned.

---

# PHASE 2 — Observability Improvements

## 2.1 Structured Metrics

- Track:
  - Tasks completed
  - Tasks blocked
  - Average retries per task
  - Total patches applied

- Emit periodic summary logs.

## 2.2 Failure Recording

- Save build failures to:
  logs/build_failures/<task_id>.log

- Save rejected patches to:
  logs/rejected_patches/<task_id>.diff

## 2.3 Run Summary Report

At completion, generate:

AUTONOMOUS_RUN_SUMMARY.md

Containing:
- Total tasks
- Completed tasks
- Blocked tasks
- Execution duration
- Git branch name

---

# PHASE 3 — Smarter Planning

## 3.1 Dependency Inference

- Infer task dependencies automatically based on:
  - File overlap
  - Shared keywords
  - Explicit phrases ("after", "requires")

## 3.2 Task Priority System

Add priority levels:
- critical
- high
- normal
- low

Execution order should respect priority before FIFO.

## 3.3 Task Aging

Tasks retried >3 times should be deprioritized.

---

# PHASE 4 — Context Optimization

## 4.1 File Relevance Scoring

Improve resolveContextFiles():
- Score files by keyword frequency.
- Prefer recently modified files.
- Limit context size by total characters, not file count.

## 4.2 Context Summarization

If context exceeds safe size:
- Summarize file content before sending to LLM.
- Preserve signatures and public interfaces.

---

# PHASE 5 — Autonomous Refactoring Capability

## 5.1 Refactor Mode

Add a flag:
--refactor

When enabled:
- Allow structural changes.
- Increase patch size limit to 120 lines.
- Require build to pass before commit.

## 5.2 Test-Aware Refactoring

Before refactor:
- Snapshot test results.
After refactor:
- Ensure no new failures introduced.

---

# PHASE 6 — Long-Running Robustness

## 6.1 Crash Recovery

On startup:
- Detect interrupted tasks.
- Resume from last safe checkpoint.

## 6.2 Journal File

Create:
orchestrator.journal

Track:
- Current task
- Retry count
- Last applied patch hash

Use journal to resume safely.

---

# PHASE 7 — Controlled Self-Modification

## 7.1 Orchestrator Self-Edit Guard

When modifying orchestrator.go:
- Require:
  - Build passes
  - Tests pass
  - Diff preview logged
- Auto-create backup before patch.

## 7.2 Two-Phase Self-Modification

For changes to orchestrator.go:
1. Apply change to copy: orchestrator_next.go
2. Compile
3. Replace original only if successful

---

# PHASE 8 — Autonomous Learning Loop

## 8.1 Post-Task Reflection

After each task:
- Generate a short reflection:
  - What went wrong?
  - Why did retries occur?
  - How to reduce retries next time?

Store reflections in:
logs/reflections/<task_id>.md

## 8.2 Heuristic Adaptation

Use reflections to:
- Adjust retry thresholds
- Adjust context size
- Adjust temperature dynamically

---

# PHASE 9 — Governance & Limits

## 9.1 Resource Limits

Add configurable limits:
- Max runtime duration
- Max tasks per run
- Max commits per run

## 9.2 Emergency Stop

If:
- >5 tasks blocked consecutively
- Or build failing >20 iterations

Stop execution automatically.

---

# SUCCESS CRITERIA

The orchestrator is considered improved when:

- It can modify itself safely.
- It can resume after crash.
- It can reject unsafe patches.
- It generates run summaries.
- It handles dependency ordering.
- It improves stability across multiple runs.

---

# LONG TERM VISION

The orchestrator evolves into:

- A deterministic engineering engine
- Capable of safe self-improvement
- Governed by strict patch boundaries
- Operating continuously without human supervision
- Producing clean, reviewable git history
