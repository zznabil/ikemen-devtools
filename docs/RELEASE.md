# Release metadata

`ikm metadata` is the current release-metadata entry point. It hashes the repeated `--file` inputs in canonical path order and emits compact canonical JSON:

```text
go run ./cmd/ikm metadata \
  --timestamp 2026-07-30T12:00:00Z \
  --file README.md --file go.mod > release-metadata.json
```

The timestamp is required and must parse as RFC3339. The default module is `github.com/ikemen-engine/ikemen-devtools`; the default metadata contract is the current IR identity contract (`0.2.0`). Override them with `--module` and `--contract` when producing a deliberately different artifact. Build/toolchain, OS/architecture, and VCS fields are populated by the CLI when available; each file entry contains a SHA-256 digest.

The Go `internal/release` package also exposes `Build`, `CanonicalJSON`, `Sign`, and `Verify`. Signing uses Ed25519 and canonical JSON with the signature removed from the signed payload. Release signing, changelog generation, installers, checksums publication, SBOMs, and CI release automation are not yet bundled; consumers must provide those controls externally.
