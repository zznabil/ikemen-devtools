# 08 — Enforce root containment and path sandboxing

**What to build:** One cross-platform path-authority module used before every read, resource, export, cache, runtime, and write operation.

**Blocked by:** 07 — Define workspace configuration, root, profile, and budgets

**Status:** ready-for-agent

- [ ] Canonicalization handles separators, case behavior, symlinks, junctions, relative paths, and file URIs.
- [ ] Traversal and link escapes are denied after resolution, including on Windows.
- [ ] Explicit external roots grant only their canonical subtree.
- [ ] Workspace-relative paths are preserved for stable identities and output.
- [ ] Adversarial cross-platform tests cover read and write boundaries.
