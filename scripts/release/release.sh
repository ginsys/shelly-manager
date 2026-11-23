#!/usr/bin/env bash
# release.sh — the release script of the Release workflow (.github/workflows/release.yml).
# Adapted from go-kure/.github scripts/release/release.sh at f6118283b02f18fb8e2d26b4b6d8f06b6522fd85,
# without release branches, the forbidden-terms check and git-cliff: the changelog comes from the
# fragments, through scripts/changelog-assemble.sh. Design: docs/development/RELEASE_WORKFLOW_DESIGN.md
# section 5. How to run it: docs/development/RELEASING.md.
#
# One action per run (RELEASE_ACTION), on main, where VERSION is vX.Y.Z-alpha|beta|rc.N:
#   release                  tag VERSION as it stands; VERSION -> the next number
#   release-as-beta|rc       move to that stage (.0) and tag it; a VERSION already at that stage is
#                            tagged as it stands, like `release`
#   release-as-stable        tag vX.Y.Z; VERSION -> vX.Y.(Z+1)-alpha.0
#   start-next-minor|major   no tag; VERSION -> the next line's alpha.0
#   skip-prerelease-number   no tag; VERSION -> the next number (recovery)
#
# A tagging action assembles the changelog fragments into CHANGELOG.md in the release commit; exit 3
# when none is left (empty notes). The VERSION-only actions assemble nothing and need none.
# Going back a stage (rc -> beta) is refused. A real run is refused off main; a dry run previews from
# any branch and changes nothing. Every push is one atomic `git push`: main and its new tag land
# together or not at all.
#
# Environment:
#   RELEASE_ACTION         one of the actions above (default: release)
#   RELEASE_BRANCH         the branch being released (default: the checked-out one)
#   RELEASE_REF_TYPE       `branch` or `tag`, the workflow's github.ref_type (default: branch)
#   DRY_RUN                1 = preview only (default: 0)
#   CI                     `true` = set the bot identity, require HEAD to be main's tip on origin, push
#   CHANGELOG_DATE         YYYY-MM-DD for the changelog heading (default: today; the tests set it)
#
# Usage:
#   release.sh            run RELEASE_ACTION
#   release.sh --guard    check the branch and the action only
# Exit: 0 done, 1 refused, 3 a tagging action with no changelog fragment left.

set -euo pipefail
# Keep set -e inside $(...) too; bash turns it off there by default.
shopt -s inherit_errexit

VERSION_FILE=VERSION
CHANGELOG_FILE=CHANGELOG.md
REMOTE=origin
GUIDE="docs/development/RELEASING.md"
ASSEMBLE="$(dirname "${BASH_SOURCE[0]}")/../changelog-assemble.sh"
ACTIONS="release release-as-beta release-as-rc release-as-stable start-next-minor start-next-major skip-prerelease-number"

DRY_RUN="${DRY_RUN:-0}"
RELEASE_ACTION="${RELEASE_ACTION:-release}"
RELEASE_BRANCH="${RELEASE_BRANCH:-}"
RELEASE_REF_TYPE="${RELEASE_REF_TYPE:-branch}"

log_info() { printf 'INFO: %s\n' "$1"; }
log_ok()   { printf 'OK: %s\n' "$1"; }
log_warn() { printf 'WARN: %s\n' "$1"; }
# die <message> [exit code]. Both lines go to stderr: die also runs inside $(...), which would
# capture a stdout annotation. The runner reads workflow commands from both streams.
die() {
    printf 'ERROR: %s\n' "$1" >&2
    if [ "${GITHUB_ACTIONS:-}" = true ]; then
        printf '::error::%s See %s\n' "$1" "$GUIDE" >&2
    fi
    exit "${2:-1}"
}

# ── Versions ──────────────────────────────────────────────────────────────

NUM='(0|[1-9][0-9]*)'
VERSION_RE="^v${NUM}\.${NUM}\.${NUM}(-(alpha|beta|rc)\.${NUM})?\$"

# parse_version <v>: sets V_MAJOR V_MINOR V_PATCH, and V_TYPE V_NUM (empty for a stable version).
parse_version() {
    [[ "$1" =~ $VERSION_RE ]] \
        || die "Invalid version '$1': expected vX.Y.Z or vX.Y.Z-alpha|beta|rc.N"
    V_MAJOR=${BASH_REMATCH[1]}
    V_MINOR=${BASH_REMATCH[2]}
    V_PATCH=${BASH_REMATCH[3]}
    V_TYPE=${BASH_REMATCH[5]}
    V_NUM=${BASH_REMATCH[6]}
}

stage_order() {
    case "$1" in
        alpha) echo 0 ;;
        beta)  echo 1 ;;
        rc)    echo 2 ;;
    esac
}

read_version() {
    [ -f "$VERSION_FILE" ] || die "No $VERSION_FILE file at the repository root"
    tr -d '[:space:]' < "$VERSION_FILE"
}

# ── Checks ────────────────────────────────────────────────────────────────

guard() {
    case " $ACTIONS " in
        *" $RELEASE_ACTION "*) ;;
        *) die "Unknown action '$RELEASE_ACTION'. Pick one of: $ACTIONS." ;;
    esac
    case "$DRY_RUN" in
        0|1) ;;
        *) die "DRY_RUN must be 0 or 1, got '$DRY_RUN'." ;;
    esac
    [ "$RELEASE_REF_TYPE" = branch ] \
        || die "Release runs from a branch, not from the $RELEASE_REF_TYPE '$RELEASE_BRANCH'. In 'Use workflow from', pick main."
    [ -n "$RELEASE_BRANCH" ] || die "No branch to release: RELEASE_BRANCH is empty."
    if [ "$RELEASE_BRANCH" != main ]; then
        [ "$DRY_RUN" = 1 ] \
            || die "A release runs only from main, not from '$RELEASE_BRANCH'. Tick 'Dry run' to preview from here."
        log_warn "Dry run from '$RELEASE_BRANCH': a real run is refused on this branch."
    fi
}

validate_git_state() {
    if ! git diff --quiet || ! git diff --cached --quiet; then
        die "The working tree has uncommitted changes."
    fi
    if [ -n "$(git ls-files --others --exclude-standard)" ]; then
        log_warn "Untracked files present (not part of the release)"
    fi
}

# check_tag_absent <tag> [hint]: runs in a dry run too, so a preview fails on a collision instead of
# promising a release the real run cannot make.
check_tag_absent() {
    local tag="$1" hint="${2:-}" remote
    if git rev-parse -q --verify "refs/tags/$tag" >/dev/null; then
        die "Tag $tag already exists in this checkout.${hint:+ $hint}"
    fi
    remote=$(git ls-remote --tags "$REMOTE" "refs/tags/$tag") \
        || die "Could not list the tags on $REMOTE."
    [ -z "$remote" ] || die "Tag $tag already exists on $REMOTE.${hint:+ $hint}"
}

# ── CI ────────────────────────────────────────────────────────────────────

setup_ci() {
    log_info "CI: configuring the release bot's git identity"
    git config user.name "shelly-release-bot"
    git config user.email "shelly-release-bot@users.noreply.github.com"

    git fetch --quiet --no-tags "$REMOTE" "refs/heads/$RELEASE_BRANCH" \
        || die "Could not fetch $RELEASE_BRANCH from $REMOTE."
    local head tip
    head=$(git rev-parse HEAD)
    tip=$(git rev-parse FETCH_HEAD)
    [ "$head" = "$tip" ] \
        || die "HEAD ($head) is not the tip of $RELEASE_BRANCH on $REMOTE ($tip): the branch moved after this run started. Start the release again."
    log_ok "HEAD is the tip of $RELEASE_BRANCH"
}

set_output() {
    if [ -n "${GITHUB_OUTPUT:-}" ]; then
        printf '%s=%s\n' "$1" "$2" >> "$GITHUB_OUTPUT"
    fi
}

# push_line <extra refspec>...: the one command that publishes, atomically: main plus every extra ref.
push_line() {
    echo "git push --atomic $REMOTE HEAD:refs/heads/$RELEASE_BRANCH $*"
}

push() {
    if [ "${CI:-}" != true ]; then
        log_warn "Not in CI: nothing pushed. To publish: $(push_line "$@")"
        return
    fi
    log_info "Pushing: $(push_line "$@")"
    git push --atomic "$REMOTE" "HEAD:refs/heads/$RELEASE_BRANCH" "$@" \
        || die "The push was refused; nothing was published."
    log_ok "Pushed"
}

# ── Plan ──────────────────────────────────────────────────────────────────

# plan <tag> <next VERSION>: the one summary a run prints and, in a workflow, the job summary.
plan() {
    local tag="$1" next="$2" title
    if [ "$DRY_RUN" = 1 ]; then title="Dry run: nothing is committed, tagged or pushed"; else title="Release"; fi
    printf '\n%s\n' "$title"
    printf '  %-16s %s\n' \
        "Branch:" "$RELEASE_BRANCH" \
        "Action:" "$RELEASE_ACTION" \
        "VERSION now:" "$CURRENT" \
        "Tag:" "${tag:-none}" \
        "VERSION after:" "$next"
    printf '\n'
    if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
        # The backticks are literal Markdown code spans, not command substitution.
        # shellcheck disable=SC2016
        {
            printf '### %s\n\n' "$title"
            printf '| | |\n|---|---|\n'
            printf '| Branch | `%s` |\n' "$RELEASE_BRANCH"
            printf '| Action | `%s` |\n' "$RELEASE_ACTION"
            printf '| VERSION now | `%s` |\n' "$CURRENT"
            printf '| Tag | `%s` |\n' "${tag:-none}"
            printf '| VERSION after | `%s` |\n' "$next"
        } >> "$GITHUB_STEP_SUMMARY"
    fi
}

# changelog_preview <tag>: print the section the release would write. Exit 3 when no fragment is left.
changelog_preview() {
    local out rc=0
    out=$(bash "$ASSEMBLE") || rc=$?
    case "$rc" in
        0) ;;
        3) die "No changelog fragment is left in changelog.d/ for $1: a release with empty notes is refused." 3 ;;
        *) die "changelog.d/ is invalid (the output above names the fragment)." ;;
    esac
    printf 'Changelog section for %s:\n\n%s\n\n' "$1" "$out"
    if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
        # The backticks are a literal Markdown fence, not command substitution.
        # shellcheck disable=SC2016
        printf '#### Changelog section\n\n```markdown\n%s\n```\n' "$out" >> "$GITHUB_STEP_SUMMARY"
    fi
}

# ── Git writes ────────────────────────────────────────────────────────────

write_version() {
    printf '%s\n' "$1" > "$VERSION_FILE"
}

commit_version() {
    write_version "$1"
    git add "$VERSION_FILE"
    git commit --quiet -m "$2"
    log_ok "Committed: $2"
}

# cut_release <tag> <next VERSION> <commit message for the VERSION bump>
cut_release() {
    local tag="$1" next="$2" bump_msg="$3"
    plan "$tag" "$next"
    check_tag_absent "$tag" "If it was released already, skip-prerelease-number moves VERSION past it."
    # A release never leaves VERSION naming a tag that already exists.
    check_tag_absent "$next"
    changelog_preview "$tag"
    if [ "$DRY_RUN" = 1 ]; then
        log_ok "Dry run complete: a real run tags $tag and sets VERSION to $next, with: $(push_line "refs/tags/$tag")"
        return
    fi
    validate_git_state

    [ "$tag" = "$CURRENT" ] || write_version "$tag"
    log_info "Assembling the CHANGELOG section for $tag"
    bash "$ASSEMBLE" --write "${tag#v}"
    git add "$VERSION_FILE" "$CHANGELOG_FILE"
    git commit --quiet -m "release: $tag"
    git tag -a "$tag" -m "Release $tag"
    log_ok "Committed and tagged $tag"
    commit_version "$next" "$bump_msg"

    push "refs/tags/$tag"
    if [ "${CI:-}" = true ]; then
        set_output tag "$tag"
    fi
    log_ok "Release $tag complete."
}

# move_version <next VERSION> <commit message>: the actions that only move VERSION.
move_version() {
    local next="$1" msg="$2"
    plan "" "$next"
    check_tag_absent "$next"
    if [ "$DRY_RUN" = 1 ]; then
        log_ok "Dry run complete: a real run sets VERSION to $next, with: $(push_line)"
        return
    fi
    validate_git_state
    commit_version "$next" "$msg"
    push
    log_ok "VERSION is now $next."
}

# ── Actions ───────────────────────────────────────────────────────────────

require_prerelease() {
    [ -n "$V_TYPE" ] \
        || die "VERSION is $CURRENT, not a prerelease. start-next-minor or start-next-major begins a new line."
}

run_main() {
    local base="v$V_MAJOR.$V_MINOR.$V_PATCH" target next
    case "$RELEASE_ACTION" in
        release)
            require_prerelease
            next="$base-$V_TYPE.$((V_NUM + 1))"
            cut_release "$CURRENT" "$next" "chore: bump version: $CURRENT -> $next"
            ;;
        release-as-beta|release-as-rc)
            require_prerelease
            target=${RELEASE_ACTION#release-as-}
            if [ "$(stage_order "$target")" -lt "$(stage_order "$V_TYPE")" ]; then
                die "VERSION is $CURRENT: going back from $V_TYPE to $target is refused. To restart at $target, begin a new line with start-next-minor first."
            fi
            if [ "$target" = "$V_TYPE" ]; then
                next="$base-$V_TYPE.$((V_NUM + 1))"
                cut_release "$CURRENT" "$next" "chore: bump version: $CURRENT -> $next"
            else
                cut_release "$base-$target.0" "$base-$target.1" \
                    "chore: bump version: $base-$target.0 -> $base-$target.1"
            fi
            ;;
        release-as-stable)
            require_prerelease
            next="v$V_MAJOR.$V_MINOR.$((V_PATCH + 1))-alpha.0"
            cut_release "$base" "$next" "chore: start next cycle: $next"
            ;;
        start-next-minor)
            next="v$V_MAJOR.$((V_MINOR + 1)).0-alpha.0"
            move_version "$next" "chore: start next cycle: $next"
            ;;
        start-next-major)
            next="v$((V_MAJOR + 1)).0.0-alpha.0"
            move_version "$next" "chore: start next cycle: $next"
            ;;
        skip-prerelease-number)
            require_prerelease
            next="$base-$V_TYPE.$((V_NUM + 1))"
            move_version "$next" "chore: bump prerelease: $CURRENT -> $next"
            ;;
    esac
}

main() {
    if [ "${1:-}" = --guard ]; then
        guard
        log_ok "'$RELEASE_ACTION' on $RELEASE_BRANCH is allowed (DRY_RUN=$DRY_RUN)."
        return
    fi
    [ $# -eq 0 ] || die "Usage: release.sh [--guard] (the action comes from RELEASE_ACTION)"
    if [ -z "$RELEASE_BRANCH" ]; then
        RELEASE_BRANCH=$(git symbolic-ref --quiet --short HEAD) \
            || die "HEAD is detached: set RELEASE_BRANCH to the branch being released."
    fi
    guard
    # GitHub sets CI=true on every runner, the workflow's dry-run preview included: that is safe only
    # because setup_ci, the push and set_output each stay off when DRY_RUN=1 (cut_release and
    # move_version return before them).
    if [ "${CI:-}" = true ] && [ "$DRY_RUN" != 1 ]; then
        setup_ci
    fi

    CURRENT=$(read_version)
    parse_version "$CURRENT"
    run_main
}

main "$@"
