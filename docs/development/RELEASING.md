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
`.github/workflows/check-provisioner-binary.yml`, `.goreleaser.yaml`, `VERSION` or `CHANGELOG.md` runs a
dry run in `release-dry-run.yml`. It runs the guard, the provisioner build, the check of the binary and
the preview of the release. That file has no dispatch, a fixed dry run and no image job, so it never makes
a release or pushes an image, and it skips the release pull request itself.

## What a run does

A real run that makes a tag (`release` and the three `release-as-*`):

1. The guard refuses a wrong branch, a tag that exists and a run with no changelog fragment.
2. The workflow opens a release pull request. It holds the release commit (the changelog section and
   `VERSION` at the tag) and a second commit that moves `VERSION` to the next number.
3. The pull request runs the same required checks as any other, then merges by itself (rebase).
4. The workflow makes the tag on the release commit through the GitHub API. It waits at most 50 minutes
   for the merge. The tag is lightweight, and a tag made this way starts no other workflow, so the steps
   that follow are jobs of the same run.
5. The run builds the provisioner for linux amd64 and arm64 (`.goreleaser.yaml`), runs the amd64 binary
   and fails unless it reports the tag, and builds and pushes both images.
6. The run makes the GitHub pre-release (never Latest) with the archives, `SHA256SUMS` and the notes.

If `main` moves during the wait, the workflow closes the pull request, deletes its branch and makes no
tag. Nothing reached `main`. Run the release again.

The images are built from the commit the run started on, which is the one before the release commit. The
image revision label names that commit, and its files match the tag apart from `CHANGELOG.md` and
`VERSION`.

What an operator does with the result, the tags and files a release publishes and how to upgrade and roll
back, is in [upgrade-and-rollback.md](../guides/upgrade-and-rollback.md).

## When a run is red

- **Before the pull request opens:** nothing changed. Fix the cause and run again.
- **The pull request is open, or merged with no tag:** follow "A run that was interrupted" on the
  shared page. Do not make the tag by hand.
- **The tag exists and a later job is red** (the build, an image or the publish step): the tag is
  permanent. Never move or delete it. Read the failed job. If it was an outside fault, re-run the failed
  jobs of the same run: the publish step refuses a tag that already has a release, so it cannot publish
  twice. If the tag cannot be used, run `skip-prerelease-number`, then release again.
- A refused step on the first real run may mean the App or the environment is not set up as above.
  Report it on ginsys/shelly-manager#846.
