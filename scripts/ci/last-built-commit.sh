#!/usr/bin/env bash
# Prints the full sha of the commit the newest main image was built from, or nothing: the caller then
# builds. The build tags each image <date>-<short sha of the commit it checked out> and moves `latest`
# only after the push, so that tag names the built commit. A run's own head_sha does not: on a
# workflow_run it is main's tip at trigger time.
# usage: last-built-commit.sh <package>   (needs gh, GH_TOKEN, GITHUB_REPOSITORY_OWNER, a full clone)
set -uo pipefail

package=${1:?usage: last-built-commit.sh <package>}
if ! tag=$(gh api "orgs/${GITHUB_REPOSITORY_OWNER:?}/packages/container/$package/versions?per_page=100" \
  --jq 'first(.[] | select(.metadata.container.tags | index("latest")) | .metadata.container.tags[] | select(test("^[0-9]{8}-[0-9]{6}-[0-9a-f]{7,40}$"))) // empty'); then
  echo "::warning::cannot read the versions of $package: the image build runs without a base" >&2
  exit 0
fi
[[ -n $tag ]] || exit 0
git rev-parse --verify --quiet "${tag##*-}^{commit}" || true
