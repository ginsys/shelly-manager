#!/usr/bin/env bash
# wait-publish.sh <tag> — the last step of the Release workflow: wait for the Release Publish run
# that the pushed tag started, and fail unless it succeeded. Needs gh, GH_TOKEN and GITHUB_REPOSITORY.
# Exit 0 Publish succeeded; 1 anything else, with the reason, and the last gh error where gh failed.
# A gh that keeps failing (a bad token, a 403 on `actions: read`) or keeps answering nothing is an
# error after WAIT_MISSES reads in a row (about a minute), never a silent wait for the deadline.
#
# Tunables, for the tests: WAIT_POLL seconds between reads (10), WAIT_START_TRIES reads for the run
# to appear (12, so 2 minutes), WAIT_MISSES failed or empty reads in a row (6), WAIT_DEADLINE seconds
# for Publish to finish (5400, 90 minutes: Publish takes about 50).
set -euo pipefail

tag=${1:?usage: wait-publish.sh <tag>}
poll=${WAIT_POLL:-10}
start_tries=${WAIT_START_TRIES:-12}
max_misses=${WAIT_MISSES:-6}
limit=${WAIT_DEADLINE:-5400}
workflow=release-publish.yml

err=$(mktemp)
trap 'rm -f "$err"' EXIT

fail() {
    echo "::error::$1 The tag is pushed: see docs/development/RELEASING.md, 'When a run is red'."
    exit 1
}

# The run the tag started. An empty answer means "not started yet"; a failed read is a miss.
run_id=""
misses=0
for ((i = 1; i <= start_tries; i++)); do
    if run_id=$(gh run list --repo "$GITHUB_REPOSITORY" --workflow "$workflow" --branch "$tag" \
        --event push --json databaseId --limit 1 --jq '.[0].databaseId // empty' 2>"$err"); then
        misses=0
        [ -z "$run_id" ] || break
        echo "Attempt $i/$start_tries: Release Publish has not started for $tag."
    else
        run_id=""
        misses=$((misses + 1))
        echo "::warning::gh run list failed ($misses/$max_misses): $(cat "$err")"
        [ "$misses" -lt "$max_misses" ] \
            || fail "Cannot read the runs of $workflow: $(cat "$err")"
    fi
    sleep "$poll"
done
[ -n "$run_id" ] || fail "Release Publish did not start for $tag within $start_tries reads."

deadline=$(($(date +%s) + limit))
misses=0
echo "Watching run $run_id of Release Publish for $tag..."
while [ "$(date +%s)" -lt "$deadline" ]; do
    if conclusion=$(gh run view "$run_id" --repo "$GITHUB_REPOSITORY" --json status,conclusion \
        --jq 'if .status == "completed" then .conclusion else "in_progress" end' 2>"$err") \
        && [ -n "$conclusion" ]; then
        misses=0
        case "$conclusion" in
            success) echo "Release Publish completed for $tag."; exit 0 ;;
            in_progress) ;;
            *) fail "Release Publish for $tag concluded $conclusion." ;;
        esac
    else
        misses=$((misses + 1))
        last=$(cat "$err")
        echo "::warning::gh run view gave no answer ($misses/$max_misses): ${last:-an empty answer}"
        [ "$misses" -lt "$max_misses" ] \
            || fail "Cannot read run $run_id of $workflow: ${last:-gh answered nothing}"
    fi
    sleep "$poll"
done
fail "Release Publish did not finish for $tag within $limit seconds."
