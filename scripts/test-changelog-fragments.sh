#!/usr/bin/env bash
# test-changelog-fragments.sh — fixture tests for changelog-assemble.sh and
# check-changelog-fragment.sh (ginsys/shelly-manager#600).
#
# Every case builds a throw-away git repository, copies the two scripts into it
# and runs them there, so nothing touches this checkout. Exit 0 only if every
# case passed; a failing case prints what it expected and what it got.
set -u

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
T=$(mktemp -d "${TMPDIR:-/tmp}/changelog-fragments-test.XXXXXX") || exit 2
trap 'rm -rf "$T"' EXIT

PASS=0
FAIL=0
N=0

ok() { PASS=$((PASS + 1)); printf 'ok   %s\n' "$1"; }
bad() {
	FAIL=$((FAIL + 1))
	printf 'FAIL %s\n' "$1"
	shift
	printf '     %s\n' "$@"
}

# eq <name> <expected> <actual>
eq() {
	if [ "$2" = "$3" ]; then ok "$1"; else bad "$1" "expected: $2" "actual:   $3"; fi
}

# has <name> <needle> <haystack>
has() {
	case "$3" in *"$2"*) ok "$1" ;; *) bad "$1" "expected to contain: $2" "actual: $3" ;; esac
}

# git in a fixture, with a fixed identity and no signing.
fxgit() {
	git -c user.name=test -c user.email=test@example.invalid -c commit.gpgsign=false \
		-c core.hooksPath=/dev/null "$@"
}

# newfx <name>: a repository on branch main with the scripts, a CHANGELOG and
# changelog.d/README.md, and refs/remotes/origin/main pointing at main.
newfx() {
	N=$((N + 1))
	FX="$T/$N-$1"
	mkdir -p "$FX/scripts" "$FX/changelog.d"
	cp "$SCRIPTS/changelog-assemble.sh" "$FX/scripts/" 2>/dev/null
	cp "$SCRIPTS/check-changelog-fragment.sh" "$FX/scripts/" 2>/dev/null
	cat >"$FX/CHANGELOG.md" <<'EOF'
# Changelog

Intro line.

## [Unreleased]

### Added
- Legacy entry one.
  Second line of it. Refs owner/repo#1.

### Added
- A repeated heading, as the real file has. Refs owner/repo#2.

### Fixed
- Legacy fix. Fixes owner/repo#3.

## [0.5.4] - 2024-01-15

### Added
- Old release entry.
EOF
	printf 'Format notes.\n' >"$FX/changelog.d/README.md"
	(
		cd "$FX" || exit 1
		fxgit init -q -b main .
		fxgit add -A
		fxgit commit -q -m base
		fxgit update-ref refs/remotes/origin/main main
	) >/dev/null 2>&1
}

# frag <file> <section> <bullet text...>: write a fragment in the fixture.
frag() {
	local f=$1 section=$2
	shift 2
	printf '%s\n%s\n' "$section" "$*" >"$FX/changelog.d/$f"
}

# run <cmd...>: run in the fixture, set OUT (stdout+stderr) and RC.
run() {
	OUT=$(cd "$FX" && "$@" 2>&1)
	RC=$?
}

# nfrags: how many fragment files (every changelog.d/*.md but README.md) the fixture holds.
nfrags() {
	local n=0 f
	for f in "$FX"/changelog.d/*.md; do
		[ -e "$f" ] || continue
		[ "$(basename "$f")" = README.md ] && continue
		n=$((n + 1))
	done
	echo "$n"
}

# ---------------------------------------------------------------- assemble

newfx order
frag 12-b.md Fixed '- Fix b. Fixes owner/repo#12.'
frag 3-a.md Added '- Add a. Refs owner/repo#3.'
frag 12-a.md Fixed '- Fix a. Fixes owner/repo#12.'
frag 7-x.md Changed '- Change x. Refs owner/repo#7.'
frag 3-z.md Fixed '- Fix z. Fixes owner/repo#3.'
run bash scripts/changelog-assemble.sh
EXPECT='### Added
- Add a. Refs owner/repo#3.

### Changed
- Change x. Refs owner/repo#7.

### Fixed
- Fix z. Fixes owner/repo#3.
- Fix a. Fixes owner/repo#12.
- Fix b. Fixes owner/repo#12.'
eq "preview: sections in fixed order, files by number (sort -V)" "$EXPECT" "$OUT"
eq "preview: exit 0" 0 "$RC"
eq "preview: CHANGELOG.md untouched" "$(cd "$FX" && fxgit status --porcelain -- CHANGELOG.md)" ""
eq "preview: fragments still there" 5 "$(nfrags)"

newfx unknown-section
frag 5-a.md Fixed '- Good one.'
frag 6-b.md Feature '- Bad section.'
run bash scripts/changelog-assemble.sh --write 1.2.3
eq "unknown section: exit 2" 2 "$RC"
has "unknown section: names the file" "6-b.md" "$OUT"
eq "unknown section: nothing written" "" "$(cd "$FX" && fxgit status --porcelain -- CHANGELOG.md)"
eq "unknown section: no fragment removed" 2 "$(nfrags)"

for bad in 'Fix-thing.md' 'abc.md' '12.md' '12-Foo.md' '12-a_b.md'; do
	newfx badname
	printf 'Fixed\n- text.\n' >"$FX/changelog.d/$bad"
	run bash scripts/changelog-assemble.sh
	eq "bad file name $bad: exit 2" 2 "$RC"
done

newfx empty-body
printf 'Fixed\n' >"$FX/changelog.d/9-empty.md"
run bash scripts/changelog-assemble.sh
eq "empty body: exit 2" 2 "$RC"

newfx not-a-bullet
printf 'Fixed\nplain text, no bullet\n' >"$FX/changelog.d/9-plain.md"
run bash scripts/changelog-assemble.sh
eq "body that is not a bullet: exit 2" 2 "$RC"

newfx two-bullets
printf 'Fixed\n- one.\n- two.\n' >"$FX/changelog.d/9-two.md"
run bash scripts/changelog-assemble.sh
eq "two bullets in one file: exit 2" 2 "$RC"

newfx star-bullet
printf 'Fixed\n- one.\n* two.\n' >"$FX/changelog.d/9-star.md"
run bash scripts/changelog-assemble.sh
eq "a second bullet written with * : exit 2" 2 "$RC"

newfx heading-line
printf 'Fixed\n- one.\n## [9.9.9] - 2030-01-01\n' >"$FX/changelog.d/9-heading.md"
run bash scripts/changelog-assemble.sh
eq "an unindented heading line in the body: exit 2" 2 "$RC"
has "heading line: the error names the file" "9-heading.md" "$OUT"

newfx continuation
printf 'Fixed\n- one,\n  continued.\n\n  A second paragraph. Fixes owner/repo#9.\n' >"$FX/changelog.d/9-cont.md"
run bash scripts/changelog-assemble.sh
eq "indented continuation lines and an indented paragraph pass: exit 0" 0 "$RC"

newfx none
run bash scripts/changelog-assemble.sh
eq "no fragments, preview: exit 3" 3 "$RC"
run bash scripts/changelog-assemble.sh --write 1.2.3
eq "no fragments, write: exit 3" 3 "$RC"

newfx write
frag 3-a.md Added '- Add a. Refs owner/repo#3.'
frag 12-a.md Fixed '- Fix a. Fixes owner/repo#12.'
(cd "$FX" && fxgit add -A && fxgit commit -q -m frags)
LEGACY=$(sed -n '/^## \[Unreleased\]$/,/^## \[0.5.4\]/p' "$FX/CHANGELOG.md" | sed '1d;$d')
run env CHANGELOG_DATE=2026-01-02 bash scripts/changelog-assemble.sh --write 1.2.3
eq "write: exit 0" 0 "$RC"
EXPECT_FILE='# Changelog

Intro line.

## [Unreleased]

## [1.2.3] - 2026-01-02
'"$LEGACY"'

### Added
- Add a. Refs owner/repo#3.

### Fixed
- Fix a. Fixes owner/repo#12.

## [0.5.4] - 2024-01-15

### Added
- Old release entry.'
eq "write: legacy lines kept byte for byte and first, fragments after, one version heading" \
	"$EXPECT_FILE" "$(cat "$FX/CHANGELOG.md")"
eq "write: fragments removed from the tree" 0 "$(nfrags)"
eq "write: fragments removed from the index" "changelog.d/README.md" \
	"$(cd "$FX" && fxgit ls-files changelog.d)"
eq "write: README.md kept" "Format notes." "$(cat "$FX/changelog.d/README.md")"
BEFORE=$(cat "$FX/CHANGELOG.md")
run env CHANGELOG_DATE=2026-01-02 bash scripts/changelog-assemble.sh --write 1.2.3
eq "second write: exit 3" 3 "$RC"
eq "second write: CHANGELOG.md unchanged" "$BEFORE" "$(cat "$FX/CHANGELOG.md")"

# The release is written beside CHANGELOG.md and renamed over it, so a failed
# write leaves the original whole: with the directory unwritable no temporary
# file can be made, and nothing may change.
newfx write-atomic
frag 3-a.md Added '- Add a.'
(cd "$FX" && fxgit add -A && fxgit commit -q -m frags)
BEFORE=$(cat "$FX/CHANGELOG.md")
chmod a-w "$FX"
run env CHANGELOG_DATE=2026-01-02 bash scripts/changelog-assemble.sh --write 1.2.3
chmod u+w "$FX"
if [ "$RC" -ne 0 ]; then ok "unwritable directory: the write is refused"; else bad "unwritable directory: the write is refused" "exit was 0"; fi
eq "unwritable directory: CHANGELOG.md unchanged" "$BEFORE" "$(cat "$FX/CHANGELOG.md")"
eq "unwritable directory: the fragment is still there" 1 "$(nfrags)"

newfx write-mode
frag 3-a.md Added '- Add a.'
chmod 640 "$FX/CHANGELOG.md"
run env CHANGELOG_DATE=2026-01-02 bash scripts/changelog-assemble.sh --write 1.2.3
eq "write: CHANGELOG.md keeps its mode" 1 "$(find "$FX/CHANGELOG.md" -perm 640 | wc -l | tr -d ' ')"

newfx bad-version
frag 3-a.md Added '- Add a.'
run bash scripts/changelog-assemble.sh --write v1
eq "invalid version: exit 2" 2 "$RC"
run bash scripts/changelog-assemble.sh --write
eq "missing version: exit 2" 2 "$RC"

# ------------------------------------------------------------------- check

# prbranch <name>: branch off main in the fixture and check it out.
prbranch() { (cd "$FX" && fxgit checkout -q -b "$1" main); }
commit_all() { (cd "$FX" && fxgit add -A && fxgit commit -q -m "$1"); }
check() { run env "$@" bash scripts/check-changelog-fragment.sh; }

newfx check-pass
prbranch fix/1-a
frag 1-a.md Fixed '- Fix. Fixes owner/repo#1.'
commit_all c
check HEAD_REF_NAME=fix/1-a
eq "check: a PR with a fragment passes (exit 0)" 0 "$RC"
has "check: PASS line" "PASS" "$OUT"

newfx check-nofrag
prbranch fix/2-a
printf 'code\n' >"$FX/code.txt"
commit_all c
check HEAD_REF_NAME=fix/2-a
eq "check: no fragment fails (exit 1)" 1 "$RC"
has "check: FAIL line says what to add" "changelog.d/" "$OUT"

newfx check-readme-only
prbranch docs/3-a
printf 'more notes\n' >>"$FX/changelog.d/README.md"
commit_all c
check HEAD_REF_NAME=docs/3-a
eq "check: editing README.md is not a fragment (exit 1)" 1 "$RC"

newfx check-edit-unreleased
prbranch fix/4-a
frag 4-a.md Fixed '- Fix. Fixes owner/repo#4.'
(cd "$FX" && sed -i 's/^- Legacy fix. Fixes owner\/repo#3.$/&\n- Added the old way. Fixes owner\/repo#4./' CHANGELOG.md)
commit_all c
check HEAD_REF_NAME=fix/4-a
eq "check: a CHANGELOG.md edit fails even with a fragment (exit 1)" 1 "$RC"
has "check: FAIL names CHANGELOG.md" "CHANGELOG.md" "$OUT"

# The workflow passes the exact-membership answer (contains() over the label
# array), never a joined label list, so a label such as "docs,no-changelog"
# cannot pass for the label no-changelog.
newfx check-label
prbranch ci/5-a
printf 'ci\n' >"$FX/ci.txt"
commit_all c
check HEAD_REF_NAME=ci/5-a PR_NO_CHANGELOG=true
eq "check: PR_NO_CHANGELOG=true skips (exit 0)" 0 "$RC"
has "check: SKIP line names the reason" "SKIP" "$OUT"
check HEAD_REF_NAME=ci/5-a PR_NO_CHANGELOG=false
eq "check: PR_NO_CHANGELOG=false does not skip (exit 1)" 1 "$RC"
check HEAD_REF_NAME=ci/5-a PR_NO_CHANGELOG=
eq "check: an empty PR_NO_CHANGELOG does not skip (exit 1)" 1 "$RC"
check HEAD_REF_NAME=ci/5-a PR_NO_CHANGELOG=yes
eq "check: only the word true skips (exit 1)" 1 "$RC"

newfx check-bots
prbranch dependabot/go_modules/x
printf 'dep\n' >"$FX/dep.txt"
commit_all c
check HEAD_REF_NAME=dependabot/go_modules/x
eq "check: dependabot ref skips (exit 0)" 0 "$RC"
check HEAD_REF_NAME=renovate/x
eq "check: renovate ref skips (exit 0)" 0 "$RC"
check HEAD_REF_NAME=fix/dependabot/x
eq "check: dependabot only as a prefix (exit 1)" 1 "$RC"

newfx check-badfrag
prbranch fix/6-a
frag 6-a.md Feature '- Bad section.'
commit_all c
check HEAD_REF_NAME=fix/6-a
eq "check: an invalid fragment fails (exit 1)" 1 "$RC"
has "check: FAIL names the fragment" "6-a.md" "$OUT"

newfx check-subdir
prbranch fix/7-a
mkdir -p "$FX/changelog.d/sub"
printf 'Fixed\n- text.\n' >"$FX/changelog.d/sub/7-a.md"
commit_all c
check HEAD_REF_NAME=fix/7-a
eq "check: a fragment in a subdirectory is not one (exit 1)" 1 "$RC"

newfx check-nobase
(cd "$FX" && fxgit update-ref -d refs/remotes/origin/main)
prbranch fix/8-a
frag 8-a.md Fixed '- Fix.'
commit_all c
check HEAD_REF_NAME=fix/8-a
eq "check: a missing base ref is an error, never an empty diff (exit 2)" 2 "$RC"
has "check: ERROR line names the ref" "ERROR origin/main not found" "$OUT"

newfx check-base-ref
(cd "$FX" && fxgit update-ref refs/remotes/origin/develop main)
prbranch fix/9-a
frag 9-a.md Fixed '- Fix.'
commit_all c
check BASE_REF=develop HEAD_REF_NAME=fix/9-a
eq "check: BASE_REF selects another base (exit 0)" 0 "$RC"

newfx check-release
frag 3-a.md Added '- Add a.'
commit_all frags
(cd "$FX" && fxgit update-ref refs/remotes/origin/main main)
prbranch chore/release
(cd "$FX" && CHANGELOG_DATE=2026-01-02 bash scripts/changelog-assemble.sh --write 1.2.3 >/dev/null 2>&1)
commit_all release
check HEAD_REF_NAME=chore/release
eq "check: the assemble commit passes (exit 0)" 0 "$RC"
has "check: it says it is a release commit" "release commit" "$OUT"

# A release commit must show each deleted fragment's entry in CHANGELOG.md, under
# a full version heading. A heading alone, or an incomplete one, is no release.
newfx check-release-discards
frag 3-a.md Added '- Add a. Refs owner/repo#3.'
commit_all frags
(cd "$FX" && fxgit update-ref refs/remotes/origin/main main)
prbranch chore/release2
(cd "$FX" && git rm -q changelog.d/3-a.md && sed -i 's/^## \[Unreleased\]$/&\n\n## [1.2.3] - 2026-01-02/' CHANGELOG.md)
commit_all discards
check HEAD_REF_NAME=chore/release2
eq "check: a full heading but a discarded entry is not a release (exit 1)" 1 "$RC"

newfx check-release-short-heading
frag 3-a.md Added '- Add a. Refs owner/repo#3.'
commit_all frags
(cd "$FX" && fxgit update-ref refs/remotes/origin/main main)
prbranch chore/release3
(cd "$FX" && git rm -q changelog.d/3-a.md && sed -i 's/^## \[Unreleased\]$/&\n\n## [1\n- Add a. Refs owner\/repo#3./' CHANGELOG.md)
commit_all short
check HEAD_REF_NAME=chore/release3
eq "check: an incomplete heading is not a release (exit 1)" 1 "$RC"

newfx check-release-invalid-deleted
printf 'Feature\n- Odd. Refs owner/repo#3.\n' >"$FX/changelog.d/3-a.md"
commit_all frags
(cd "$FX" && fxgit update-ref refs/remotes/origin/main main)
prbranch chore/release4
(cd "$FX" && git rm -q changelog.d/3-a.md && sed -i 's/^## \[Unreleased\]$/&\n\n## [1.2.3] - 2026-01-02\n- Odd. Refs owner\/repo#3./' CHANGELOG.md)
commit_all odd
check HEAD_REF_NAME=chore/release4
eq "check: a deleted fragment that was invalid is not a release (exit 1)" 1 "$RC"

newfx check-fake-release
frag 3-a.md Added '- Add a.'
commit_all frags
(cd "$FX" && fxgit update-ref refs/remotes/origin/main main)
prbranch fix/10-a
(cd "$FX" && git rm -q changelog.d/3-a.md)
commit_all removal
check HEAD_REF_NAME=fix/10-a
eq "check: deleting a fragment without a version heading is not a release (exit 1)" 1 "$RC"

# ------------------------------------------------- the acceptance: two PRs

newfx merge-either-order
(cd "$FX" && fxgit checkout -q -b a main)
frag 1-a.md Fixed '- Fix a.'
commit_all a
(cd "$FX" && fxgit checkout -q -b b main)
frag 2-b.md Fixed '- Fix b.'
commit_all b
(cd "$FX" && fxgit checkout -q -b ab a && fxgit merge -q --no-edit b >/dev/null 2>&1)
AB=$?
(cd "$FX" && fxgit checkout -q -b ba b && fxgit merge -q --no-edit a >/dev/null 2>&1)
BA=$?
eq "two branches that each add a fragment merge, a then b (exit 0)" 0 "$AB"
eq "two branches that each add a fragment merge, b then a (exit 0)" 0 "$BA"
eq "both orders give the same tree" \
	"$(cd "$FX" && git rev-parse 'ab^{tree}')" "$(cd "$FX" && git rev-parse 'ba^{tree}')"

newfx merge-control
(cd "$FX" && fxgit checkout -q -b a main && sed -i 's/^- Legacy fix. Fixes owner\/repo#3.$/&\n- Entry a./' CHANGELOG.md)
commit_all a
(cd "$FX" && fxgit checkout -q -b b main && sed -i 's/^- Legacy fix. Fixes owner\/repo#3.$/&\n- Entry b./' CHANGELOG.md)
commit_all b
(cd "$FX" && fxgit merge -q --no-edit a >/dev/null 2>&1)
CTRL=$?
if [ "$CTRL" -ne 0 ]; then ok "control: two PRs editing the same CHANGELOG.md place do conflict"; else bad "control: two PRs editing the same CHANGELOG.md place do conflict" "merge exit was 0"; fi

printf '\n%s passed, %s failed\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
