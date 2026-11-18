#!/usr/bin/env bash
# check-changelog-fragment.sh — a PR records its release note as a fragment.
#
# Compares HEAD with origin/<base> (three-dot, so only the PR's own changes) and
# prints one verdict:
#   SKIP  the head branch is a bot's (dependabot/, renovate/) or the PR carries
#         the label no-changelog (CI, tests and docs-only changes)
#   PASS  the PR adds a valid changelog.d/<issue>-<slug>.md and adds no line to
#         CHANGELOG.md; or it is a release commit made by changelog-assemble.sh
#         (it deletes fragments, adds a full "## [X.Y.Z] - date" heading, and
#         every deleted entry appears among the added CHANGELOG.md lines)
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
VERSION_HEADING='^## \[[0-9]+\.[0-9]+\.[0-9]+([-.][0-9A-Za-z.]+)?\] - [0-9]{4}-[0-9]{2}-[0-9]{2}$'

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

tmp=$(mktemp -d "${TMPDIR:-/tmp}/check-changelog-fragment.XXXXXX")
trap 'rm -rf "$tmp"' EXIT

added=()
deleted=()
subdir=()
while IFS=$'\t' read -r status path; do
	case "$path" in
	changelog.d/README.md) continue ;;
	changelog.d/*/*.md) [ "$status" = A ] && subdir+=("$path") ;;
	changelog.d/*.md)
		case "$status" in
		A) added+=("$path") ;;
		D) deleted+=("$path") ;;
		esac
		;;
	esac
done < <(git diff --name-status --no-renames "$base...HEAD")

# The lines this PR adds to CHANGELOG.md, without the diff markers.
git diff --no-renames -U0 "$base...HEAD" -- CHANGELOG.md |
	awk '/^\+\+\+ /{next} /^\+/{print substr($0, 2)}' >"$tmp/added"
changelog_lines=$(wc -l <"$tmp/added" | tr -d ' ')

# A release commit: fragments deleted and a full version heading added. The
# exception is granted only when every deleted fragment is valid and its entry
# is among the added lines, so deleting entries cannot pass for a release.
if [ "${#deleted[@]}" -gt 0 ] && grep -Eq "$VERSION_HEADING" "$tmp/added"; then
	mb=$(git merge-base "$base" HEAD)
	ok=1
	for p in "${deleted[@]}"; do
		git show "$mb:$p" >"$tmp/$(basename "$p")"
		if ! msg=$("$here/changelog-assemble.sh" --validate "$tmp/$(basename "$p")" 2>&1); then
			echo "FAIL release commit: the deleted fragment $p was not valid: $msg"
			ok=0
			continue
		fi
		while IFS= read -r line; do
			[ -n "$line" ] || continue
			if ! grep -Fxq -- "$line" "$tmp/added"; then
				echo "FAIL release commit: the entry of $p is not among the lines added to CHANGELOG.md: $line"
				ok=0
				break
			fi
		done < <(tail -n +2 "$tmp/$(basename "$p")" | awk 'NF')
	done
	[ "$ok" -eq 1 ] || exit 1
	echo "PASS release commit: ${#deleted[@]} fragment(s) assembled into CHANGELOG.md"
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
