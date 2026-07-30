# 50 — Mutation Adversarial and Full UAT

**What to build:** Establish the release gate for mutation safety using property, fault-injection, adversarial-path, concurrency, and real-corpus tests.

**Blocked by:** 49, 47

**Status:** ready-for-agent

## User story

As a maintainer, I want mutation safety proven under hostile inputs and failures so that enabling agent writes does not risk silent project corruption.

## Acceptance criteria

- [ ] Property tests prove a successful plan applies exactly its declared edits and a rejected plan changes zero target bytes.
- [ ] Fault injection covers permission denial, disk-full simulation, rename failure, interrupted process, corrupted journal, and cleanup failure.
- [ ] Adversarial paths cover traversal, symlink/junction escape, alternate separators, case aliases, reserved Windows names, and duplicate file identities.
- [ ] Concurrent apply attempts serialize or one refuses before mutation; no interleaved partial result is possible.
- [ ] Fuzz targets cover plan decoding, span arithmetic, diff generation, and journal recovery.
- [ ] A disposable copy of representative real distribution files passes rename/fix plan, apply, rollback, and re-index UAT.
- [ ] Tests assert diagnostics and exit/error types, not only message strings.
- [ ] The write-capability release gate fails if any rollback, containment, or stale-state test fails.

## UAT

1. Execute the mutation safety suite on Windows and one non-Windows CI runner.
2. Kill an apply process at each journal phase and run recovery.
3. Confirm every recovered fixture matches either the complete pre-state or complete post-state, never a mixture.
4. Attempt a junction escape and confirm the external target remains untouched.

## Non-goals

- Proving correctness of the IKEMEN engine itself.
- Benchmarking read-only query performance.
- Supporting network filesystems with weaker atomicity than documented.
