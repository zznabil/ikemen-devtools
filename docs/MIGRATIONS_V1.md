# v1 migration

Configuration is `.ikm/config.json`, version `0.1`; unknown fields are rejected. Move old profile and budget settings into `profile`, `cache`, and `budgets`, then run `ikm config show --effective --json` and record `digest`.

Delete and rebuild indexes when the schema or identity contract changes; SQLite is disposable. Consumers must validate `schemaVersion` and `operation` in every envelope. Deprecated operation aliases are not guaranteed; discover canonical names at startup. MCP tools/resources are generated from the capability registry. No GUI, daemon, cloud service, bundled model, or autonomous runtime is required.
