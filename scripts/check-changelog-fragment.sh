#!/usr/bin/env bash
# check-changelog-fragment.sh — a PR records its release note as a fragment.
#
# Compares HEAD with origin/<base> (three-dot, so only the PR's own changes) and
# prints one verdict:
#   SKIP  the head branch is a bot's (dependabot/, renovate/) or the PR carries
#         the label no-changelog (CI, tests and docs-only changes)
#   PASS  the PR adds a valid changelog.d/<issue>-<slug>.md and adds no line to
#         CHANGELOG.md; or it is a release commit made by changelog-assemble.sh
#         (it deletes fragments and adds a version heading)
#   FAIL  anything else, with what to do
#   ERROR the base ref is missing (a shallow checkout): never read as "no fragment"
#
# Inputs (environment): BASE_REF (default main), HEAD_REF_NAME (the PR's head
# branch), PR_LABELS (comma- or newline-separated label names).
# Exit codes: 0 PASS or SKIP, 1 FAIL, 2 ERROR. See changelog.d/README.md.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
cd "$(git rev-parse --show-toplevel)"

base="origin/${BASE_REF:-main}"
head_ref=${HEAD_REF_NAME-}
labels=${PR_LABELS-}

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

while IFS= read -r label; do
	label=${label#"${label%%[![:space:]]*}"}
	label=${label%"${label##*[![:space:]]}"}
	if [ "$label" = no-changelog ]; then
		echo "SKIP label no-changelog"
		exit 0
	fi
done < <(printf '%s\n' "$labels" | tr ',' '\n')

added=()
deleted=0
subdir=()
while IFS=$'\t' read -r status path; do
	case "$path" in
	changelog.d/README.md) continue ;;
	changelog.d/*/*.md) [ "$status" = A ] && subdir+=("$path") ;;
	changelog.d/*.md)
		case "$status" in
		A) added+=("$path") ;;
		D) deleted=$((deleted + 1)) ;;
		esac
		;;
	esac
done < <(git diff --name-status --no-renames "$base...HEAD")

changelog_lines=$(git diff --no-renames -U0 "$base...HEAD" -- CHANGELOG.md |
	awk '/^\+\+\+ /{next} /^\+/{n++} END{print n+0}')
version_heading=$(git diff --no-renames -U0 "$base...HEAD" -- CHANGELOG.md |
	awk '/^\+## \[[0-9]/{n++} END{print n+0}')

if [ "$deleted" -gt 0 ] && [ "$version_heading" -gt 0 ]; then
	echo "PASS release commit: $deleted fragment(s) assembled into CHANGELOG.md"
	exit 0
fi

fail=0
if [ "$changelog_lines" -gt 0 ]; then
	echo "FAIL CHANGELOG.md has $changelog_lines added line(s): put the entry in changelog.d/<issue>-<slug>.md instead (a release is cut by scripts/changelog-assemble.sh)"
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
