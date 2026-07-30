# Release metadata

`ikm metadata` is the current release-metadata entry point. It hashes the repeated `--file` inputs in canonical path order and emits compact canonical JSON:

```text
go run ./cmd/ikm metadata \
  --timestamp 2026-07-30T12:00:00Z \
  --file README.md --file go.mod > release-metadata.json
```

The timestamp is required and must parse as RFC3339. The default module is `github.com/ikemen-engine/ikemen-devtools`; the default metadata contract is the current IR identity contract (`0.2.0`). Override them with `--module` and `--contract` when producing a deliberately different artifact. Build/toolchain, OS/architecture, and VCS fields are populated by the CLI when available; each file entry contains a SHA-256 digest.

The Go `internal/release` package also exposes `Build`, `CanonicalJSON`, `Sign`, and `Verify`. Signing uses Ed25519 and canonical JSON with the signature removed from the signed payload. Local development does not require release credentials; the hosted workflow supplies keyless signing and provenance.

## v1 release workflow

`.github/workflows/release.yml` builds pinned Go single binaries for Windows amd64, Linux amd64, macOS amd64, and macOS arm64 with `-trimpath` and tag-embedded version metadata. Each artifact has a SHA-256 manifest signed by keyless Sigstore Cosign, an SPDX SBOM from Anchore Syft, license notices, and a GitHub artifact attestation. The publish job verifies checksums and signatures before uploading.

`.release/thresholds.json` is the versioned performance, memory, input-bound, and security gate contract. Benchmark output must include binary version, platform, and corpus identity; threshold changes require reviewed rationale. The compiled UAT job exercises version, doctor, workspace scan, capabilities, MCP initialize/discovery, and LSP initialize on the built binary.
