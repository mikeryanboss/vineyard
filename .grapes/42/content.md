## Goal

`go install github.com/mikeryanboss/vineyard@latest` installs a new release within about a minute of its tag, instead of up to 30 minutes later.

## Description

The Go module mirror, proxy.golang.org, caches a module's version list. On 2026-10-11, releases v0.1.5–v0.1.7 came minutes apart and `go install …@latest` kept installing an older one for a while; at one point `@latest` still answered v0.1.4 while `@v/list` already had v0.1.7. The mirror's FAQ (https://proxy.golang.org/, "why isn't it showing up when I run go get -u") says to "explicitly request that version via `go get module@version`. The new version should be available within one minute." pkg.go.dev (https://pkg.go.dev/about, adding a package) says a request to any endpoint of the module proxy protocol counts, for example `https://proxy.golang.org/example.com/my/module/@v/v1.0.0.info`.

After GoReleaser publishes, the release workflow requests the `.info` of each `v*` tag on the released commit from the mirror. A commit can carry two: a minor version tagged by hand at `origin/main`, which Auto Tag has usually tagged already.

The user also confirmed on 2026-10-12 that only changes to the application are released, as `auto-tag.yml`'s `paths` filter does (#37). This PR changes no program file, so its merge is the first check that such a merge starts no Auto Tag run, the criterion #37 left open.

## Context

- `.github/workflows/release.yml`: runs on a pushed `v*` tag, or when `.github/workflows/auto-tag.yml` calls it. In a call, `github.ref_name` is `main`, not the tag; the checked-out commit carries the tag, which GoReleaser finds the same way.
- The module path is `github.com/mikeryanboss/vineyard` (`go.mod`); it has no upper-case letters, so it needs no case-encoding in proxy URLs.
- `docs/development.md` (Releases).

## Acceptance Criteria

- [x] After GoReleaser succeeds, `release.yml` requests `https://proxy.golang.org/<module>/@v/<tag>.info` for each `v*` tag on the released commit, and the job fails when a request fails.
- [x] The step works whether a tag push or Auto Tag started the workflow.
- [x] `docs/development.md` says so.
- [ ] After merge: no Auto Tag run for the merge commit, since it changes no program file.

## Verify

Run from the worktree root:

```bash
actionlint .github/workflows/*.yml
```

Run the new step's script, extracted from the workflow, in a scratch clone with HEAD at v0.1.8 (one tag), with a second tag on HEAD, and with a tag the mirror cannot find.

After merge: `gh run list --workflow auto-tag.yml --limit 3` shows no run for the merge commit.

## Pass Criteria

actionlint prints nothing. At v0.1.8 the script prints the mirror's JSON for v0.1.8; with two tags it requests both; with a tag missing from GitHub it exits non-zero. After merge, no Auto Tag run lists the merge commit.
