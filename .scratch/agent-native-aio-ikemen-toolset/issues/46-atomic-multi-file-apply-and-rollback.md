# 46 — Atomic Multi-File Apply and Rollback

**What to build:** Replace best-effort multi-file writes with a transactional apply path that either commits the complete validated plan or restores the original workspace.

**Blocked by:** 45, 08

**Status:** ready-for-agent

## User story

As an agent, I want patch application to be atomic so that a disk error or concurrent edit cannot leave an IKEMEN project half-modified.

## Acceptance criteria

- [ ] `ikm patch apply <plan>` validates root containment, workspace identity, all hashes, spans, encodings, and postconditions before the first target is replaced.
- [ ] New contents are written to same-volume temporary files, flushed, and swapped into place using the safest supported platform primitive.
- [ ] Original files are retained until every replacement succeeds.
- [ ] Any failure restores every already-replaced file byte-for-byte and returns a typed transaction error.
- [ ] A transaction journal records phase and recovery data without storing files outside the configured workspace cache.
- [ ] Startup detects interrupted journals and offers a deterministic `patch recover` operation.
- [ ] Successful completion removes recovery artifacts and invalidates affected snapshots/index rows.
- [ ] Dry-run is the default unless the caller explicitly selects apply.
- [ ] Tests inject failures before staging, during replacement, after one replacement, and during cleanup.

## UAT

1. Apply a valid two-file plan and confirm both files and the semantic index reflect the new state.
2. Inject a failure after the first replacement.
3. Confirm both files match their original hashes and the journal clearly reports the recovered transaction.
4. Modify a target concurrently before apply; expect zero writes and a stale-plan error.

## Non-goals

- Cross-volume transactions.
- Source-control commits.
- Background file watching.
