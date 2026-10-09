#!/usr/bin/env bash
# Create the throwaway repository that demo.tape records, replacing any
# earlier one, and stop the demo's tmux server.
# Usage: doc/vhs/demo-repo.sh <dir>
set -euo pipefail

dir=${1:?usage: demo-repo.sh <dir>}
tmux -L vineyard-demo kill-server 2>/dev/null || true
rm -rf "$dir"
mkdir -p "$dir"
cd "$dir"
git init -q -b main

cat > tally.py <<'EOF'
"""Count the words in a text file."""

import sys
from collections import Counter


def count(text):
    return Counter(text.split())


def main():
    with open(sys.argv[1]) as f:
        counts = count(f.read())
    for word, n in sorted(counts.items()):
        print(f"{n:6} {word}")


if __name__ == "__main__":
    main()
EOF

issue() { # id status priority title body
	mkdir -p ".grapes/$1"
	cat > ".grapes/$1/meta.toml" <<EOF
title = '$4'
status = '$2'
priority = '$3'
labels = []
created = 2026-10-01T09:00:00Z
updated = 2026-10-01T09:00:00Z
EOF
	printf '## Goal\n%s\n' "$5" > ".grapes/$1/content.md"
}
issue 1 done medium 'Count words in a file' 'Print how often each word appears in a file.'
issue 2 todo high 'Add a --top flag' 'Add `--top N` to `tally.py`, printing only the N most common words, most common first.'
issue 3 todo medium 'Ignore case when counting' 'Count "The" and "the" as the same word in `tally.py`.'
issue 4 todo low 'Read from stdin' 'Make `tally.py` read standard input when no file is given.'

git add .
git commit -q -m "Count words in a file"
