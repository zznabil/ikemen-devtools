# Release gates

The release gate consumes `.release/thresholds.json` and records JSON `{binary, version, platform, corpus, metrics, thresholds, pass}`. Cold scan, cold semantic build, full metadata, warm refresh, indexed-query p50/p95, MCP overhead, peak RSS, cache size, and cancellation latency are measured. Comparisons use repeated samples and the platform tolerance; one noisy run never replaces a baseline.

Security fixtures must exercise root escape, symlink/junction traversal, malicious URI/path input, oversized messages, decompression/parse bombs, JSON depth, output budgets, graph-node budgets, and cancellation. Race coverage includes shared sessions, repository access, MCP concurrency, cancellation, and mutation serialization. A deliberate threshold breach fails the workflow. Exceptions require an explicit threshold edit, rationale, reviewer approval, and evidence artifact.
