# 15 — Adopt supported Go and embedded SQLite

**What to build:** Upgrade the build baseline to supported Go 1.26 and bundle a pinned CGO-free SQLite driver after platform, license, size, and performance verification.

**Blocked by:** 11 — Define workspace manifest and snapshot identities

**Status:** ready-for-agent

- [ ] Go module, CI, docs, and release metadata use a supported Go 1.26 patch release.
- [ ] The selected SQLite driver and matching transitive versions are pinned and license-audited.
- [ ] Windows, Linux, and macOS binaries build with CGO disabled.
- [ ] Driver tests prove FTS5, transactions, locking, cancellation, and migration prerequisites.
- [ ] Binary size/build-time impact is measured and accepted against a documented ceiling.
