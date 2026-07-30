# v1 release-candidate acceptance

The release candidate is built only by `.github/workflows/release.yml` from a `v*` tag. The workflow emits Windows amd64 (primary), Linux amd64, macOS amd64, and macOS arm64 single binaries. Go and SQLite are not required on the target machine.

## Operator gate

1. Download the archive, checksum, SPDX SBOM, and attestation from the release.
2. Verify the GitHub artifact attestation (`gh attestation verify`) and SHA-256 before executing the binary.
3. Run `ikm version --json`, `ikm doctor --json --root <workspace>`, and `ikm workspace scan <workspace>`; retain stdout as protocol data and stderr as logs.
4. Run diagnostics, symbols, references, graph, impact, inspect, and export. Repeat through MCP initialize/discovery and compare canonical JSON envelopes.
5. Confirm `capabilities --json` lists read operations only. Mutation/runtime require separate opt-in policy and are absent by default.
6. Delete the disposable `.ikm` cache, rerun scan, and compare query results.

Record OS/architecture, binary SHA-256, release tag, commit, Go version, corpus identity, command outputs, waivers, known limitations, and rollback (restore the previous verified archive). High/critical safety, protocol, data-loss, and unexplained performance failures are not waivable. Tagging and publishing remain explicit maintainer actions after evidence review.

## Contract guarantees

JSON uses `contract.Envelope` schema `0.1.0`; paths are workspace-relative and secrets are never emitted. Findings have stable codes and remediation evidence. Read-only checks never mutate state; `doctor --fix --rebuild-cache` is the only explicit reversible maintenance action.
