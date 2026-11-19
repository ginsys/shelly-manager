// Shows a duration the server sent as nanoseconds (a Go time.Duration, which JSON carries as a whole
// number). Anything that is not a finite, non-negative number has no usable duration: a dash.
export function formatDuration(ns: unknown): string {
  if (typeof ns !== 'number' || !Number.isFinite(ns) || ns < 0) return '—'
  const ms = ns / 1e6
  if (ns === 0) return '0 ms'
  if (ms < 1) return '< 1 ms'
  if (ms < 1000) return `${Math.round(ms)} ms`
  const s = ms / 1000
  if (s < 60) return `${s.toFixed(1)} s`
  const whole = Math.round(s)
  return `${Math.floor(whole / 60)} min ${whole % 60} s`
}
