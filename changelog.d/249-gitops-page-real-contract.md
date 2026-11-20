Fixed
- The GitOps Export page now talks to routes the server has, and the export history shows what a GitOps
  record holds. The page lists the GitOps history, reads one record, previews through the generic
  preview route and creates with only what the server reads: a name, a grouping (location or device
  type), the common and template options and a device filter. The form options that no part of the
  server read (Terraform, Ansible, Kubernetes and Docker Compose formats, repository structure, git
  and webhook settings, secrets and README switches, fixed sample devices, the fixed integration
  panel, per-format statistics and the Download button) are gone; a GitOps export is a directory and
  the server does not zip it yet. New `DELETE /api/v1/export/gitops/{id}` deletes the history record
  of a GitOps export and never touches the exported files. The preview now follows the common and
  template options and lists the devices no group claims under `ungrouped/`, as the export writes
  them. The history size of a GitOps export is now the total size of the exported files, where it
  was the size of the directory entry. Fixes ginsys/shelly-manager#249.
