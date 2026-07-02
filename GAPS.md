# Implementation Gaps — 2026-07-02

## Gap 1: Architecture audit pass misses documented cycle/layer checks
- **Intended Behavior**: Audit architecture pass should detect dependency cycles, package layering violations, and file/function size concerns.
- **Current State**: `RunArchitecturePass` only aggregates hotspots, isolated packages, and dead-function candidates (`/home/runner/work/orchestrator/orchestrator/audit/passes.go:8-12`), and dependency graph construction has no cycle/layer evaluation (`/home/runner/work/orchestrator/orchestrator/audit/graph.go:3-14`).
- **Blocked Goal**: README audit-mode architecture promises are only partially delivered (`/home/runner/work/orchestrator/orchestrator/README.md:121`).
- **Implementation Path**: Add graph cycle detection and layer-rule validation in `audit`, then emit architecture findings from those checks alongside hotspot checks.
- **Dependencies**: None.
- **Effort**: Medium.

## Gap 2: API audit pass does not implement interface-drift/documentation checks
- **Intended Behavior**: API pass should assess exported surface, interface drift, and undocumented exports.
- **Current State**: API pass only reports exported interfaces and package export-count thresholds (`/home/runner/work/orchestrator/orchestrator/audit/passes.go:28-31,91-143`).
- **Blocked Goal**: README API-pass contract remains partially unmet (`/home/runner/work/orchestrator/orchestrator/README.md:122`).
- **Implementation Path**: Extend `RunAPIPass` to (1) detect interface/implementation drift and (2) flag undocumented exports via AST comment checks.
- **Dependencies**: Gap 1 is independent; can be implemented in parallel.
- **Effort**: Medium.

## Gap 3: Concurrency audit pass is a heuristic stub
- **Intended Behavior**: Concurrency pass should analyze shared-state access, goroutine safety, and mutex behavior.
- **Current State**: Pass returns a generic finding when concurrency imports are present (`/home/runner/work/orchestrator/orchestrator/audit/passes.go:33-35,146-169`) without real concurrency-path analysis.
- **Blocked Goal**: README concurrency-pass scope is not met (`/home/runner/work/orchestrator/orchestrator/README.md:123`).
- **Implementation Path**: Add AST checks for goroutine captures, mutable shared variable access, and lock/unlock discipline; surface concrete file/function findings.
- **Dependencies**: None.
- **Effort**: Large.

## Gap 4: Symbol-level planning component is not connected to runtime planning
- **Intended Behavior**: Symbol-level task generation should decompose work into deterministic symbol-scoped tasks.
- **Current State**: `main_symtask.go` functions are unreachable in runtime code (`/home/runner/work/orchestrator/orchestrator/main_symtask.go:29-124`; deadcode scan findings).
- **Blocked Goal**: Roadmap milestone 3 completion item “Implement symbol-level task generator” is not realized end-to-end (`/home/runner/work/orchestrator/orchestrator/ROADMAP.md:110-114`).
- **Implementation Path**: Invoke `symbolTasksForFiles` from planning/granularity flow, persist generated subtasks, and add integration tests proving execution path usage.
- **Dependencies**: None.
- **Effort**: Medium.

## Gap 5: Function DAG utility is dead and unconsumed
- **Intended Behavior**: Function-level DAG should be available to influence deterministic dependency/order decisions.
- **Current State**: All core DAG functions in `audit/dag.go` are unreachable in non-test execution (`/home/runner/work/orchestrator/orchestrator/audit/dag.go:26-151`; deadcode scan output).
- **Blocked Goal**: Deterministic dependency-DAG capability is incomplete as an operational feature (`/home/runner/work/orchestrator/orchestrator/ROADMAP.md:116-118`).
- **Implementation Path**: Either wire DAG output into planner/audit ordering logic or remove the unused code path and tests to avoid misleading architecture signals.
- **Dependencies**: Gap 4 if DAG use is intended for symbol-level planning.
- **Effort**: Medium.

## Gap 6: Redundant unreachable LLM wrapper in main package
- **Intended Behavior**: LLM invocation helpers should be minimal and actively used.
- **Current State**: `callLLM` is defined but unreachable (`/home/runner/work/orchestrator/orchestrator/main.go:227-229`; deadcode scan output).
- **Blocked Goal**: No direct feature block; this is maintainability debt.
- **Implementation Path**: Remove `callLLM` or route at least one stable call path through it, then keep one tested invocation abstraction.
- **Dependencies**: None.
- **Effort**: Small.
