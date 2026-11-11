// Decide whether the "UI refresh recommended" banner shows.
//
// Both sides carry the git revision they were built from: the UI through the Vite build
// (`__UI_BUILD__.git_sha`) and the server through `GET /api/v1/version` (`git_sha`). The banner
// shows only when both revisions are real and differ. A side with no revision (empty, 'dev' or
// 'unknown') says nothing about a mismatch, so the banner stays hidden.

const UNKNOWN_REVISIONS = new Set(['', 'dev', 'unknown'])

function normalize(sha: unknown): string {
  return typeof sha === 'string' ? sha.trim().toLowerCase() : ''
}

export function hasRevision(sha: unknown): boolean {
  return !UNKNOWN_REVISIONS.has(normalize(sha))
}

// A short revision (the UI build uses `git rev-parse --short`) matches a full one when it is its
// prefix, so the comparison is by prefix.
export function sameRevision(a: string, b: string): boolean {
  const x = normalize(a)
  const y = normalize(b)
  return x.startsWith(y) || y.startsWith(x)
}

export function shouldShowVersionBanner(uiSha: unknown, apiSha: unknown): boolean {
  if (!hasRevision(uiSha) || !hasRevision(apiSha)) return false
  return !sameRevision(normalize(uiSha), normalize(apiSha))
}
