#!/usr/bin/env bash
# check-action-pins.sh — Guard against a mutable third-party GitHub Actions ref.
#
# CVE-2025-30066 (March 2025): an attacker moved the v-tags of
# tj-actions/changed-files onto a poisoned commit; ~23k repositories that pinned
# by tag executed it and leaked secrets into CI logs. A tag is a mutable pointer;
# a commit SHA is not. This script fails CI on any third-party `uses:` that is
# not a full 40-hex commit SHA.
#
# Allowed unpinned forms, and why:
#   ./path                    — an action inside this repo; same commit as the caller.
#   docker://image:tag        — not a git ref; image pinning is a container concern.
#
# Vendored from gitlab.com/autops/wharf/dot-github's scripts/check-action-pins.sh,
# which additionally exempts go-kure/*/.github/workflows/x.yml@ref (a first-party
# REUSABLE WORKFLOW, exempt under GitHub's own sha_pinning_required policy in that
# org). That exemption is deliberately NOT carried over here: this repository is
# github.com/ginsys/shelly-manager (go.mod:1), unrelated to go-kure, so keeping it
# would leave a third-party namespace exempt from the exact check this script
# exists to enforce. If this repo ever gains its own first-party reusable
# workflows that need the same GitHub-policy exemption, scope a new regex to
# ginsys/shelly-manager's own path — do not reintroduce the go-kure one.
#
# Known limitation: this is a line-anchored grep, not a YAML parser. A `run: |`
# block scalar whose shell text happens to start a line with `uses:` would be
# misdetected as an actions ref. Verified absent across every real workflow in
# this org today; closing the gap unconditionally needs a YAML parser, which is
# out of scope here. If a future script legitimately needs a `run:` line shaped
# like that, indent it so it is not first-on-line, or extend this checker then.
#
# Usage: check-action-pins.sh [REPO_ROOT]
# Exits non-zero and lists every unpinned reference.

set -euo pipefail

ROOT="${1:-.}"
ROOT="$(cd "$ROOT" && pwd)"

errors=0
fail() { echo "FAIL: $*" >&2; errors=$((errors + 1)); }

checked=0
while IFS= read -r -d '' file; do
  # Strip a trailing `# comment` before matching so the `# v7` provenance
  # comment on a correct pin can never be read as part of the ref.
  while IFS= read -r ref; do
    checked=$((checked + 1))
    case "$ref" in
      ./*|docker://*) continue ;;
    esac
    if [[ ! "$ref" =~ @[0-9a-f]{40}$ ]]; then
      fail "$(basename "$file"): unpinned action ref '$ref' (pin to a 40-char commit SHA, keep the tag as a trailing comment)"
    fi
  done < <(
    grep -hoE '^[[:space:]]*(-[[:space:]]+)?uses:[[:space:]]*[^#]+' "$file" \
      | sed -E 's/^[[:space:]]*(-[[:space:]]+)?uses:[[:space:]]*//; s/[[:space:]]+$//; s/^["'"'"']//; s/["'"'"']$//' \
      || true
  )
done < <(find "$ROOT/.github" -type f \( -name '*.yml' -o -name '*.yaml' \) -print0 2>/dev/null)

if [ "$errors" -gt 0 ]; then
  echo "check-action-pins: $errors unpinned ref(s) across $checked checked" >&2
  exit 1
fi
echo "check-action-pins: OK ($checked refs checked)"
