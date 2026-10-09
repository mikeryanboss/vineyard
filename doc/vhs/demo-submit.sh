#!/usr/bin/env bash
# Submit prompts left in Claude Code's input box, for <seconds>. Claude Code
# can take the Enter that Vineyard sends after pasting a prompt as a newline.
# Usage: doc/vhs/demo-submit.sh <seconds>
set -uo pipefail

declare -A waiting
end=$((SECONDS + ${1:?usage: demo-submit.sh <seconds>}))
while [ $SECONDS -lt $end ]; do
	for s in $(tmux -L vineyard-demo ls -F '#S' 2>/dev/null); do
		# An unsent prompt sits just below the input box's top rule.
		if tmux -L vineyard-demo capture-pane -p -t "=$s:" | grep -B1 'Work on grapes issue' | head -1 | grep -q '^─'; then
			# Leave Vineyard a moment to send its own Enter first.
			if [ -n "${waiting[$s]:-}" ]; then
				tmux -L vineyard-demo send-keys -t "=$s:" Enter
				unset "waiting[$s]"
			else
				waiting[$s]=1
			fi
		else
			unset "waiting[$s]"
		fi
	done
	sleep 1
done
