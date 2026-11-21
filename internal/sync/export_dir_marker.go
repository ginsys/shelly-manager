package sync

// ExportDirMarker is the file an export that writes a directory tree puts into that directory. A
// directory that holds it as a regular file was written by an export: the next export may clean it, and
// a download may zip it. A symlink in its place does not count.
const ExportDirMarker = ".shelly-manager-gitops"
