# Vineyard TUI demo
# Usage, from the repository root: vhs doc/vhs/demo.tape
# Runs real Claude Code agents in a throwaway repository at /tmp/vineyard-demo,
# which Claude Code must already trust. See "README Demo" in docs/development.md.

Output doc/demo.gif

Require claude
Require tmux
Require vineyard

Set Shell "bash"
Set FontSize 14
Set Width 1200
Set Height 720
Set Padding 20
Set Theme "Catppuccin Mocha"
Set TypingSpeed 50ms

# Off camera: create the repository and start agents on issues #2 and #3.
Hide
Type "doc/vhs/demo-repo.sh /tmp/vineyard-demo && cd /tmp/vineyard-demo && export VINEYARD_TMUX_SOCKET=vineyard-demo"
Enter
Sleep 1s
Type "$OLDPWD/doc/vhs/demo-submit.sh 240 & vineyard -y -i 2"
Enter
Sleep 2s
Enter
Sleep 300ms
Enter
Sleep 1s
Type "q"
Sleep 500ms
Type "vineyard -y -i 3"
Enter
Sleep 2s
Enter
Sleep 300ms
Enter
# Both agents have finished once both sessions are ready with changes.
Sleep 5s
Wait+Screen@180s /2 ready[\s\S]*2\/add-a-top-flag[^\n│]*[+][0-9]+ -[0-9]+[\s\S]*3\/ignore-case[^\n│]*[+][0-9]+ -[0-9]+/
Type "k"
Sleep 1s
Show

# Each agent's live screen.
Sleep 2.5s
Type "j"
Sleep 2.5s
Type "k"
Sleep 1s

# The selected session's diff and issue, then back to its screen.
Tab
Sleep 3s
Tab
Sleep 2.5s
Tab
Sleep 300ms
Tab
Sleep 1s

# The issues screen opens at the selected session's issue.
Type "i"
Sleep 2s
Escape
Sleep 1.5s

# Start an agent on #4 from the board.
Type "/"
Sleep 300ms
Type "stdin"
Enter
Sleep 1s
Type "a"
Sleep 2s
Enter
Sleep 300ms
Enter

# Watch it work.
Sleep 10s

Hide
Type "q"
Sleep 500ms
Type "tmux -L vineyard-demo kill-server"
Enter
