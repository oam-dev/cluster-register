# Releasing cluster-register

KubeVela uses cluster-register as a Go module: its `pkg/hub` and `pkg/spoke` packages register a cluster for
`vela cluster join --engine ocm`. A release is a Go module tag; no workflow publishes an image or a chart. A fix
reaches KubeVela users through a cluster-register tag, then KubeVela's `go.mod` and a KubeVela patch release.

## Branches

| Branch | What it is | Tags | Used by KubeVela |
| --- | --- | --- | --- |
| `master` | development; the next minor release | none | — |
| `release-1.1` | the 1.1 line | `v1.1.x` (`v1.1.0-alpha.1` today) | `master`, `release-1.12` |
| `release-1.0` | the 1.0 line | `v1.0.x` (`v1.0.4` today) | `release-1.10`, `release-1.11` |

The release branches get security and bug fixes; new features go to `master`. A new minor line starts with a
`release-x.y` branch cut from `master` at the commit of its first tag.

## Backports

A fix goes to `master` first, then to the release branches that need it. The
[Backport workflow](.github/workflows/backport.yml) opens the backport pull requests:

- **Before merging**, add a label per branch: `backport release-1.0`, `backport release-1.1`. When the pull request is
  merged, the workflow cherry-picks its commits (with `-x`) onto each branch and opens one pull request per branch.
- **After merging**, a maintainer comments `/backport release-1.0` (or `/backport 1.0`; several branches separated by
  spaces) on the merged pull request. Adding a label after the merge does nothing.

If a commit doesn't apply, the workflow says so in a comment and the run fails; backport that branch by hand
(`git cherry-pick -x`) in a pull request titled `[Backport release-x.y] <title>`.

[Dependabot](.github/dependabot.yml) opens one grouped pull request a week per branch for the GitHub Actions.

## CI

[CI](.github/workflows/ci.yml) runs on every push to `master` and `release-*` and on pull requests to them: `go.mod` /
`go.sum` are tidy, `go build ./...`, `go vet ./...` and `gofmt`, with the Go version from each branch's `go.mod`.

## Making a release

1. Merge the fixes into the release branch (backports above), with CI green.
2. Test the branch head as KubeVela uses it: point a KubeVela branch that uses the line at the commit
   (`go mod edit -require=github.com/oam-dev/cluster-register@<commit>`, `go mod tidy`), build it, and join a cluster
   with `vela cluster join --engine ocm`.
3. Tag the tested commit, `vX.Y.Z` on `release-x.y`. Tags are never moved or reused: a fix after tagging is the next
   patch.
4. Check that the Go proxy serves the tag (`go list -m github.com/oam-dev/cluster-register@vX.Y.Z`), then publish the
   GitHub Release with the release notes. Mark only the highest final release as Latest; a `-alpha.N` / `-rc.N` tag
   is a pre-release.
5. Update KubeVela's `go.mod` on the branches that use the line (table above) to the new tag. Then update the table
   above and in the [README](README.md).
