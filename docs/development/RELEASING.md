# How to cut a release

One workflow makes a release: **Release** (`.github/workflows/release.yml`). You start it by hand. It
calls the shared release workflow of ginsys/.github. The actions, the checks and the repair steps are
on the [shared release page](https://github.com/ginsys/.github/blob/main/docs/release.md). This page
holds only what is specific to shelly-manager. The older design note is
[RELEASE_WORKFLOW_DESIGN.md](RELEASE_WORKFLOW_DESIGN.md) (ginsys/shelly-manager#846).

## Before the first real run

The first real run is the owner's. A real run opens a release pull request as a GitHub App. So these
must exist first:

1. The GitHub App `ginsys-release-bot`, installed on this repository, with no bypass of any ruleset.
2. An environment named `release`, limited to the branch `main`, with the secrets `RELEASE_APP_ID` and
   `RELEASE_APP_PRIVATE_KEY`.
3. Auto-merge allowed in the repository. It is.

A dry run needs none of this. It reads no secret.

## Start a run

Open Actions, then Release, then Run workflow. Pick the branch `main`. Two inputs:

| Input | Meaning |
|---|---|
| `dry_run` | **On by default.** Prints the plan and changes nothing. Untick it for a real run. |
| `action` | What to do. The shared page lists the seven actions. |

The file `VERSION` holds the next version: `vX.Y.Z-alpha.N`, `-beta.N` or `-rc.N`. The first value is
`v0.6.0-alpha.1`. You never type a version into the workflow. Always run it as a dry run first: the
preview shows the version, the tag and the changelog section.

A pull request that changes `.github/workflows/release.yml`, `.github/workflows/release-dry-run.yml`,
`.goreleaser.yaml`, `VERSION` or `CHANGELOG.md` runs a dry run in `release-dry-run.yml`. That file has no
dispatch and a fixed dry run, so it never makes a release, and it skips the release pull request itself.

## What a run does

A real run that makes a tag (`release` and the three `release-as-*`):

1. The guard refuses a wrong branch, a tag that exists and a run with no changelog fragment.
2. The workflow opens a release pull request. It holds the release commit (the changelog section and
   `VERSION` at the tag) and a second commit that moves `VERSION` to the next number.
3. The pull request runs the same required checks as any other, then merges by itself (rebase).
4. The workflow makes the tag on the release commit through the GitHub API. It waits at most 50 minutes
   for the merge. A tag made this way starts no other workflow.

If `main` moves during the wait, the workflow closes the pull request, deletes its branch and makes no
tag. Nothing reached `main`. Run the release again.

Until the build and publish jobs are added to this workflow, a run ends at the tag.

What an operator does with the result, the tags and files a release publishes and how to upgrade and roll
back, is in [upgrade-and-rollback.md](../guides/upgrade-and-rollback.md).

## When a run is red

- **Before the pull request opens:** nothing changed. Fix the cause and run again.
- **The pull request is open, or merged with no tag:** follow "A run that was interrupted" on the
  shared page. Do not make the tag by hand.
- **The tag exists:** it is permanent. Never move or delete it.
- A refused step on the first real run may mean the App or the environment is not set up as above.
  Report it on ginsys/shelly-manager#846.
