# 10 — Classify files and inventory binary metadata

**What to build:** Bounded file-kind detection and metadata extraction for authored text, SFF, SND, fonts, shaders, images, audio, and unknown assets.

**Blocked by:** 09 — Implement deterministic workspace discovery

**Status:** ready-for-agent

- [ ] Extension and bounded signature checks produce a stable file kind and MIME type.
- [ ] Text limits and binary header-read limits are enforced before allocation.
- [ ] SFF/SND and other large assets are inventoried without full reads or hashes by default.
- [ ] Unsupported/corrupt formats yield diagnostics rather than scan failure.
- [ ] Reference-corpus tests include the largest observed authored files and multi-gigabyte assets.
