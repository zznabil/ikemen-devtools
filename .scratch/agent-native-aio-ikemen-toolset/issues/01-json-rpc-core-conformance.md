# 01 — Make JSON-RPC core conformant

**What to build:** A transport-independent JSON-RPC dispatcher that correctly handles requests, explicit null IDs, notifications, errors, and batches for every protocol adapter.

**Blocked by:** None — can start immediately

**Status:** ready-for-agent

- [ ] Success responses contain `result` and no `error`; failures contain `error` and no `result`.
- [ ] Parse error, invalid request, method not found, invalid params, and internal error are distinct.
- [ ] Notifications are identified only by an absent `id` and never receive responses.
- [ ] Mixed and notification-only batches follow JSON-RPC 2.0 with stable request correlation.
- [ ] Parser/dispatcher fuzz and compiled-stdio regression tests pass.
