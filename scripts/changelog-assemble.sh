#!/usr/bin/env bash
# changelog-assemble.sh — turn changelog fragments into CHANGELOG.md text.
#
# Each change adds one file, changelog.d/<n>-<slug>.md, so two PRs never edit the
# same CHANGELOG.md lines (see changelog.d/README.md for the format).
#
#   changelog-assemble.sh                   print the assembled sections, change nothing
#   changelog-assemble.sh --validate FILE…  check fragment files, change nothing
#   changelog-assemble.sh --write VERSION   cut a release: keep every line under
#                                           "## [Unreleased]", add the fragment sections
#                                           after them, give the block the heading
#                                           "## [VERSION] - <date>", leave an empty
#                                           "## [Unreleased]" above it and delete the
#                                           fragments (git rm where tracked)
#
# Exit codes: 0 done, 2 a fragment, argument or CHANGELOG.md is invalid (nothing is
# written), 3 there are no fragments. Run from anywhere; it works on its own checkout.
# CHANGELOG_DATE=YYYY-MM-DD overrides today's date (used by the tests).
set -euo pipefail

SECTIONS="Added Changed Deprecated Removed Fixed Security"
NAME_RE='^[0-9]+-[a-z0-9][a-z0-9-]*\.md$'
VERSION_RE='^[0-9]+\.[0-9]+\.[0-9]+([-.][0-9A-Za-z.]+)?$'
DATE_RE='^[0-9]{4}-[0-9]{2}-[0-9]{2}$'

ROOT=$(cd "$(dirname "$0")/.." && pwd)
FRAG_DIR=changelog.d
CHANGELOG=CHANGELOG.md

usage() {
	echo "usage: changelog-assemble.sh [--validate FILE… | --write VERSION]" >&2
	exit 2
}

is_section() {
	local s
	for s in $SECTIONS; do
		[ "$s" = "$1" ] && return 0
	done
	return 1
}

# validate_file <path>: print one ERROR line and return 1 when the fragment is invalid.
validate_file() {
	local path=$1 base section verdict
	base=$(basename "$path")
	if ! [[ $base =~ $NAME_RE ]]; then
		echo "ERROR $path: the name must be <issue>-<slug>.md: digits, a dash, then lowercase letters, digits and dashes" >&2
		return 1
	fi
	section=
	IFS= read -r section <"$path" || true
	if ! is_section "$section"; then
		echo "ERROR $path: line 1 must be one of: $SECTIONS (got '$section')" >&2
		return 1
	fi
	verdict=$(awk '
		NR == 1 { next }
		!seen && $0 ~ /^[[:space:]]*$/ { next }
		{
			if (!seen) { seen = 1; if ($0 !~ /^- /) bad = "the entry must start with \"- \"" }
			else if ($0 ~ /^- /) multi = 1
			else if ($0 !~ /^[[:space:]]*$/ && $0 !~ /^  /) cont = 1
		}
		END {
			if (!seen) print "the entry is empty"
			else if (bad != "") print bad
			else if (multi) print "one entry per file: a second line starts with \"- \""
			else if (cont) print "every line after the first must be blank or indented two spaces"
		}' "$path")
	if [ -n "$verdict" ]; then
		echo "ERROR $path: $verdict" >&2
		return 1
	fi
	return 0
}

# body <path>: the entry, without the section line and without blank lines around it.
body() {
	awk '
		NR == 1 { next }
		{ lines[++n] = $0 }
		END {
			s = 1
			while (s <= n && lines[s] ~ /^[[:space:]]*$/) s++
			e = n
			while (e >= s && lines[e] ~ /^[[:space:]]*$/) e--
			for (i = s; i <= e; i++) print lines[i]
		}' "$1"
}

mode=preview
version=
case "${1-}" in
--validate)
	shift
	[ "$#" -gt 0 ] || usage
	rc=0
	for f in "$@"; do
		validate_file "$f" || rc=2
	done
	exit "$rc"
	;;
--write)
	[ "$#" -eq 2 ] || usage
	version=$2
	[[ $version =~ $VERSION_RE ]] || {
		echo "ERROR the version '$version' must look like 1.2.3 or 1.2.3-rc.1" >&2
		exit 2
	}
	mode="write"
	;;
"")
	;;
*)
	usage
	;;
esac
[ "$#" -le 2 ] || usage

cd "$ROOT"

names=()
while IFS= read -r n; do
	[ -n "$n" ] && names+=("$n")
done < <(
	for f in "$FRAG_DIR"/*.md; do
		[ -e "$f" ] || continue
		[ "$(basename "$f")" = README.md ] && continue
		basename "$f"
	done | sort -V
)

if [ "${#names[@]}" -eq 0 ]; then
	echo "no fragments in $FRAG_DIR/" >&2
	exit 3
fi

invalid=0
for n in "${names[@]}"; do
	validate_file "$FRAG_DIR/$n" || invalid=1
done
[ "$invalid" -eq 0 ] || exit 2

tmp=$(mktemp -d "${TMPDIR:-/tmp}/changelog-assemble.XXXXXX")
next=
trap 'rm -rf "$tmp"; [ -z "$next" ] || rm -f "$next"' EXIT

gen=$tmp/generated
: >"$gen"
first=1
for s in $SECTIONS; do
	have=0
	for n in "${names[@]}"; do
		sec=
		IFS= read -r sec <"$FRAG_DIR/$n" || true
		[ "$sec" = "$s" ] || continue
		if [ "$have" -eq 0 ]; then
			[ "$first" -eq 1 ] || echo >>"$gen"
			echo "### $s" >>"$gen"
			have=1
			first=0
		fi
		body "$FRAG_DIR/$n" >>"$gen"
	done
done

if [ "$mode" = preview ]; then
	cat "$gen"
	exit 0
fi

date=${CHANGELOG_DATE:-$(date +%F)}
[[ $date =~ $DATE_RE ]] || {
	echo "ERROR CHANGELOG_DATE '$date' must be YYYY-MM-DD" >&2
	exit 2
}
if ! grep -qx '## \[Unreleased\]' "$CHANGELOG"; then
	echo "ERROR $CHANGELOG has no '## [Unreleased]' line" >&2
	exit 2
fi

awk -v ver="$version" -v date="$date" -v gen="$gen" '
	function flush(tail,   i, line) {
		for (i = 1; i <= n; i++) print buf[i]
		if (n == 0 || buf[n] != "") print ""
		while ((getline line < gen) > 0) print line
		close(gen)
		if (tail) print ""
	}
	state == 0 {
		print
		if ($0 == "## [Unreleased]") { print ""; print "## [" ver "] - " date; state = 1; n = 0 }
		next
	}
	state == 1 {
		if ($0 ~ /^## \[/) { flush(1); state = 2; print; next }
		buf[++n] = $0
		next
	}
	{ print }
	END { if (state == 1) flush(0) }
' "$CHANGELOG" >"$tmp/new"

# Write beside CHANGELOG.md and rename over it: a failed or interrupted write
# leaves the original whole, and the fragments go only after the rename.
next=$(mktemp "$ROOT/$CHANGELOG.XXXXXX")
cp -p "$CHANGELOG" "$next"
cat "$tmp/new" >"$next"
mv -f "$next" "$CHANGELOG"
next=
for n in "${names[@]}"; do
	if git ls-files --error-unmatch -- "$FRAG_DIR/$n" >/dev/null 2>&1; then
		git rm -q -f -- "$FRAG_DIR/$n"
	else
		rm -f -- "$FRAG_DIR/$n"
	fi
done
echo "wrote $CHANGELOG: ## [$version] - $date from ${#names[@]} fragment(s); fragments removed"
