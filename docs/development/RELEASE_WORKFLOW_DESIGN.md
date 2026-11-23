# Design note: a release workflow for shelly-manager

Status: proposal, revision 2. Section 6 lists the points that are still the owner's, one
recommendation each. Nothing is built, tagged or released by this note, and no workflow file changes
with it. The build is cut into slices (section 7) that are briefed after the owner confirms.

**Revision 2, 2026-11-22.** The owner answered the first version, about 21:50 CET: "all should happen
from a single workflow, automated; I want a similar experience as with other projects; allow bumping
the version, pre-release type, etc; update recommendations". Revision 1 recommended a release PR that
carried the version, typed by hand into its changelog heading, and a separate dispatch that read that
heading and tagged. Sections 5, 6 and 7 now recommend one dispatch with
an action choice, as go-kure/kure has it. Sections 1 to 4 and 8 are the record of what was read and
weighed, with a one-line edit where the answer made a sentence untrue.

It answers ginsys/shelly-manager#846. Every `file:line` for this repository is read at origin/main
`e31982b4`, except where section 5 or 6 names another commit. Other repositories are public and were
read live on 2026-11-22.

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

**Eight rows of version values, none of them a release.**

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

*2026-11-23, slice 5 of ginsys/shelly-manager#846:* the rows above record what existed. The application version has been one value since
ginsys/shelly-manager#900 (the two `cmd/` rows above, the provisioner's `0.5.0-alpha` and the manager's `1.0.0`, now read it). This slice changed the
bug template placeholder (it names `shelly-manager --version`), the chart `appVersion` example (a placeholder naming the root `VERSION`
file; the chart `version` is the chart's own and stays) and the version line of `.claude/CLAUDE.md`. It left `docs/api/openapi.yaml:14`
(the version of the API document, kept by hand, now with a comment saying where the application version lives), the kurel README's
`version: v1.0.0` (a kurel definition's own version), the UI guide's "v0.5.4+" (a floor that is still true) and the example
`shelly_manager_version: "0.5.1"` in `docs/features/BACKUP_FORMAT_SPECIFICATION.md` and `docs/features/DATABASE_ARCHITECTURE.md`
(illustrative values of a recorded field). `ui/package.json:4` and `api_version` are code and not part of this slice.

What does carry a build identity is the revision: the server reads `vcs.revision` from the Go build
information (`internal/api/build_info.go:8-19`) and reports it as `git_sha` (`handlers.go:252`); the UI
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
second place to type it competes with this one. *Revision 2: section 5 keeps this script and takes
the version from a `VERSION` file instead of a typed argument; the release PR is replaced by the
workflow's own commit.*

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
  makes starts no other workflow (`:6-7`), so nothing else can publish. *Revision 2: not carried over.
  The tag must start Publish by itself, so section 5 pushes it with the release bot's token.*
- **One release at a time** (`:25-28`).

**Does not carry over, and why:**
- **Archives as the product.** Bronzeward ships archives; here the main product is two images, and
  the host binary is a second, smaller output.
- **Hand-written changelog section.** Here the fragments and `changelog-assemble` already make it.
  The guard would read `## [X.Y.Z] - <date>`, not `## <tag>`.
- **Licence check** (`guard.sh:21-28`): specific to bronzeward's open licence decision.
- **A typed `version` input** (`release-create.yml:15-18`). It makes the version typed twice, once in
  the changelog heading and once at dispatch, and a guard to compare them. *Revision 2: section 5
  reads it from a `VERSION` file and offers an `action` choice instead, as go-kure/kure does.*
- **SBOM, keyless signing and attestations** (`release-publish.yml:87-124`). Section 5 says what each
  prevents and why the first build leaves them out.
- **Pre-releases only** (`guard.sh:18`). Fine while this project is alpha; the guard here must also
  accept a stable tag later.

## 4. Options

*Revision 2: the owner chose the shape of C, with the fragments kept where C replaces them. Section
5 says how. A and B stay as what was weighed, and C's "throws that one away" below no longer holds.*

**A. Release PR, then a dispatched workflow tags and publishes.** The maintainer runs
`make changelog-assemble VERSION=X.Y.Z` in a branch, opens the release PR, and merges it through the
normal checks. A dispatch on `main` then reads the newest `## [X.Y.Z...]` heading, checks it, tags
**the head of `main` at the moment of the dispatch** (as bronzeward does, `release-create.yml:113`,
not necessarily the release PR's merge commit) and publishes. A pull request merged between the two
would ship a change the changelog section omits, so the workflow refuses while any fragment is left
in `changelog.d/` at that commit. *Order:* release PR, then tag. *What ties the tag to the
heading:* the workflow derives the tag **from** the heading, so they cannot differ. *Cost to build:*
one workflow, plus a version variable and a callable image build (slices 1 to 3). *Cost to run:* one
PR and one button, plus a full `Test` run on the release commit, started by the workflow; the dry
run is the default.

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

## 5. Recommendation: one dispatch, as in kure, with this repository's own script

**One workflow.** `release.yml` starts by hand (`workflow_dispatch`) with two inputs: an `action`
choice and `dry_run`. A real run is allowed from `main` only, and one run at a time. The run checks
the branch and the action, waits for `Test` on the dispatched commit, then runs
`scripts/release/release.sh`: it works out the version from a `VERSION` file, assembles the
changelog, commits `release: vX.Y.Z-alpha.N`, tags it, commits the bump to the next number, and
pushes `main` and the tag in **one atomic push**. The tag starts `release-publish.yml` by itself, and
the release run waits for it and turns red if it fails, so one green run means the version is out.
That is the path of the actions that tag (`release` and the three `release-as-*`). The three that
only move `VERSION` (`start-next-minor`, `start-next-major`, `skip-prerelease-number`) make no
tag, assemble no changelog, skip the `Test` wait and wait for no Publish, as kure's workflow has it
(the CI job runs only for an action that starts with `release`, `release.yml:74`; the Publish wait only
when the script reports a tag, `:190`). This is the sequence of go-kure/kure: its caller `.github/workflows/release.yml` (40 lines), the shared
`release.yml` in go-kure/.github and its `scripts/release/release.sh` (read at go-kure/.github
`f6118283`, 2026-11-22).

**The actions** are kure's seven, each mapped to what it does here (`release.sh` in go-kure/.github,
`run_main`):

| Action | Here |
|---|---|
| `release` | kept: tag `VERSION` as it stands; `VERSION` moves to the next number |
| `release-as-beta`, `release-as-rc` | kept: move to that stage at `.0` and tag it; going back a stage is refused |
| `release-as-stable` | kept: tag `vX.Y.Z`; `VERSION` moves to the next patch at `-alpha.0` |
| `start-next-minor`, `start-next-major` | kept for the `VERSION` move to the next line's `alpha.0`, with no tag. The script's `release/vX.Y` branch for patch releases is dropped: it exists only when the line being left has a stable tag, and none does. Add it with the first stable tag |
| `skip-prerelease-number` | kept: recovery when a tag was used, or to move past one |
| `dry_run` | kept, and **on by default** (kure's default is off): a tag is permanent |

**Reuse or copy: copy and adapt, with the source commit named in the file.** A ginsys repository
cannot call go-kure's shared workflow today, for four reasons read from its own lines. Every job runs
on `autops-kube-kure` (`release.yml:35`, `:75`, `:124`), a go-kure runner label; every workflow here
uses `ubuntu-latest` or `ubuntu-24.04`, and a read of the ginsys org's runners returned none. It signs in
with the go-kure App (`KURE_BOT_APP_ID` and `KURE_BOT_APP_PRIVATE_KEY`, `:146-147`), which is
installed on go-kure and not on ginsys. The script writes the changelog with `git-cliff`
(`release.sh:352`) with no switch, and refuses to push without a downstream-reference check file
(`FORBIDDEN_TERMS_CHECK`, `:62`). Its publish workflow is GoReleaser, a Go module proxy refresh and a
docs deploy, none of which this repository has. ginsys/.github holds no release workflow to inherit
(`ci.yml`, `pr-review.yml`, `settings.yml` and two tracker audits). So this repository carries the
caller shape, the action choice and the script's version logic, minus the release branches, the
forbidden-terms check and `git-cliff`. A shim that fakes `git-cliff` would make the shared script
the owner of a step it cannot do.

**Version.** A root `VERSION` file holds the next version, `vX.Y.Z-alpha.N`, and the actions above
move it. SemVer with a pre-release part, matching the project's alpha status. **The first value is
`v0.6.0-alpha.1`:** the changelog already used `0.5.4`, and the 149 unreleased fragments include 21
Changed and 3 Removed, so a minor step is honest. The first tag is the action `release`, dry run
first: it tags `v0.6.0-alpha.1` and moves `VERSION` to `v0.6.0-alpha.2`. Breaking, while below 1.0,
means a minor bump (`start-next-minor`): a changed or removed route inside `/api/v1`, a database
change that needs the repair step or refuses start-up (`database-upgrade.md:49`), a new SMA
`FormatVersion` (`sma.go:19`), and a removed or renamed configuration key (an unknown key is ignored,
not refused: the loader calls a plain `viper.Unmarshal`, `internal/config/config.go:248`, and a
binary test starts with removed keys still set, `cmd/shelly-manager/main_test.go:237`; so a rename is
silent). Anything else is the next `alpha.N`. The tag is then the source for everything built:
`-ldflags -X` sets the version in both binaries, `/api/v1/version` reports it beside `git_sha`, the
image labels and tags use it, and the UI build receives it with `GIT_SHA`.

**Changelog: the fragments, assembled by the release; no `git-cliff`.** `scripts/changelog-assemble.sh
--write` already turns the 149 curated fragments into the section, heads it `## [X.Y.Z-alpha.N] -
<date>` (the version without its `v`; the tag has one) and deletes the fragments. `git-cliff` would
replace that text with commit subjects, and using both is two generators. The fragment check for pull
requests stays as it is. The release commit is made by the workflow, not by a pull request, so it needs
no `no-changelog` label. A run that tags with no fragment left is refused (the script exits 3): empty
notes. A `VERSION`-only action assembles nothing, so it needs none. One
wrinkle: each cut deletes the fragments it used, so the final stable section lists only what came
after the last alpha; it is merged by hand once.

**Trigger and control.** Anyone with write access to the repository can start the run, which here is
the owner. The push to `main` is the one act that needs more than the workflow token: section 6,
point 1, gives its cost.

**Artifacts (what Publish builds).** Publish runs on the tag (`push` of `v*`). Both images get the
tag, as `vX.Y.Z-alpha.N`, built by the existing build made callable with a version input, so there is
one image build with two callers. The per-commit date names and
`latest` stay as they are (`latest` keeps meaning the newest `main` image; operators who want a fixed
version pin the tag). The provisioner binary is built for `linux/amd64` and `linux/arm64` with plain
`go build`, `CGO_ENABLED=0` (`Dockerfile.provisioner:19`), packed with `tar`, with one `SHA256SUMS`.
The GitHub release is a pre-release, never Latest, with the notes of that changelog section.

**Gates.** Each one beyond a tag, images, binaries and the changelog section names the failure it
prevents:

| Gate | Failure it prevents |
|---|---|
| For an action that tags: `VERSION` parses, the tag and the next `VERSION` do not exist yet, and at least one fragment is left. A `VERSION`-only action checks that the `VERSION` it moves to is not an existing tag, and needs no fragment | a second use of a tag, which is permanent, or a release with empty notes; and a recovery action (`skip-prerelease-number`, after a tag was used) refused for lack of fragments that were already consumed |
| `HEAD` is the tip of `main` on the remote, and the push is atomic | releasing a stale checkout, or a `main` and a tag that disagree: a pull request merged during the wait makes the push non-fast-forward, and nothing is published |
| For an action that tags, a `Test` run passed on the dispatched commit (the whole workflow, database provider job included), as kure's CI wait does. The push trigger skips a commit that touches no listed path (`test.yml:10-22`), so where no run exists the workflow starts one with the dispatch trigger (`test.yml:26`) and waits | shipping a commit nobody tested |
| Dry run by default | making a permanent tag by a mistaken click |
| `SHA256SUMS` | a truncated or corrupted download of the host binary |
| One release at a time | two runs racing for the same tag |

**The limit of that gate:** the release commit (the changelog section, the deleted fragments, `VERSION`)
is made after `Test` passed, so the tagged commit differs from the tested one by those files only.
kure's release has the same shape.

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

**Deploy files.** The release edits none of `deploy/` (the kustomize `newTag`, the chart `appVersion`,
the compose `TAG`): the files carry the owner's choices, and changing them is a second commit to
`main` that nobody asked for.

## 6. Decisions still the owner's

The owner decides these on ginsys/shelly-manager#846. The pull request that carries this note decides
none. His answer of 2026-11-22 settled the trigger (one dispatch), where the version lives (the
`VERSION` file and the actions), the gate and the deploy files; those are recommendations in section
5, not questions. What is left:

1. **The release bot, which only he can create.** The real run pushes a commit to `main`, and the
   ruleset `general` refuses it: it requires status checks on the commit, and its `bypass_actors`
   list is empty (rulesets API, read 2026-11-22). The same token must also make the tag, because a tag
   made with the workflow's own token starts no workflow, so Publish would not run. *Recommend:* a
   GitHub App owned by ginsys, installed on this repository only, added as a bypass actor of
   `general`, with its id and key as secrets of an environment `release` that only `main` may use.
   - **What it costs.** A bypass actor passes every rule of `general` for every push it makes, not
     only a release push: required status checks, linear history, no non-fast-forward, no deletion,
     the Copilot review rule, code scanning and code quality. The guard then is the App's private key
     plus the one workflow that holds it. Whoever can read the key, or get a change to that workflow
     merged to `main`, can push anything to `main` and make any tag. That is the owner, who has admin
     rights, and anyone else given write access whose workflow change is merged through the checks.
     The environment limit keeps a workflow on another branch from reading the key.
   - **A narrower route, not read:** split `general` in two and give the bypass only to the ruleset
     that holds `required_status_checks`, so the release push still meets the other rules; and a tag
     ruleset that lets only the App create `v*` tags. Whether the rules for code scanning and code
     quality, which expect a pull request, refuse a direct push is **unknown**, and so is whether a
     bypass actor can be an App on this plan. Slice 3 settles both with a trial.
   - **The fallback keeps every rule:** the workflow opens the release PR itself and starts, on its
     branch, every workflow the ruleset needs, because a pull request made with the workflow token
     starts no checks. That is `Test`, which holds the five required status checks (`Validate Go
     Version`, `Run Tests`, `Frontend Unit Tests`, `Security Tests`, `Build Binary`), and CodeQL for
     the code scanning rule (`codeql.yml:11` has `workflow_dispatch`; its push trigger covers `main`
     only). It waits for all of them, merges through the normal route, tags the merge commit and
     calls Publish in the same run, as bronzeward does (`release-create.yml:113`). It needs no App
     and no bypass. It costs two waits and more steps in the workflow. **Unknown:** whether the
     workflow token may merge under `general`, and whether a dispatched CodeQL run satisfies the code
     scanning rule for the pull request. Not recommended: it is the more complex of the two for a
     repository with one owner.
2. **Scheme and first tag.** Recommend SemVer with `-alpha.N` and `v0.6.0-alpha.1`, from a dry run.
   Reason: the changelog used `0.5.4`, the unreleased work contains breaking changes, and nothing is
   stable yet.
3. **The dry-run default.** Recommend on, where kure's is off. Reason: the tag is permanent, and the
   owner starts the run by hand, so ticking one box costs less than one wrong tag.
4. **Host binary and signing, one confirmation.** Recommend plain `go build` for linux amd64 and
   arm64, `tar` and `SHA256SUMS`, with no GoReleaser, and no signing, SBOM or attestations in the
   first build. Reason: two binaries and two platforms need a dozen lines, GoReleaser adds a config
   and a pinned tool, and the only consumer is the owner's own cluster; revisit when someone else
   pulls an artifact.

## 7. Build slices

Each slice is one pull request with its own tests; none is a release.

1. **A version variable and the `VERSION` file.** A package variable set by `-ldflags -X`, shown by
   both binaries and by `/api/v1/version`, and the hard-coded `1.0.0` and `0.5.0-alpha` replaced by it.
   A root `VERSION` file holding `v0.6.0-alpha.1`. Tests pin the output, with `dev` when unset, and
   that the file parses.
2. **Callable image build.** `docker-build.yml` gains a version input; both Dockerfiles declare
   `ARG VERSION` and pass it in `-ldflags`; the UI build receives it. Default behaviour is unchanged.
3. **The release script and workflow.** `scripts/release/release.sh` copied from go-kure/.github and
   trimmed as section 5 says, with tests against real git repositories, and `release.yml`: guard, the
   `Test` wait, dry run, the atomic push. It needs point 1 of section 6 first, and its first trial
   settles the two unknowns there.
4. **The Publish workflow.** On the tag: both images through slice 2, the provisioner archives and
   `SHA256SUMS`, a pre-release with the changelog section as its notes. Slice 3 and this one may merge
   if together they stay under about 300 lines.
5. **Documentation.** The release procedure and the operator's upgrade and rollback page; the release-PR
   paragraph of `changelog.d/README.md:17-25`; the stale values from section 1 (chart `appVersion`, the
   bug template placeholder, the OpenAPI `version`).
6. **Only if point 4 of section 6 changes:** signing, SBOM and attestations.

Recommended: the first tag is cut by the owner, from a dry run, and
ginsys/shelly-manager#846 stays open until slice 4 has run once. Both are the owner's to confirm.

## 8. What this note does not do

It builds nothing and measures nothing: whether an image carries `git_sha` needs a build. Whether a
bypass actor can be a GitHub App on this plan, and whether a narrower ruleset still lets the release
push through, need a trial at slice 3 (section 6, point 1). It does not audit private projects, and it
does not change the stale values it lists.
