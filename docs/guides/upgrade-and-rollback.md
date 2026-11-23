# Upgrading and rolling back

How to move a running install to the next release, and back. It names no version: the releases are
listed on the project's GitHub releases page, and the version of what you run is shown as below.

## What a release gives you

| What | Where |
|---|---|
| A tag, `vX.Y.Z` or `vX.Y.Z-alpha.N`, `-beta.N`, `-rc.N` | the GitHub releases page; every release is marked pre-release for now |
| Two images with that tag | `ghcr.io/ginsys/shelly-manager:<tag>` and `ghcr.io/ginsys/shelly-provisioner:<tag>` |
| The provisioner as a program for a host | the release's `shelly-provisioner_<tag>_linux_amd64.tar.gz` and `..._linux_arm64.tar.gz`, with `SHA256SUMS` |
| The notes | the changelog section of that tag |

Each image also keeps a tag made of its build date and commit, and the tag `latest`. **`latest` is the
newest build of `main`, and a release never moves it.** Do not use it to upgrade to a release, and
never to roll back.

To see which version you run: `shelly-manager --version`, or `GET /api/v1/version` (see
[the README](../../README.md#version)). The `VERSION` file in the repository is the *next* release, not
the one you run.

## Before you upgrade

1. **Read the notes of the release.** A change that repairs or refuses an existing database is named in
   its `Changed` or `Fixed` entry.
2. **Look at the size of the step.** While the project is below 1.0, a step to a new minor version
   (the middle number of `vX.Y.Z` changes) is where a breaking change lands. That is a changed or removed `/api/v1` route,
   a database change that needs the start-up repair or refuses start-up, a new archive format
   version, or a removed or renamed configuration key. A step from one `alpha.N` to the next is not
   meant to break anything.
3. **Take a backup of the database.** Create it in the web UI (Export & Import, then Backup Management)
   or with the export API, as [the export and import guide](export-import-system.md) and
   [the UI guide](ui-guide.md) describe. Keep the backup's id: a rollback needs it. Restoring a backup
   works for SQLite only.

## Upgrade

Pin the new tag for **both** images, in the place your install keeps it.

| Install | Where the tag is set |
|---|---|
| Docker Compose (`deploy/docker-compose/`) | the `TAG` variable, read by `docker-compose.yml` and `docker-compose.prod.yml` for both images |
| Kustomize (`deploy/kubernetes/kustomize/`) | `newTag` under `images:` in the base and in each overlay, or `kustomize edit set image ghcr.io/ginsys/shelly-manager:<tag>` and the same for `ghcr.io/ginsys/shelly-provisioner` |
| Helm, Kurel | there is no chart or Kurel package yet: the two README files in `deploy/kubernetes/` describe plans |
| The provisioner on a host | download the archive of the tag and `SHA256SUMS`, run `sha256sum -c --ignore-missing SHA256SUMS`, unpack the archive and replace the `shelly-provisioner` binary |

Then recreate the manager (on a rolling update, scale it to 0 first: the new pod would start
while the old pod still has the SQLite file open). At start it repairs and migrates the database, and **a migration cannot be
undone**: this is why the backup comes first. Read [the database upgrade guide](database-upgrade.md)
for what the repair does.

**If the manager refuses to start,** read its error and the table in
[When startup refuses](database-upgrade.md#when-startup-refuses). A refusal in the first check (the preflight),
and a failure in the data step on SQLite or PostgreSQL, leave the database as it was: fix the rows the error
names, or pin the previous tag against that database. A failure later in the repair leaves the database
partly repaired and the next start resumes it. This page gives no way back from that state.

## Roll back

A restore is staged on a **running** manager and applied at the **next start** of the manager, so
the order matters:

1. With the upgraded manager still running, stage the backup you took before the upgrade. In the web UI use
   **Stage restore**; through the API it is `POST /api/v1/import/restore` with the backup's id. The
   [export and import guide](export-import-system.md#restoring-a-backup-sqlite) has the checks and the
   space it needs.
2. Stop the manager. In Kubernetes the base manifest uses the `Recreate` strategy; for a rolling update,
   scale to 0 first.
3. Pin the previous tag for both images, as above.
4. Start the manager. The previous version applies the staged backup before it opens the database, then
   migrates it.

Two things to know:

- The previous release must be one whose manager applies a staged restore at start. Every release does.
- The upgraded database stays beside the database file as `<database file>.pre-restore` until the next
  restore. It is your way back to the upgraded state.

**Pinning the previous tag alone is not a rollback.** Nothing shows an older manager running on a
database a newer one has migrated, so restore the backup.
