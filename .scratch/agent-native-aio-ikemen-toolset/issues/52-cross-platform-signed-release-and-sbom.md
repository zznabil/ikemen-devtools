# 52 — Cross-Platform Signed Release and SBOM

**What to build:** Produce reproducible single-binary releases with checksums, provenance, and a software bill of materials for supported platforms.

**Blocked by:** 15, 51

**Status:** ready-for-agent

## User story

As an agent operator, I want a verifiable standalone binary so that installation is simple and supply-chain identity is machine-checkable.

## Acceptance criteria

- [ ] Document the supported OS/architecture matrix, with Windows amd64 as the primary IKEMEN target.
- [ ] CI builds one `ikm`/`ikm.exe` per target without a runtime installer, external database process, or C toolchain.
- [ ] Release archives include the binary, license notices, checksums, SBOM, and verification instructions.
- [ ] Build metadata embedded in the binary matches the release tag and provenance statement.
- [ ] Artifacts are signed using the repository’s selected keyless or managed signing workflow; verification is automated in CI.
- [ ] Repeated builds use pinned toolchain/dependencies and are reproducible to the documented practical boundary.
- [ ] A clean machine smoke test runs `version`, `doctor`, `workspace scan`, CLI query, MCP initialize/discovery, and LSP initialize.
- [ ] Release workflow refuses a dirty version contract, failing tests, vulnerable forbidden dependency, or mismatched artifact hash.

## UAT

1. Download the Windows amd64 release into a clean VM containing a representative IKEMEN workspace.
2. Verify signature, checksum, SBOM presence, and embedded version.
3. Run the smoke suite without installing Go or SQLite.
4. Tamper with the binary and confirm verification fails.

## Non-goals

- MSI/GUI installers.
- Auto-update services.
- Code-signing commitments that require unavailable credentials in local development.
