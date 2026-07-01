# Orchestrator Roadmap
Deterministic-First Evolution Plan
Target Environment: Quantized LLMs on Consumer Hardware

--------------------------------------------------------------------
Guiding Principles
--------------------------------------------------------------------

- Deterministic by default.
- Sophisticated only when necessary.
- Intelligence resides in scaffolding, not prompts.
- Micro-edits over large refactors.
- Strict validation before mutation.
- Escalation must be metric-triggered.
- Complexity must remain bounded.
- All advanced behavior must be reversible.
- Optimize for retry convergence, not reasoning depth.
- Preserve architectural invariants at all times.

====================================================================
FOUNDATION PHASE — Deterministic Reliability First
====================================================================

--------------------------------------------------------------------
MILESTONE 1 — Deterministic Stability Foundation
--------------------------------------------------------------------

Goal:
Eliminate avoidable model failures and reduce retries.

Completion Checklist:

[x] Implement compile error classifier
    - undefined symbol
    - unused import
    - missing import
    - redeclaration
    - type mismatch

[x] Generate structured fix hints instead of raw compiler output

[x] Implement automatic trivial fix engine
    - remove unused imports
    - normalize import blocks
    - auto-run go fmt
    - fix obvious missing imports

[x] Implement strict patch shape validator
    - reject >30% file deletions
    - reject full file rewrites
    - reject unexpected renames
    - enforce file-touch limits
    - enforce line delta caps

[x] Implement deterministic task granularity enforcer
    - split oversized tasks
    - split multi-file tasks pre-execution

Success Criteria:
- Reduced retry rate
- Fewer catastrophic patches
- Stable micro-edit execution

--------------------------------------------------------------------
MILESTONE 2 — Structured Prompt Protocol
--------------------------------------------------------------------

Goal:
Replace prose prompts with compact execution protocol.

Completion Checklist:

[x] Define structured execution block format
    - MODE: PLAN | EXECUTE | FIX | ARCH_PLAN
    - TASK_ID
    - FILES_ALLOWED
    - MAX_PATCH_LINES
    - CONSTRAINTS
    - FAIL_REASON (optional)

[x] Implement prompt compression layer
    - remove verbose prose
    - remove redundant instructions
    - cap token usage aggressively

[x] Inject compressed adaptive memory summary (≤10 lines)

[x] Enforce token budget per call

Success Criteria:
- Lower token usage
- Higher diff formatting reliability
- Reduced hallucinated edits

--------------------------------------------------------------------
MILESTONE 3 — Deterministic Planning Engine
--------------------------------------------------------------------

Goal:
Move planning intelligence into Go code.

Completion Checklist:

[x] Implement AST-based impact analyzer
    - parse Go files
    - extract function boundaries
    - extract struct definitions
    - build symbol map

[x] Implement symbol-level task generator
    - add function
    - modify struct
    - update method
    - adjust imports

[x] Build deterministic dependency DAG

[x] Restrict model context to function-level scope when possible

Success Criteria:
- Smaller prompts
- Smaller diffs
- Reduced planning failures

--------------------------------------------------------------------
MILESTONE 4 — Adaptive Control Layer
--------------------------------------------------------------------

Goal:
Self-tuning behavior via structured memory.

Completion Checklist:

[x] Implement dynamic patch budget controller
    - derive from avg_success_patch_size
    - clamp within safe bounds

[x] Implement failure pattern compression
    - track top N failure types
    - track problematic files

[x] Implement patch confidence scoring
    - entropy scoring
    - deletion ratio detection
    - structural churn detection

[x] Implement retry convergence monitor

Success Criteria:
- Stable patch sizes
- Retry average below threshold
- Improved convergence consistency

--------------------------------------------------------------------
MILESTONE 5 — CPU-Optimized Parallel Execution
--------------------------------------------------------------------

Goal:
Trade cores for reliability.

Completion Checklist:

[x] Implement multi-branch speculative execution

[x] Evaluate branches using:
    - build success
    - risk score
    - diff entropy
    - retry count

[x] Implement temperature strategy controller
    - low-temp first pass
    - moderate-temp fallback

[x] Ensure concurrency-safe execution manager

Success Criteria:
- Higher complex-task success
- Controlled parallelism
- Stable memory integrity

--------------------------------------------------------------------
MILESTONE 6 — Model Load Optimization
--------------------------------------------------------------------

Goal:
Reduce inference burden aggressively.

Completion Checklist:

[x] Enforce hard prompt token budget

[x] Implement task hash caching

[x] Support model role specialization
    --planner-model
    --executor-model
    --architect-model

[x] Log token usage metrics

Success Criteria:
- Reduced inference frequency
- Faster iteration loops
- Lower CPU usage

--------------------------------------------------------------------
MILESTONE 7 — Self-Simplification Engine
--------------------------------------------------------------------

Goal:
Make codebase easier for quantized models.

Completion Checklist:

[ ] Implement complexity audit tool
    - track function length
    - track file growth
    - basic cyclomatic complexity

[ ] Implement dead code detection

[ ] Implement invariant registry
    architecture/invariants.json

[ ] Inject compressed invariant summary into prompts

Success Criteria:
- Reduced structural complexity
- Stable architectural boundaries
- Improved long-term reliability

--------------------------------------------------------------------
MILESTONE 8 — Structured Execution DSL
--------------------------------------------------------------------

Goal:
Turn model into transformation engine.

Completion Checklist:

[ ] Define execution DSL
    EXECUTE { file, change_type, max_lines }

[ ] Implement schema validator

[ ] Enforce transformation-only mode

[ ] Reject malformed execution instructions

Success Criteria:
- Predictable diffs
- Minimal hallucination
- Stable micro-edit behavior

--------------------------------------------------------------------
MILESTONE 9 — Autonomous Adaptive Scheduler
--------------------------------------------------------------------

Goal:
Self-optimizing execution under consumer hardware constraints.

Completion Checklist:

[ ] Implement reward scoring engine
    - retries
    - patch size
    - build speed

[ ] Implement hardware-aware scheduler
    - monitor CPU load
    - adjust parallelism
    - adjust patch limits

[ ] Implement long-run stability monitor
    - detect oscillation
    - detect regressions
    - trigger safe mode

Success Criteria:
- Stable extended runs
- Controlled compute usage
- No runaway behavior

====================================================================
ADVANCED SOPHISTICATION LAYER
(Only after deterministic reliability achieved)
====================================================================

--------------------------------------------------------------------
MILESTONE 10 — Tiered Intelligence Escalation
--------------------------------------------------------------------

Goal:
Introduce controlled sophistication tiers.

Completion Checklist:

[ ] Define Intelligence Tiers:
    - Tier 0: deterministic micro-edit (default)
    - Tier 1: coordinated multi-file edit
    - Tier 2: architectural planning mode
    - Tier 3: experimental speculative mode (opt-in)

[ ] Implement escalation triggers:
    - retry threshold exceeded
    - high centrality file touched
    - public interface modification detected
    - repeated subsystem instability

[ ] Implement escalation guardrails
    - hard patch caps
    - invariant validation required
    - successful build mandatory

[ ] Implement automatic de-escalation logic

Success Criteria:
- Default execution remains simple
- Sophistication activates only when necessary

--------------------------------------------------------------------
MILESTONE 11 — Risk-Aware Patch Scoring Engine
--------------------------------------------------------------------

Goal:
Score patch risk before mutation.

Completion Checklist:

[ ] Define risk factors
    - deletion ratio
    - file modification percentage
    - file centrality
    - retry count
    - API surface modification

[ ] Implement weighted risk scoring function

[ ] Define risk thresholds (low / medium / high)

[ ] Enforce risk-based gating

[ ] Log risk metrics in memory branch

Success Criteria:
- Reduced destabilizing patches
- Quantifiable mutation risk

--------------------------------------------------------------------
MILESTONE 12 — Cross-Task Semantic Grouping
--------------------------------------------------------------------

Goal:
Enable subsystem-aware coordination.

Completion Checklist:

[ ] Build subsystem map
    - directory clusters
    - symbol reference graph
    - interface dependency map

[ ] Detect clustered tasks

[ ] Merge related micro-tasks when safe

[ ] Respect patch caps after merge

[ ] Track subsystem stability metrics

Success Criteria:
- Cleaner subsystem-level changes
- Reduced retry cascades

--------------------------------------------------------------------
MILESTONE 13 — Architectural Invariant Enforcement
--------------------------------------------------------------------

Goal:
Allow sophistication without structural decay.

Completion Checklist:

[ ] Define invariants
    - no cyclic dependencies
    - no global mutable state
    - max function length
    - max file length

[ ] Implement pre-execution invariant validator

[ ] Implement post-patch invariant check

[ ] Block patches violating invariants

Success Criteria:
- Architectural stability preserved
- Controlled evolution

--------------------------------------------------------------------
MILESTONE 14 — Parallel Strategy Competition
--------------------------------------------------------------------

Goal:
Use CPU parallelism to approximate deeper reasoning.

Completion Checklist:

[ ] Implement multi-strategy branch generation

[ ] Allow alternative design strategies

[ ] Evaluate strategies using:
    - build success
    - risk score
    - diff entropy
    - complexity delta

[ ] Prune losing branches automatically

Success Criteria:
- Higher success for ambiguous architectural tasks
- Controlled speculative exploration

--------------------------------------------------------------------
MILESTONE 15 — Selective Model Escalation
--------------------------------------------------------------------

Goal:
Escalate model size only when required.

Completion Checklist:

[ ] Implement model escalation conditions

[ ] Add cooldown period between escalations

[ ] Log escalation events

[ ] Automatically revert to smaller model after completion

Success Criteria:
- Low average compute cost
- Rare but effective escalations

--------------------------------------------------------------------
MILESTONE 16 — Internal Simulation Layer
--------------------------------------------------------------------

Goal:
Catch structural failures before disk write.

Completion Checklist:

[ ] Implement in-memory patch application

[ ] Run AST validation in memory

[ ] Detect syntax and type failures pre-build

[ ] Reject invalid patches before file write

Success Criteria:
- Fewer build failures
- Faster retry convergence

--------------------------------------------------------------------
MILESTONE 17 — Subsystem Stability Analytics
--------------------------------------------------------------------

Goal:
Adaptive mutation pressure per subsystem.

Completion Checklist:

[ ] Track per-subsystem metrics
    - retry rate
    - risk score average
    - patch size average

[ ] Detect unstable subsystems

[ ] Reduce patch budget in unstable areas

[ ] Increase budget in stable areas

Success Criteria:
- Stabilization over long sessions
- Reduced oscillation patterns

--------------------------------------------------------------------
MILESTONE 18 — Strategic Review Mode (Rare Invocation)
--------------------------------------------------------------------

Goal:
Allow bounded architectural reasoning bursts.

Completion Checklist:

[ ] Define strategic review trigger conditions

[ ] Expand context temporarily (bounded)

[ ] Require invariant validation post-review

[ ] Require successful build before acceptance

[ ] Automatically return to Tier 0 after completion

Success Criteria:
- Controlled architectural correction
- No runaway complexity growth

====================================================================
Non-Goals Under Consumer Constraints
====================================================================

- Large embedding databases
- Long chain-of-thought prompting
- Massive context windows
- Heavy RAG infrastructure
- Multi-repo reasoning
- Continuous architectural rewriting

====================================================================
Final Target State
====================================================================

The orchestrator:

- Defaults to deterministic micro-edits.
- Escalates sophistication only when triggered.
- Scores risk before mutation.
- Enforces invariants automatically.
- Uses parallel speculation instead of deeper reasoning.
- Escalates model size rarely and deliberately.
- Tracks subsystem stability.
- Self-adjusts mutation pressure.
- Runs stably for long sessions on consumer CPUs with quantized models.

Sophistication is layered on top of reliability,
never replacing it.