#!/usr/bin/env bash
# Publish PR evidence images to the orphan branch `pr-evidence` as <id>/<basename>.
# Leaves the current branch, index, and working tree untouched.
# Prints the evidence commit SHA to embed in image links.
set -euo pipefail

if (( $# < 2 )); then
  echo "usage: $0 <issue-id> <image>..." >&2
  exit 2
fi
id=$1
shift

tmp="$(git rev-parse --show-toplevel)/.grapes/$id/tmp"
mkdir -p "$tmp"
export GIT_INDEX_FILE="$tmp/evidence.index"
rm -f "$GIT_INDEX_FILE"

status=0
git ls-remote --exit-code --heads origin pr-evidence >/dev/null || status=$?
case $status in
  0)
    git fetch --quiet origin pr-evidence
    parent=(-p FETCH_HEAD)
    git read-tree FETCH_HEAD
    ;;
  2) parent=() ;;  # first image ever: the branch does not exist yet
  *) exit "$status" ;;
esac

for image in "$@"; do
  blob=$(git hash-object -w "$image")
  git update-index --add --cacheinfo "100644,$blob,$id/$(basename "$image")"
done
commit=$(git commit-tree "$(git write-tree)" "${parent[@]}" -m "#$id: Add PR evidence")
git push --quiet origin "$commit:refs/heads/pr-evidence"
echo "$commit"
