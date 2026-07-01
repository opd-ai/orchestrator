# orchestrator

**Autonomous engineering orchestrator for local LLMs — build agentic workflows on consumer hardware.**

Orchestrator is a Go-based agentic loop that reads planning documents, generates atomic tasks, executes them against a local LLM, validates and applies the resulting patches, and commits clean, reviewable git history — all without human supervision.

It is purpose-built for **Qwen2.5 / Qwen3 Coder** and other open-weight models running on consumer CPUs via [Ollama](https://ollama.com) or any OpenAI-compatible endpoint.

---

## Features

- **Document-driven task generation** — reads `AUDIT.md`, `GAPS.md`, `GOALS.md`, `PLAN.md`, and `ROADMAP.md` to bootstrap a task queue automatically
- **DAG-based execution** — respects `depends_on` ordering; tasks run only when their dependencies are satisfied
- **Patch safety** — validates every diff before application: line-count limits, file-touch limits, and deletion-ratio guards
- **Automatic task splitting** — oversized or failing tasks are split into smaller atomic subtasks and re-queued
- **Build-and-fix loop** — runs `go build ./... && go test ./...` after each patch; retries with compiler output as context until the build passes or retries are exhausted
- **Structured JSON logging** — every event is appended to `orchestrator.log` in machine-readable format
- **Git branch isolation** — each run creates a fresh `autonomous/<timestamp>` branch; commits are small and traceable
- **Adaptive memory** — cross-run metrics (retry rates, patch sizes, branch history) are persisted and injected into the planner on the next run
- **Static audit mode** — standalone code-quality analysis: architecture, API surface, and concurrency passes against any Go package graph
- **Self-evolution mode** — elevated patch limits for the orchestrator to improve itself (`--self-evolve`)
- **Dry-run mode** — simulate a full run without writing files or committing

---

## Requirements

| Dependency | Purpose |
|---|---|
| Go 1.21+ | Build and run the orchestrator |
| `git` | Branch management and commits |
| `patch` | Apply unified diffs |
| Ollama (or compatible server) | Serve the local LLM |

Recommended models: **Qwen2.5-Coder-32B-Instruct** or **Qwen3-Coder** (quantized GGUF via Ollama).

---

## Quick Start

```bash
# 1. Pull a local model (example using Ollama)
ollama pull qwen2.5-coder:32b

# 2. Build the orchestrator
go build -o orchestrator .

# 3. Write a planning document in your target repo
echo "# GOALS\n\n- Add error handling to the HTTP client\n- Add unit tests for parser" > GOALS.md

# 4. Run
./orchestrator \
  --endpoint http://localhost:11434/v1/chat/completions \
  --model qwen2.5-coder:32b \
  --max-patch-lines 50 \
  --max-files 3 \
  --max-retries 5
```

The orchestrator will:
1. Parse `GOALS.md` (and any other planning docs it finds) into atomic tasks
2. Create a new git branch `autonomous/<timestamp>`
3. Execute each task in dependency order, applying and committing patches as they pass build validation

---

## CLI Flags

| Flag | Default | Description |
|---|---|---|
| `--endpoint` | `http://localhost:11434/v1/chat/completions` | OpenAI-compatible chat completions URL |
| `--model` | `local-27b` | Model name to pass in the request body |
| `--max-retries` | `5` | Maximum fix attempts per task before splitting or blocking |
| `--max-patch-lines` | `50` | Hard line-count limit per patch |
| `--max-files` | `3` | Maximum files a single patch may touch |
| `--max-runtime` | *(unlimited)* | Wall-clock time limit (e.g. `2h`, `30m`) |
| `--max-tasks` | *(unlimited)* | Maximum tasks to execute in one run |
| `--resume` | `false` | Resume on the current branch instead of creating a new one |
| `--dry-run` | `false` | Parse and plan without writing files or committing |
| `--verbose` | `false` | Print JSON log entries to stdout |
| `--self-evolve` | `false` | Raise patch limits to allow the orchestrator to modify itself |

### Audit mode flags

| Flag | Description |
|---|---|
| `--audit` | Enable static analysis mode |
| `--audit-pattern` | Go package pattern to analyse (default `./...`) |
| `--audit-pass` | One of `architecture`, `api`, `concurrency`, or `all` (default) |
| `--audit-output` | Output file for findings (default `audit_findings.json`) |

---

## Planning Documents

The orchestrator scans the working directory for any of the following files at startup and generates tasks from them if `tasks.json` does not already exist:

| File | Prefix | Purpose |
|---|---|---|
| `AUDIT.md` | `A` | Code-quality findings to address |
| `GAPS.md` | `G` | Missing features or coverage gaps |
| `GOALS.md` | `O` | Improvement goals |
| `PLAN.md` | `P` | Explicit implementation plans |
| `ROADMAP.md` | `R` | Milestone-level roadmap items |

Tasks are deduplicated by content hash across documents. Existing `tasks.json` files are never overwritten — delete or rename the file to re-bootstrap from documents.

---

## Audit Mode

Run a standalone static analysis pass on any Go codebase:

```bash
./orchestrator --audit --audit-pattern ./... --audit-pass all --audit-output findings.json
```

Audit passes:

- **architecture** — dependency cycles, package layering violations, file and function size
- **api** — exported symbol surface, interface drift, undocumented exports
- **concurrency** — shared-state access, goroutine safety, mutex usage

Findings are written as structured JSON to `--audit-output`.

---

## Execution Flow

```
Planning docs
     │
     ▼
ensureTasksFile()   ← generate + deduplicate tasks via LLM
     │
     ▼
nextExecutableTask() ← DAG ordering, deps satisfied check
     │
     ▼
resolveContextFiles() + gatherFileContext()
     │
     ▼
executeTask()       ← LLM returns unified diff
     │
     ▼
validatePatch()     ← line count, file count, deletion ratio
     │
     ▼
applyPatch()        ← `patch -p1`
     │
     ▼
build()             ← `go build ./... && go test ./...`
     │
   pass? ──yes──► gitCommit() → task complete
     │
    no
     │
     ▼
fixTask() loop      ← retry with compiler errors as context
     │
  exhausted? ──► splitTask() → re-queue smaller subtasks
```

---

## Memory & Adaptive Metrics

Cross-run state is stored under the `memory/` package and persisted to disk. On each run start, a compressed summary of prior metrics is injected into the planner prompt, allowing the orchestrator to:

- Prefer smaller patches in areas with high historical retry rates
- Recognise recently unstable files
- Track cumulative task throughput over many sessions

---

## Design Principles

- **Deterministic by default** — intelligence lives in the Go scaffolding, not in prompts
- **Micro-edits over large refactors** — patch caps are enforced at every level
- **Strict validation before mutation** — no patch reaches disk without passing structural checks
- **Retry convergence over reasoning depth** — the loop retries with structured error context rather than relying on model chain-of-thought
- **Bounded complexity** — all advanced behaviour is behind explicit flags and must remain reversible

---

## License

See [LICENSE](LICENSE).
