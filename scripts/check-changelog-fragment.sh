#!/usr/bin/env bash
# check-changelog-fragment.sh — a PR records its release note as a fragment.
#
# Compares HEAD with origin/<base> (three-dot, so only the PR's own changes) and
# prints one verdict:
#   SKIP  the head branch is a bot's (dependabot/, renovate/) or the PR carries
#         the label no-changelog (CI, tests and docs-only changes, and the release
#         PR made by changelog-assemble.sh --write)
#   PASS  the PR adds a valid changelog.d/<issue>-<slug>.md and adds no line to
#         CHANGELOG.md
#   FAIL  anything else, with what to do
#   ERROR the base ref is missing (a shallow checkout): never read as "no fragment"
#
# Inputs (environment): BASE_REF (default main), HEAD_REF_NAME (the PR's head
# branch), PR_NO_CHANGELOG (the word true when the PR has the label no-changelog;
# the workflow sets it with contains() over the label list, an exact match).
# Exit codes: 0 PASS or SKIP, 1 FAIL, 2 ERROR. See changelog.d/README.md.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
cd "$(git rev-parse --show-toplevel)"

base="origin/${BASE_REF:-main}"
head_ref=${HEAD_REF_NAME-}
no_changelog=${PR_NO_CHANGELOG-}

if ! git rev-parse --verify -q "$base^{commit}" >/dev/null; then
	echo "ERROR $base not found: fetch it first (actions/checkout needs fetch-depth: 0)"
	exit 2
fi

case "$head_ref" in
dependabot/* | renovate/*)
	echo "SKIP bot branch $head_ref"
	exit 0
	;;
esac

if [ "$no_changelog" = true ]; then
	echo "SKIP label no-changelog"
	exit 0
fi

added=()
subdir=()
while IFS=$'\t' read -r status path; do
	case "$path" in
	changelog.d/README.md) continue ;;
	changelog.d/*/*.md) [ "$status" = A ] && subdir+=("$path") ;;
	changelog.d/*.md) [ "$status" = A ] && added+=("$path") ;;
	esac
done < <(git diff --name-status --no-renames "$base...HEAD")

changelog_lines=$(git diff --no-renames -U0 "$base...HEAD" -- CHANGELOG.md |
	awk '/^\+\+\+ /{next} /^\+/{n++} END{print n+0}')

fail=0
if [ "$changelog_lines" -gt 0 ]; then
	echo "FAIL CHANGELOG.md has $changelog_lines added line(s): put the entry in changelog.d/<issue>-<slug>.md instead (a release PR carries the label no-changelog)"
	fail=1
fi
for p in ${subdir[@]+"${subdir[@]}"}; do
	echo "FAIL $p is in a subdirectory: fragments sit directly in changelog.d/"
	fail=1
done
if [ "${#added[@]}" -eq 0 ]; then
	echo "FAIL no fragment added: add changelog.d/<issue>-<slug>.md, or label the PR no-changelog when it needs no entry"
	fail=1
fi
for p in ${added[@]+"${added[@]}"}; do
	if ! msg=$("$here/changelog-assemble.sh" --validate "$p" 2>&1); then
		echo "FAIL $msg"
		fail=1
	fi
done

if [ "$fail" -ne 0 ]; then
	exit 1
fi
echo "PASS ${#added[@]} fragment(s) added"
