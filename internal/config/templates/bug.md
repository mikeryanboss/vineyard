You own grapes issue #{{.ID}}: {{.Title}}. Owning it means you keep its files current: status, acceptance criteria, and comment log. Read .grapes/{{.ID}}/, its specification and comments, before you start.

Vineyard created this worktree and branch for this issue. Work and commit here; do not create another worktree.

It is a bug. Reproduce it first, and add a test that fails before your fix and passes after it. Record how you reproduced it in the issue's comments.

{{if eq .Status "in_progress"}}It is already in progress. Its comments say where earlier work stopped; continue from there.{{else if ne .Status "todo"}}Its status is {{.Status}}. Confirm with the user before working on it.{{end}}

{{with .Parent}}It is a sub-issue of #{{.ID}}: {{.Title}}. Read #{{.ID}} for shared context, but do not edit its files: its owner maintains them.{{end}}

{{if .Blockers}}It is blocked by issues that are not done:
{{range .Blockers}}- #{{.ID}} {{.Title}} ({{.Status}})
{{end}}Tell the user before you start.{{end}}

{{if .SubIssues}}It has sub-issues:
{{range .SubIssues}}- #{{.ID}} {{.Title}} ({{.Status}}){{with .Branch}}: another session works on it in branch {{.}}; do not edit its files{{end}}
{{end}}
{{if .BuildSubIssues}}Build the open sub-issues no other session works on yourself, one at a time, blockers first. Keep each one's status, acceptance criteria, and comments current, and commit each one's work separately.{{else}}Do not build its sub-issues here; they are left to their own sessions.{{end}}{{end}}
