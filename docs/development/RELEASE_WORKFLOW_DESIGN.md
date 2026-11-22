# Design note: a release workflow for shelly-manager

Status: proposal. Section 6 lists the decisions, one recommendation each. Nothing is built, tagged
or released by this note, and no workflow file changes with it. The build is cut into slices
(section 7) that are briefed after the owner chooses.

It answers ginsys/shelly-manager#846. Every `file:line` for this repository is read at origin/main
`e31982b4`. Other repositories are public and were read live on 2026-11-22.

## 1. What exists today

**No tag and no release.** `git ls-remote --tags origin` lists none, and `gh release list` is empty.
An operator cannot tell which version an image is.

**Images.** `docker-build.yml` runs after a green `Test` on `main` (`:3-8`, `:78`) and only when a
Docker-relevant path changed (`:25`, `:41`). It names each image `<UTC date>-<short sha>` (`:112`)
and `latest` (`:148-149`, `:163-164`), for `linux/amd64` and `linux/arm64` (`:177`, `:194`). It
passes `VERSION=` as a build argument (`:184`, `:201`). **Neither Dockerfile declares that argument**:
the only `ARG` lines are `TARGETPLATFORM` and `BUILDPLATFORM` (`deploy/docker/Dockerfile.manager:16-17`,
`Dockerfile.provisioner:16-17`), so the value is passed and dropped. The build flags are
`-ldflags '-s -w'` (`:21`), with no version in them.

**Eight values, none of them a release.**

| Where | Value |
|---|---|
| `CHANGELOG.md:1161`, `:1571` | `0.5.4` (2024-01-15), `0.5.4-alpha` |
| `.github/ISSUE_TEMPLATE/bug.yml:30` | `v0.5.5-alpha` (a placeholder) |
| `cmd/shelly-provisioner/main.go:540` | `0.5.0-alpha` |
| `deploy/kubernetes/helm/README.md:72-73` | chart `0.1.0`, `appVersion` `0.5.0-alpha` |
| `cmd/shelly-manager/main.go:506`, `docs/api/openapi.yaml:14` | `1.0.0` |
| `ui/package.json:4` | `0.0.1` |
| `internal/api/handlers.go:250` | `api_version` is the literal `dev` |
| image labels, `docker-build.yml:153`, `:168` | the date and sha name |

What does carry a build identity is the revision: the server reads `vcs.revision` from the Go build
information (`internal/api/build_info.go:8-14`) and reports it as `git_sha` (`handlers.go:252`); the UI
takes `GIT_SHA` or `git rev-parse` (`ui/vite.config.ts:9-13`), and a banner shows when the two differ
(`ui/src/App.vue:49`). Whether an image carries it is not read here: `deploy/docker/.dockerignore:5`
lists `.git`, but Docker reads that file from the build context root, and a build would settle it.

**The changelog is the one part that is ready, and it is a release mechanism already.** Each change
adds `changelog.d/<issue>-<slug>.md` (149 fragments today: 30 Added, 21 Changed, 3 Removed, 81 Fixed,
14 Security). `make changelog-assemble VERSION=X.Y.Z` runs `scripts/changelog-assemble.sh --write`
(`:9-14`): it keeps every line under `## [Unreleased]`, adds the fragment sections, heads the block
`## [X.Y.Z] - <date>` and deletes the fragments. `VERSION` may carry a pre-release part (`:23`).
`changelog.d/README.md:17-25` describes it as a **release PR**, which takes the label `no-changelog`
(`.github/workflows/changelog-fragment.yml:39`; `scripts/check-changelog-fragment.sh:7`). So the
version is typed into the tree, in a reviewed PR, **before any tag exists**. A design that adds a
second place to type it competes with this one.

**Branch rules.** `main` has required status checks, linear history and no deletion or non-fast-forward
(`gh api repos/ginsys/shelly-manager/rules/branches/main`, read 2026-11-22). Whether a workflow token
may push to `main` past them is not read; the recommendation does not need it.

**Two outputs ship differently.** The manager runs in a container. The provisioner runs on a Linux
host with NetworkManager (`docs/guides/provisioner-status.md:3-4`), and is also built as an image
(`docker-build.yml:190-201`). `Makefile:157` and `:161` build with `CGO_ENABLED=1`, but both
Dockerfiles build with `CGO_ENABLED=0` (`Dockerfile.manager:19`, `Dockerfile.provisioner:19`), so a
cross-compiled host binary needs no C toolchain.

**What deploys pin.** `deploy/kubernetes/kustomize/overlays/production/kustomization.yaml:42-45` pins
`newTag: 20250825-143022-abc1234`; the compose files read `${TAG:-latest}`
(`deploy/docker-compose/docker-compose.yml:3`, `:64`); the base deployment uses `:latest`
(`deploy/kubernetes/kustomize/base/deployment.yaml:34`, `:158`).

**Gates that exist.** `test.yml` has `Run Tests` (`:45`), `Database Provider Migration` (`:139`),
`Frontend Unit Tests` (`:202`), `Security Tests` (`:241`), `action-pins` (`:258`) and `Build Binary`
(`:268`, needs four of them, `:271`). End-to-end and performance runs are weekly
(`e2e-weekly.yml:11`, `performance-weekly.yml:10-11`).

**Upgrade and rollback.** Start-up runs repairs and `AutoMigrate` (`internal/database/migrations.go:17-37`);
an ambiguous legacy database makes start-up refuse (`docs/guides/database-upgrade.md:49`), and a
failure leaves the repair partly applied and resumable (`:89-101`). A search of `internal/database`
and `docs` for a downgrade or down-migration finds none. A SQLite backup is staged and swapped in at
the next start (`internal/database/restore/restore.go:1-6`), and the previous file is put back when the
restored database cannot be opened or migrated (`:258-260`). Archives carry a closed format version
(`internal/plugins/sync/sma/sma.go:19`, `2026.1`).

## 2. What the owner's other projects do

Public repositories with release automation:

| Project | Trigger | Scheme | Changelog | Version into the binary | Artifacts |
|---|---|---|---|---|---|
| ginsys/bronzeward | dispatch on `main`, `dry_run` default on (`release-create.yml:12-23`) | `v0.N.0-alpha.M`, also beta, rc (`guard.sh:18`) | section by hand, heading `## <tag>` (`guard.sh:31-36`) | `BW_VERSION` env to the package task; a check that every binary reports tag and commit (`release-publish.yml:46`, `:83-85`) | archives, SPDX SBOM each, `SHA256SUMS`, signed (`:91-130`) |
| ginsys/forward-email | push of a `v*` tag (`release.yml:3-6`) | SemVer, optional `-alpha.N`, `-beta.N`, `-rc.N` (`:23`) | section by hand, `## $TAG` checked (`:25`); notes from commits (`.goreleaser.yml:30-37`) | `-X ...version.Version` (`.goreleaser.yml:9-12`) | linux, darwin, windows, amd64, arm64 archives, `checksums.txt` |
| ginsys/goshlink | dispatch with a pre-release suffix (`release.yaml:4-9`) | file `VERSION` plus suffix (`:26-27`) | GitHub-generated notes (`.goreleaser.yaml:35-37`) | `-X main.version` (`.goreleaser.yaml:12`) | archives, `checksums.txt`, third-party licences (`release.yaml:53-54`) |
| go-kure/kure and go-kure/launcher | dispatch with an action choice and a dry run (`release.yml:9-28`); the tag then triggers Publish | `vX.Y.Z[-alpha\|beta\|rc.N]`; file `VERSION` holds the next one (`docs/releasing.md`, Reference) | git-cliff from Conventional Commits, committed by the workflow (`docs/releasing.md:177`) | launcher: `-X ...shared.Version` (`.goreleaser.yml:16-18`); kure ships no binary | launcher: archives, checksums, SBOM, cosign signature (`.goreleaser.yml:30-40`) |
| ginsys/wan-dns-sync | push of a `v*` tag builds an image (`build.yaml:3-7`) | semver image tags, `latest` only on a tag (`:63-69`) | none in the repository | no binary | container image; release `v1.0.0` exists with no asset and no workflow in the tree creates it |

| Project | Dry run | Guards | After the release |
|---|---|---|---|
| bronzeward | yes, builds and checks, publishes nothing (`release-publish.yml:104-112`) | `main` only, tag form, licence, changelog section, tag absent, CI passed on the commit, no existing release (`release-create.yml:46-99`, `release-publish.yml:53-77`) | nothing; the release is never Latest (`:128`) |
| forward-email | none | tag form, changelog section, tag newer than the last (`release.yml:23-33`) | nothing |
| goshlink | none | tag must not exist (`release.yaml:33-36`) | nothing |
| kure, launcher | yes | branch, CI wait of 30 minutes, tag and next `VERSION` absent, atomic push; Publish tests with the race detector and checks the tag is the newest of its line (`docs/releasing.md:162-197`) | `VERSION` moves on in the same push |
| wan-dns-sync | none | none | nothing |

Left out by a read of `isFork` and `isArchived`: five public forks (changedetection.io, mta-log-parser,
octodns-hetzner, truenas-cert-sync, unifi-reverse-dns) and the archived repositories. Public,
original and with **no release automation**: ginsys/discourse, ginsys/octodns-gitops, ginsys/parley,
ginsys/k8s-event-aggregation, ginsys/.github, go-kure/go-kure.github.io, and the public srgvg
repositories (no workflows directory). go-kure/.github holds the shared release workflow that kure and
launcher call. No public GitLab project of the owner has a release pipeline.

Private projects on GitHub and GitLab were read the same way, by workflow and CI files and release
lists, and are not named. A keyword read of their CI files found manual release jobs and the same two
tools as the public rows (git-cliff, GoReleaser); it found no feature the public rows lack.

## 3. Comparison with bronzeward

Bronzeward's pair is `release-create.yml` (guard, wait for CI, tag, call publish) and
`release-publish.yml` (build, check, SBOM, sign, attest, release), with `scripts/release/guard.sh`.

**Carries over, and why:**
- **A dispatch on `main` with a dry run that is the default** (`release-create.yml:19-23`). A tag is
  permanent, and a dry run is the only way to see the whole path without making one.
- **The guard before the tag** (`guard.sh:18-36`): tag form and a changelog section. Both fail
  before anything permanent exists.
- **A tag that must not exist, and a CI run that passed on that exact commit**
  (`release-create.yml:51-58`, `:60-99`). The tested commit is the one that ships.
- **The workflow makes the tag through the API with the workflow token** (`:113`). A tag the token
  makes starts no other workflow (`:6-7`), so nothing else can publish.
- **One release at a time** (`:25-28`).

**Does not carry over, and why:**
- **Archives as the product.** Bronzeward ships archives; here the main product is two images, and
  the host binary is a second, smaller output.
- **Hand-written changelog section.** Here the fragments and `changelog-assemble` already make it.
  The guard would read `## [X.Y.Z] - <date>`, not `## <tag>`.
- **Licence check** (`guard.sh:21-28`): specific to bronzeward's open licence decision.
- **A typed `version` input** (`release-create.yml:15-18`). It makes the version typed twice, once in
  the changelog heading and once at dispatch, and a guard to compare them. Section 5 reads it from
  the heading instead.
- **SBOM, keyless signing and attestations** (`release-publish.yml:87-124`). Section 5 says what each
  prevents and why the first build leaves them out.
- **Pre-releases only** (`guard.sh:18`). Fine while this project is alpha; the guard here must also
  accept a stable tag later.

## 4. Options

**A. Release PR, then a dispatched workflow tags and publishes.** The maintainer runs
`make changelog-assemble VERSION=X.Y.Z` in a branch, opens the release PR, and merges it through the
normal checks. A dispatch on `main` then reads the newest `## [X.Y.Z...]` heading, checks it, makes
the tag from the merge commit and publishes. *Order:* release PR, then tag. *What ties the tag to the
heading:* the workflow derives the tag **from** the heading, so they cannot differ. *Cost to build:*
one workflow, plus a version variable and a callable image build (slices 1 to 3). *Cost to run:* one
PR and one button; the dry run is the default.

**B. Release PR, then the maintainer pushes the tag; a tag-triggered workflow publishes**, as in
forward-email (`release.yml:3-6`) and wan-dns-sync. *Order:* release PR, then tag. *What ties them:*
a first job checks that the heading for the tag exists (`forward-email release.yml:25`). *Cost to
build:* the smallest: one workflow. *Cost to run:* a hand-pushed tag, with no dry run. The checks run
**after** the tag exists, so a wrong tag is already permanent: registries and the Go module proxy
pin a tag to its commit on first fetch, so a hand-pushed tag is the one act that cannot be undone.

**C. One dispatch writes the changelog, commits, tags and bumps**, as in kure and launcher. *Order:*
tag and changelog in one push. *What ties them:* the script writes both. *Cost to build:* the
largest: a changelog generator over Conventional Commits that **replaces** the fragments, a `VERSION`
file, and a push to `main` that the required checks on `main` may refuse. *Cost to run:* the least
per release. It is a second mechanism beside the one in section 1, and it throws that one away.

**Doing less** (images keep the date name; add a tag by hand when wanted) answers none of the
operator's questions, so it is not an option here.

## 5. Recommendation: option A, the smallest version of it

**Versioning.** SemVer with a pre-release part, `vX.Y.Z-alpha.N`, matching the project's alpha status.
**The first tag is `v0.6.0-alpha.1`:** the changelog already used `0.5.4`, and the 149 unreleased
fragments include 21 Changed and 3 Removed, so a minor step is honest. Breaking, while below 1.0, means
a minor bump: a changed or removed route inside `/api/v1`, a database change that needs the repair step
or refuses start-up (`database-upgrade.md:49`), a new SMA `FormatVersion` (`sma.go:19`), and a removed
or renamed configuration key (an unknown key is ignored, not refused, so a rename is silent). Anything
else is the next `alpha.N`. One wrinkle: each cut deletes the fragments it used, so the final stable
section lists only what came after the last alpha; it is merged by hand once.

**One source.** The version is typed once, by `make changelog-assemble VERSION=...` in the release PR.
The dispatch takes **no** version input: it reads the newest changelog heading, so the heading is the
source and the tag is derived from it. The tag is then the source for everything built: `-ldflags -X`
sets the version in both binaries, `/api/v1/version` reports it beside `git_sha`, the image labels
and tags use it, and the UI build receives it with `GIT_SHA`.

**Trigger and control.** Dispatch on `main`, `dry_run` on by default, one release at a time. Anyone
with write access to the repository can run it, which here is the owner. The release PR passes the
fragment check with the label `no-changelog`, and `main`'s rules apply to it like any other PR, so
the workflow never pushes to `main`.

**Artifacts.** Both images get the tag, as `vX.Y.Z-alpha.N`, built by the existing build made callable
with a version input, so there is one image build with two callers. The per-commit date names and
`latest` stay as they are (`latest` keeps meaning the newest `main` image; operators who want a fixed
version pin the tag). The provisioner binary is built for `linux/amd64` and `linux/arm64` with plain
`go build`, `CGO_ENABLED=0` (`Dockerfile.provisioner:19`), packed with `tar`, with one `SHA256SUMS`.
The GitHub release is a pre-release, never Latest, with the notes of that changelog section.

**Gates.** Each one beyond a tag, images, binaries and the changelog section names the failure it
prevents:

| Gate | Failure it prevents |
|---|---|
| Heading exists, is not empty, and no such tag exists | a release with empty notes, or a second use of a tag, which is permanent |
| CI passed on this commit (the whole `Test` workflow, database provider job included) | shipping a commit nobody tested |
| Dry run by default | making a permanent tag by a mistaken click |
| `SHA256SUMS` | a truncated or corrupted download of the host binary |
| One release at a time | two runs racing for the same tag |

The weekly end-to-end and performance runs are **not** gates: they take hours and their failure is
reported on their own schedule, and a gate that waits for them would stop every release by default.

**Left out of the first build, with the failure each would prevent.** *Keyless signing and
attestations:* prevent a swapped artifact being accepted by someone who is not the owner; today the
only consumer is the owner's own cluster pulling images from the owner's registry. *SBOM:* prevents a
consumer being unable to list what an artifact contains; nobody asks. Both are one extra step each
later (bronzeward `release-publish.yml:91-124`), so leaving them out closes nothing.

**Upgrade and rollback.** Start-up migrates the database and cannot go back (section 1). A release
therefore carries one fixed paragraph in its notes, with no new changelog section: take a database
backup before upgrading, and to roll back restore it and pin the previous tag. This note read no
test or document that shows an older binary running on a newer schema, so the release does not
promise it. A
release with a refusing or repairing migration says so in its own `Changed` or `Fixed` fragment.

## 6. Decisions needed

The owner decides these on ginsys/shelly-manager#846. The pull request that carries this note decides none.

1. **Scheme and first tag.** Recommend SemVer with `-alpha.N` and `v0.6.0-alpha.1`. Reason: the
   changelog used `0.5.4`, the unreleased work contains breaking changes, and nothing is stable yet.
2. **Where the version is typed.** Recommend the release PR's changelog heading, with no dispatch
   input. Reason: one typing and no mismatch to guard; the review of the PR is the check.
3. **Trigger.** Recommend a dispatch on `main`, dry run by default (option A). Reason: option B makes
   the permanent act before the checks, and option C replaces the fragments.
4. **`latest` and the per-commit names.** Recommend both unchanged. Reason: nothing that works today
   changes; a release adds a name, it removes none.
5. **Host binary.** Recommend plain `go build` for linux amd64 and arm64, tar and `SHA256SUMS`, with
   no GoReleaser. Reason: two binaries and two platforms need a dozen lines; GoReleaser (used by four
   of the owner's projects) adds a config and a pinned tool.
6. **Signing, SBOM, attestations.** Recommend none in the first build. Reason: no consumer except the
   owner; revisit when someone else pulls an artifact.
7. **Gate.** Recommend the `Test` workflow on the exact commit and no weekly run. Reason: the weekly
   runs would block every release, and their failures already reach the owner.
8. **Deploy files.** Recommend that the workflow edits none of `deploy/` (the kustomize `newTag`, the
   chart `appVersion`, the compose `TAG`). Reason: the files carry the owner's choices, and a
   workflow that edits them needs a push to `main`.

## 7. Build slices

Each slice is one pull request with its own tests; none is a release.

1. **A version variable.** A package variable set by `-ldflags -X`, shown by both binaries and by
   `/api/v1/version`, and the hard-coded `1.0.0` and `0.5.0-alpha` replaced by it. Tests pin the
   output, with `dev` when unset.
2. **Callable image build.** `docker-build.yml` gains a version input; both Dockerfiles declare
   `ARG VERSION` and pass it in `-ldflags`; the UI build receives it. Default behaviour is unchanged.
3. **The release workflow.** Guard, wait for CI, dry run, tag by API, images through slice 2,
   provisioner archives and `SHA256SUMS`, pre-release with the changelog notes.
4. **Documentation.** The release procedure and the operator's upgrade and rollback page; the stale
   values from section 1 (chart `appVersion`, the bug template placeholder, the OpenAPI `version`).
5. **Only if decision 6 changes:** signing, SBOM and attestations.

Recommended: the first tag is cut by the owner from a dry run, not by an agent session, and
ginsys/shelly-manager#846 stays open until slice 3 has run once. Both are the owner's to confirm.

## 8. What this note does not do

It builds nothing and measures nothing: whether an image carries `git_sha` needs a build, and whether
a workflow token may push past `main`'s rules needs a trial; the recommendation depends on neither.
It does not audit private projects, and it does not change the stale values it lists.
