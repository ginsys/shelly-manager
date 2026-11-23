# How to cut a release

One workflow makes a release: **Release** (`.github/workflows/release.yml`). You start it by hand.
It runs `scripts/release/release.sh`. The reasons for this shape are in
[RELEASE_WORKFLOW_DESIGN.md](RELEASE_WORKFLOW_DESIGN.md) (ginsys/shelly-manager#846).

## Before the first real run

The first real run is the owner's. A real run pushes a commit and a tag to `main`, and `main` has a
ruleset (`general`) that refuses such a push. So these must exist first:

1. A GitHub App owned by ginsys, installed on this repository only, with the repository permission
   Contents: read and write.
2. The App is a **bypass actor** of the ruleset `general`. Without it the push is refused, because the
   two new commits have no status checks.
3. An environment named `release`, limited to the branch `main`, with the secrets `RELEASE_APP_ID` and
   `RELEASE_APP_PRIVATE_KEY`.

A dry run needs none of this. It reads no secret.

## Start a run

Open Actions, then Release, then Run workflow. Pick the branch `main`. Two inputs:

| Input | Meaning |
|---|---|
| `dry_run` | **On by default.** Prints the plan and changes nothing. Untick it for a real run. |
| `action` | What to do. See below. |

The file `VERSION` holds the next version: `vX.Y.Z-alpha.N`, `-beta.N` or `-rc.N`. The first value is
`v0.6.0-alpha.1`. You never type a version into the workflow.

| Action | Tag | `VERSION` after |
|---|---|---|
| `release` | `VERSION` as it is | the next number (`alpha.1` becomes `alpha.2`) |
| `release-as-beta`, `release-as-rc` | that stage at `.0` (or `VERSION`, if it is at that stage) | the next number |
| `release-as-stable` | `vX.Y.Z` | `vX.Y.(Z+1)-alpha.0` |
| `start-next-minor`, `start-next-major` | none | the next line at `alpha.0` |
| `skip-prerelease-number` | none | the next number. Use it when a tag was used, or to skip one |

Going back a stage (rc to beta) is refused. A breaking change while the project is below 1.0 means a
minor step: run `start-next-minor` after the release. See the design note, section 5.

## What a run does

Always run it as a dry run first. The preview shows the version, the tag, the changelog section and the
push it would make. A real run does the same preview first, so a wrong tag or an empty changelog stops
it before the long steps.

A real run that makes a tag (`release` and the three `release-as-*`):

1. **Guard.** Refused from any branch but `main`. Refused if the tag, or the next `VERSION`, exists.
   Refused (exit 3) if no fragment is left in `changelog.d/`: the notes would be empty.
2. **Test.** The whole `Test` workflow runs on the dispatched commit. A red `Test` ends the run, and
   nothing is tagged. (`CODECOV_TOKEN` is not passed to a called `Test`, so the coverage upload is
   skipped. That is not a failure.)
3. **Release commit and tag.** The script assembles the fragments into `CHANGELOG.md`, commits
   `release: <tag>`, tags it, and commits the new `VERSION`.
4. **Push.** `main` and the tag go in **one atomic push** with the App token. If `main` moved during
   the Test wait, or the push is refused, nothing is published.
5. **Publish.** The tag starts `Release Publish`, which builds both images and makes the pre-release.
   The run waits for it (`scripts/release/wait-publish.sh`, at most 90 minutes) and is red if Publish is
   red. It is also red, with the `gh` error printed, after about a minute of reads that fail or answer
   nothing (a bad token, a missing `actions: read`). One green run means the version is out.

The three actions that only move `VERSION` do steps 1, 3 (the `VERSION` commit only) and 4, then end.

**One run at a time.** A second dispatch waits for the first. GitHub keeps one waiting run per group,
so a third dispatch cancels the waiting second one. When the first run ends, the waiting run is
refused, because `main` has moved.

## When a run is red

- **Before the push** (guard, Test, a refused push): nothing was published. Fix the cause and run again.
- **After the push** (Publish is red): the tag exists and is permanent. Never move or delete it. Read the
  failed Publish run. If it can be re-run (an outside fault), re-run it: Publish refuses a tag that has
  a release already. If the tag cannot be used, run `skip-prerelease-number`, then release again.
- A rejected push may mean the ruleset refuses the App (a rule other than the status checks). Report it
  on ginsys/shelly-manager#846. The first real run is where this is learned.
