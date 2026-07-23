/**
 * Canonical list of metrics WebSocket message types.
 *
 * This is the frontend half of a cross-language contract. The Go backend owns
 * the source of truth in `internal/metrics/websocket.go` (`AllMessageTypes()`),
 * and `TestMessageTypeManifestParity` (internal/metrics/contract_test.go) fails
 * CI if this array diverges from it. A new backend message type therefore cannot
 * merge without being added here, and the exhaustive `switch` in the metrics
 * store (guarded by `assertNever`) then fails `vue-tsc` until it is handled.
 *
 * Keep this list in sync with the Go constants; do not hand-maintain a second
 * copy of these strings elsewhere.
 */
export const METRICS_WS_MESSAGE_TYPES = [
  'initial_metrics',
  'metrics_update',
  'alert',
  'device_status_change',
  'drift_detected',
] as const

/** Union of every metrics WebSocket message type the backend can emit. */
export type MetricsWsMessageType = (typeof METRICS_WS_MESSAGE_TYPES)[number]

/**
 * Compile-time exhaustiveness guard. Call in the `default` branch of a switch
 * over a discriminated union so that adding a new `MetricsWsMessageType` without
 * a corresponding handler becomes a `vue-tsc` error rather than a silent runtime
 * fall-through.
 */
export function assertNever(value: never): never {
  throw new Error(`Unhandled metrics WebSocket message type: ${JSON.stringify(value)}`)
}
