# Agent recipes

Discover first: `ikm capabilities --json` and MCP `initialize`/`tools/list`. Use only names and schemas returned by discovery.

Scan: `ikm workspace scan <root> --json`; retain `snapshot.id` and `page.nextCursor`. Query diagnostics/symbols/references with the cursor and bounded `limit`; stop on `truncated` and follow remediation.

Stale snapshot: restart the operation with the latest workspace status snapshot; never apply results from a stale snapshot.

Cancellation: send the operation cancellation request; treat cancellation as a terminal bounded result and discard partial mutation state.

Mutation: prepare a plan, review its diff and authorization, then apply only after explicit approval. Recovery uses the disposable copy and rollback artifact; MCP remains read-only unless policy enables mutation.

Runtime oracle: request an allowed fixture and preserve source-to-runtime evidence IDs; do not execute arbitrary runtime input.

Every request/result is validated against `docs/schema/catalog-v0.1.json`. stdout is protocol JSON; stderr is progress and diagnostics.
