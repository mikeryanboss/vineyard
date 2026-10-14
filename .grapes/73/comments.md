### 2026-10-14T12:20
[DECISION] Derive the names from the directory of the checkout that holds the script (`${BASH_SOURCE[0]}/..`), not the current directory, and not a random draw: each drive command is its own process and must find the same names without saved state. Vineyard's worktree names already carry random hex.

### 2026-10-14T12:25
[VERIFY] Two checkouts, this worktree (`rename-531d5a`) and `git worktree add --detach .grapes/73/tmp/other` with the new script, each ran `init`, `start` of the same build, `n`, a title, Enter. `tmux -L vineyard-drive-rename-531d5a ls` and `tmux -L vineyard-drive-other ls` each listed `drive`; this screen showed only "mine", the other only "theirs". After `stop` in `other`: its tmux server was gone, this one's window still listed with "mine", and `vineyard server .../vineyard-drive-dev-rename-531d5a-p5` still ran. `stop` here then ended both. `bash -n scripts/drive` clean. PASS.

### 2026-10-14T12:26
[DONE] `scripts/drive`: names `vineyard-drive-<checkout>` and `vineyard-drive-dev-<checkout>`, usage text. `docs/development.md`: the `vineyard debug` example and why the names differ per worktree.
