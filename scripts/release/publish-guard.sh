#!/usr/bin/env bash
# The guards of the Publish workflow (.github/workflows/release-publish.yml). Each subcommand
# exits 0, or 1 with the reason on stderr.
#   form <tag>                    the tag is vX.Y.Z or vX.Y.Z-alpha|beta|rc.N
#   on-main <sha> [<ref>]         the commit is reachable from <ref> (default origin/main)
#   notes <tag> [<CHANGELOG.md>]  the tag's changelog section, then the fixed upgrade paragraph
set -euo pipefail

fail() {
  echo "publish-guard: $*" >&2
  exit 1
}

case "${1:-}" in
form)
  tag=${2-}
  [[ $tag =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-(alpha|beta|rc)\.(0|[1-9][0-9]*))?$ ]] ||
    fail "tag '$tag' is not vX.Y.Z or vX.Y.Z-alpha|beta|rc.N"
  ;;
on-main)
  sha=${2-}
  ref=${3:-origin/main}
  git merge-base --is-ancestor "$sha" "$ref" 2>/dev/null || fail "commit '$sha' is not on $ref"
  ;;
notes)
  tag=${2-}
  file=${3:-CHANGELOG.md}
  [[ -r $file ]] || fail "cannot read $file"
  # A cut section is headed "## [X.Y.Z-alpha.N] - <date>", the tag without its "v"; it ends at the next "## ".
  if ! body=$(awk -v want="${tag#v}" '
    /^## / {
      if (in_section) exit
      heading = $0
      sub(/^## \[/, "", heading)
      sub(/\].*/, "", heading)
      if ($0 ~ /^## \[/ && heading == want) { in_section = found = 1; next }
    }
    in_section { print }
    END { if (!found) exit 3 }' "$file"); then
    fail "no section for $tag in $file"
  fi
  body=$(printf '%s\n' "$body" | sed '/./,$!d')
  [[ -n ${body//[[:space:]]/} ]] || fail "the section for $tag in $file is empty"
  printf '%s\n\n%s\n' "$body" \
    "Back up the database before you upgrade. To roll back, restore that backup and pin the previous tag."
  ;;
*)
  echo "usage: publish-guard.sh form <tag> | on-main <sha> [<ref>] | notes <tag> [<CHANGELOG.md>]" >&2
  exit 2
  ;;
esac
