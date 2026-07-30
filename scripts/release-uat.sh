#!/usr/bin/env bash
set -euo pipefail
binary=${1:?binary path required}
root=${2:?workspace root required}
"$binary" version --json >/tmp/ikm-version.json
"$binary" doctor --json --root "$root" >/tmp/ikm-doctor.json || status=$?
status=${status:-0}
if [ "$status" -gt 1 ]; then echo "doctor failed with internal/input status $status" >&2; exit "$status"; fi
"$binary" workspace --root "$root" --json scan >/tmp/ikm-scan.json
"$binary" capabilities --json >/tmp/ikm-capabilities.json
printf '%s\n' /tmp/ikm-version.json /tmp/ikm-doctor.json /tmp/ikm-scan.json /tmp/ikm-capabilities.json
