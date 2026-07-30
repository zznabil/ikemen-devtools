# 07 — Define workspace configuration, root, profile, and budgets

**What to build:** A versioned workspace configuration contract with explicit root authority, profile, entry points, cache mode, includes/excludes, and bounded resource limits.

**Blocked by:** 04 — Define the versioned operation, output, and exit contract

**Status:** ready-for-agent

- [ ] Flags override workspace config, which overrides documented defaults.
- [ ] Root, profile, entry points, cache, adapters, external roots, and budgets are represented.
- [ ] Config cannot independently grant write or runtime authority.
- [ ] Unknown versions/fields and invalid limits return typed configuration failures.
- [ ] Canonical config digests are deterministic and recorded in results.
