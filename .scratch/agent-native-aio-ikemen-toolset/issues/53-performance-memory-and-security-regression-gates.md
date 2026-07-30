# 53 — Performance, Memory, and Security Regression Gates

**What to build:** Turn the specification’s real-corpus budgets and safety invariants into repeatable release-blocking benchmarks and security tests.

**Blocked by:** 14, 18, 36, 44, 50

**Status:** ready-for-agent

## User story

As a maintainer, I want measurable release gates so that product growth does not silently make agent queries slow, unbounded, or unsafe.

## Acceptance criteria

- [ ] Add deterministic benchmark fixtures plus an opt-in real-distribution benchmark harness.
- [ ] Measure cold scan, cold semantic build, full metadata, warm refresh, indexed query p50/p95, MCP overhead, peak RSS, cache size, and cancellation latency.
- [ ] Encode the PRD targets and bounded-input defaults as versioned thresholds with platform-aware tolerances.
- [ ] CI fails on statistically meaningful regressions beyond the documented tolerance rather than single-run noise.
- [ ] Security regression tests cover root escape, symlink/junction behavior, malicious URI/path input, oversized messages, decompression/parse bombs, JSON depth, and output-budget enforcement.
- [ ] Race tests cover shared sessions, repository access, MCP concurrency, cancellation, and mutation serialization.
- [ ] Benchmark results are machine-readable and include binary/version/corpus identity.
- [ ] A documented exception process requires an explicit threshold change and rationale, never silent baseline replacement.

## UAT

1. Run the harness against the measured distribution corpus.
2. Confirm results cover the PRD’s cold, warm, indexed, MCP, and memory targets.
3. Introduce a fixture that exceeds an output or graph-node budget; expect a typed bounded-result response.
4. Introduce a deliberate benchmark regression and confirm the comparison gate fails.

## Non-goals

- Micro-optimizing code without profile evidence.
- Treating wall-clock equality across unlike CI machines as meaningful.
- Uploading the private game corpus.
