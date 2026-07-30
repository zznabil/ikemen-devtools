# 35 — MCP Concurrency, Batch, and Transport Hardening

**What to build:** Harden the stdio JSON-RPC loop for concurrent requests, notifications, batches, malformed input, and orderly shutdown.

**Blocked by:** 34, 01

**Status:** ready-for-agent

## User story

As an MCP client, I want a standards-compliant server under concurrent and malformed traffic so that one bad request cannot corrupt other responses or kill the process.

## Acceptance criteria

- [ ] Support the project’s declared JSON-RPC batch behavior, including empty batch, mixed requests/notifications, invalid entries, and response ordering policy.
- [ ] Notifications never receive responses; request IDs preserve exact JSON type.
- [ ] Concurrent read requests may execute in parallel while stdout writes remain framed and serialized.
- [ ] Per-request contexts isolate cancellation, deadlines, metadata, and errors.
- [ ] Oversized/deep/malformed JSON is rejected within configured memory and message limits.
- [ ] Panic recovery converts request-local failures into internal errors and leaves the server usable where safe.
- [ ] EOF, client shutdown, and process signals cancel in-flight work and close repository resources deterministically.
- [ ] Logs/progress never appear on stdout.
- [ ] Race, fuzz, stress, and protocol fixtures cover interleaving, cancellation races, duplicate IDs, and invalid batches.

## UAT

1. Send a batch containing two valid calls, a notification, and an invalid entry.
2. Confirm responses comply with JSON-RPC and no notification response is emitted.
3. Run concurrent large and small requests; confirm valid framing and that cancellation is request-local.
4. Send malformed and oversized messages, then a valid request; confirm bounded errors and continued service where specified.

## Non-goals

- HTTP/SSE transport.
- Multi-user scheduling.
- Cross-process request persistence.
