export function formatFileSize(bytes?: number): string {
  if (bytes == null || isNaN(bytes as any)) return '—'
  if (bytes < 1024) return `${bytes} B`
  const units = ['KB', 'MB', 'GB', 'TB']
  let i = -1
  do { bytes = (bytes || 0) / 1024; i++ } while ((bytes || 0) >= 1024 && i < units.length - 1)
  return `${bytes.toFixed(1)} ${units[i]}`
}

export function formatDate(iso?: string): string {
  if (!iso) return '—'
  try { return new Date(iso).toLocaleString() } catch { return iso }
}

export function toTitleCase(s: string): string {
  return (s || '').replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())
}

export function formatLabel(format: string): string {
  return (format || '').toUpperCase()
}

export function structureLabel(structure: string): string {
  return toTitleCase(structure || '')
}

export function getFileIcon(type: string): string {
  switch (type) {
    case 'dir': return '📁'
    case 'yaml':
    case 'json': return '🧾'
    case 'tf': return '📜'
    default: return '📄'
  }
}

