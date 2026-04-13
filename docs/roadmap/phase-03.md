## Phase 3: Review Pipeline

**Goal**: `quorum review main..feature --model claude-sonnet-4-6` runs a three-layer review (intent, structural, style) against a single LLM provider and displays findings grouped by layer in the terminal.

**Milestone**: `Phase 3: Review Pipeline` | **Label**: `phase-3`

- Review layer definitions (intent, structural, style) with distinct prompts
- Intent layer: prompt construction from issue/spec + high-level change summary
- Structural layer: prompt construction from blast radius subgraph + diff content
- Style layer: prompt construction from changed code only
- ReviewProvider interface definition
- Anthropic (Claude) provider backend with API key auth
- Config file loading for API keys and model selection
- quorum review CLI command with --issue and --spec flags
- Terminal output renderer with findings grouped by layer and severity
- Unit tests for prompt construction
- Integration tests for provider with recorded HTTP fixtures
