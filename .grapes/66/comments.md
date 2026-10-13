### 2026-10-13T17:40
[STARTED] Change the default status bar key from ctrl+\ to ctrl+k, as the user chose: ctrl+\ needs AltGr on a Swiss keyboard.

### 2026-10-13T17:50
[VERIFY] gofmt -l . (no output), go vet ./... clean, go test ./... PASS (TestREADMEListsTheDefaultKeys checks the README block). Real app in tmux 100x20 on Vineyard's server: attached, the bar read " no other sessions ... ctrl+q back  ctrl+k hide" and stty size gave 19 100; after ctrl+k, 20 100. Not checked: VS Code's terminal, which may take ctrl+k for its chords. PASS
