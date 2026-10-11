### 2026-10-11T16:30
[FINDINGS] Reproduced on 0910bc6 with hunk as the diff tool: a double click on a diff code line did nothing, on a file header it folded once, and on the Diff tab it opened hunk. The user reported the diff view as the one thing that does not work yet.

### 2026-10-11T16:30
[DECISION] Code lines open the full diff on a double click, as `enter` does for the tab, matching the preview, where a double click attaches. Headers and summary lines keep folding or jumping once: a double click there opening the diff tool too would act twice for one gesture. The first click decides, since a jump moves the line under the pointer.
