# Architecture

This document describes the architectural layers of quorum-review. Formal layer
definitions (names, descriptions, allowed dependencies) are maintained in
`docs/architecture.json`.

## Overview

quorum-review is a standalone Go CLI that builds a code graph from a repository's
AST, computes the blast radius of a git diff, and sends scoped context through a
multi-layer review pipeline to one or more LLM providers in parallel.

Data flows in one direction: **analyzer** extracts nodes and edges from source
code, **graph** stores them and computes blast radius, **reviewer** consumes the
scoped context to drive reviews, **provider** handles LLM communication, and
**output** formats the results. **cmd** wires everything together.

```
cmd
 |
 +-> reviewer --> graph --> analyzer
 |     |
 |     +-> provider --> config
 |     |
 |     +-> output ----> config
 |
 +-> config
```

## Layers

### cmd

**Purpose:** CLI entry point and composition root.

**Contains:** Cobra command definitions, flag parsing, dependency wiring. No
business logic lives here.

**Depends on:** All internal packages (this is where they get assembled).

### reviewer

**Purpose:** Orchestrates the three-layer review pipeline.

**Contains:** Intent review (spec vs. what was built), structural review
(blast-radius-scoped code correctness), and style review (idioms and clarity).
Handles prompt construction for each layer, parallel fan-out to multiple models,
and result aggregation including cross-model consensus.

**Depends on:** graph (for blast radius and structural context), provider (to
call LLMs), output (to emit findings), config (for model selection).

**Must not depend on:** cmd, analyzer.

### graph

**Purpose:** Code graph storage and blast-radius computation.

**Contains:** Node and edge data model, SQLite persistence, incremental update
logic (SHA-based change detection), and the blast-radius algorithm that computes
affected callers, callees, interface implementors, and related tests from a diff.

**Depends on:** analyzer (to populate the graph from parsed source).

**Must not depend on:** cmd, reviewer, provider, output.

### analyzer

**Purpose:** Language-specific AST parsing and semantic extraction.

**Contains:** The `LanguageAnalyzer` interface and the Go implementation using
`go/ast` and `go/types`. Extracts functions, types, interfaces, call edges, and
interface satisfaction edges. Designed for future language support (Rust, Elixir)
via Tree-sitter-backed implementations of the same interface.

**Depends on:** Nothing internal. Uses only the Go standard library.

**Must not depend on:** Any other internal package.

### provider

**Purpose:** LLM provider abstraction.

**Contains:** Backends for Anthropic (Claude), Google (Gemini), and OpenAI
(Codex/GPT). Each backend handles authentication, API serialization, rate
limiting, and response parsing behind a common interface.

**Depends on:** config (for API keys and model selection).

**Must not depend on:** cmd, graph, analyzer, reviewer, output.

### output

**Purpose:** Review output formatting.

**Contains:** Terminal renderer for interactive use and markdown generator for
file output (via `--markdown` flag). Groups findings by review layer (intent,
structural, style) and by model. Supports structured findings with file path,
line range, concern, and severity.

**Depends on:** config (for output preferences).

**Must not depend on:** cmd, graph, analyzer, reviewer, provider.

### config

**Purpose:** Configuration and model registry.

**Contains:** Config file loading, provider credential management, model
selection types, and default settings. Leaf layer with no internal dependencies.

**Depends on:** Nothing internal.

**Must not depend on:** Any other internal package.

## Cross-cutting Concerns

**Error handling:** Errors propagate up to cmd for user-facing display. Internal
packages return errors rather than logging or exiting directly.

**Context propagation:** All long-running operations accept `context.Context` for
cancellation support, particularly important for parallel LLM calls.

**Concurrency:** Parallel model fan-out in the reviewer layer uses goroutines
with `errgroup`. Provider backends must be safe for concurrent use.
