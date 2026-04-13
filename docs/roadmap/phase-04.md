## Phase 4: Multi-Model and Output

**Goal**: `quorum review main..feature --model gemini-2.5-pro --model claude-sonnet-4-6 --model codex-mini --html report.html` runs parallel reviews across multiple LLM providers and outputs findings as terminal, markdown, or HTML.

**Milestone**: `Phase 4: Multi-Model and Output` | **Label**: `phase-4`

- Google (Gemini) provider backend
- OpenAI (Codex/GPT) provider backend
- Parallel fan-out across models using errgroup
- Cross-model consensus summary (agreement/disagreement on findings)
- --model flag (repeatable) for selecting multiple models per run
- --markdown flag for markdown file output
- --html flag for HTML report output
- HTML report template with findings grouped by layer and model
- Unit tests for consensus aggregation
- Integration tests for multi-model fan-out with recorded fixtures
