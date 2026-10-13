### 2026-10-13T12:40
[DECISION] Show the conversation oldest first, like a chat, and open it at its end so the newest entry is in view. Human messages are the `user` lines with `"origin":{"kind":"human"}`: checked across 517 such lines in local transcripts from Claude Code 2.1.272 to 2.1.296, where slash commands, tool results, and meta lines never carry it. The `last-prompt` fallback is dropped; the messages cover it.

### 2026-10-13T13:05
[VERIFY] `gofmt -l .` empty, `go vet ./...` clean, `go test ./...` passes in about 7 s. Ran the dev build in tmux on a fake `claude` session with a hand-written transcript (CLAUDE_CONFIG_DIR in `.grapes/52/tmp/claude`): the tab shows messages and recaps interleaved in time order, opened at the newest entry, in light and dark themes. Screenshots in the PR. PASS.
