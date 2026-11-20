Added
- `DELETE /api/v1/export/gitops/{id}` deletes the history record of a GitOps export and never touches the exported files (ginsys/shelly-manager#249).

Changed
- The GitOps Export page now talks to routes the server has. It lists the GitOps history (`GET /api/v1/export/history?plugin=gitops`), reads one record through `GET /api/v1/export/{id}`, previews through `POST /api/v1/export/preview`, and posts only what `POST /api/v1/export/gitops` reads: a name, a grouping, the common and template options and a device filter. The page shows what the record holds (name, directory, file count, size, status) and counts total, successful and failed exports.
- The GitOps export preview now follows the common-file and template options, as the export does: with them off, the file tree and the file count no longer list those files.
- The history size of a GitOps export is now the total size of the exported files; it was the size of the directory entry, which says nothing about its contents.

Removed
- The GitOps form options that no part of the server read or wrote: the Terraform, Ansible, Kubernetes and Docker Compose formats and their template options, the repository structure, git and webhook settings, secrets and README switches, the three fixed sample devices and the fixed integration-status panel, the per-format and per-structure statistics, and the Download button (a GitOps export is a directory, and the server does not zip it yet, ginsys/shelly-manager#749). The five unserved GitOps routes left the route-contract baseline, which is now empty.
